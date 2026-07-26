// Package dashboard implements the dashboard domain for OpenDeploy Core.
//
// It aggregates data from multiple sources (Agent stats, module registry,
// audit log, active jobs) into a single API response used by the Vue dashboard.
// It also drives the WebSocket real-time push of metrics every 5 seconds.
package dashboard

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/anrted/opendeploy/internal/core/audit"
	"github.com/anrted/opendeploy/internal/core/module"
	"github.com/anrted/opendeploy/internal/platform/events"
	wsHub "github.com/anrted/opendeploy/internal/platform/websocket"
	"github.com/anrted/opendeploy/pkg/contract"
)

// Overview is the full dashboard snapshot returned by GET /api/v1/dashboard.
type Overview struct {
	ServerStats *contract.SystemStats `json:"server_stats"`
	Modules     ModuleSummary         `json:"modules"`
	RecentJobs  []JobSummary          `json:"recent_jobs"`
	RecentAudit []AuditSummary        `json:"recent_audit"`
	Snapshots   []StatSnapshot        `json:"snapshots"` // sparkline data (last 60 points)
}

// ModuleSummary counts modules by state.
type ModuleSummary struct {
	Total    int `json:"total"`
	Enabled  int `json:"enabled"`
	Disabled int `json:"disabled"`
	Error    int `json:"error"`
}

// JobSummary is a lightweight job view for the dashboard.
type JobSummary struct {
	ID         string     `json:"id"`
	Type       string     `json:"type"`
	State      string     `json:"state"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// AuditSummary is a lightweight audit entry for the dashboard feed.
type AuditSummary struct {
	Action    string    `json:"action"`
	Resource  string    `json:"resource,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// StatSnapshot is one data point from the stats_snapshots table.
type StatSnapshot struct {
	CPUPercent  float64   `json:"cpu"`
	MemPercent  float64   `json:"mem"`
	DiskPercent float64   `json:"disk"`
	Load1m      float64   `json:"load"`
	RecordedAt  time.Time `json:"recorded_at"`
}

// Service assembles the dashboard overview and manages the stats poller.
type Service struct {
	db         *sql.DB
	agent      contract.AgentClient
	moduleRepo module.Repository
	auditSvc   *audit.Service
	bus        *events.MemoryBus
	hub        *wsHub.Hub
	logger     *slog.Logger
	cachedStats atomic.Value
}

// NewService constructs a dashboard Service.
func NewService(
	db *sql.DB,
	agent contract.AgentClient,
	moduleRepo module.Repository,
	auditSvc *audit.Service,
	bus *events.MemoryBus,
	hub *wsHub.Hub,
	logger *slog.Logger,
) *Service {
	return &Service{
		db:         db,
		agent:      agent,
		moduleRepo: moduleRepo,
		auditSvc:   auditSvc,
		bus:        bus,
		hub:        hub,
		logger:     logger,
	}
}

// Overview returns the full dashboard snapshot.
func (s *Service) Overview(ctx context.Context) (*Overview, error) {
	overview := &Overview{}

	// Live system stats from Cache.
	stats := s.cachedStats.Load()
	if stats != nil {
		overview.ServerStats = stats.(*contract.SystemStats)
	}

	// Module summary.
	records, err := s.moduleRepo.ListAll(ctx)
	if err == nil {
		summary := ModuleSummary{Total: len(records)}
		for _, r := range records {
			switch r.State {
			case module.StateEnabled:
				summary.Enabled++
			case module.StateDisabled:
				summary.Disabled++
			case module.StateError:
				summary.Error++
			}
		}
		overview.Modules = summary
	}

	// Recent jobs (last 10).
	overview.RecentJobs = s.recentJobs(ctx, 10)

	// Recent audit entries (last 20).
	entries, err := s.auditSvc.List(ctx, 20, 0)
	if err == nil {
		for _, e := range entries {
			as := AuditSummary{
				Action:    e.Action,
				Status:    string(e.Status),
				CreatedAt: e.CreatedAt,
			}
			if e.Resource != nil {
				as.Resource = *e.Resource
			}
			overview.RecentAudit = append(overview.RecentAudit, as)
		}
	}

	// Sparkline snapshots (last 60 data points — 5 minutes at 5s interval).
	overview.Snapshots, _ = s.listSnapshots(ctx, 60)

	return overview, nil
}

// StartPoller begins the background goroutine that collects stats every
// interval and pushes them to WebSocket clients.
// Call this from the app bootstrapper. The goroutine stops when ctx is cancelled.
func (s *Service) StartPoller(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.poll(context.Background())
			}
		}
	}()
	s.logger.Info("dashboard: stats poller started", "interval", interval)
}

// poll collects one stats snapshot, stores it, and pushes to WebSocket clients.
func (s *Service) poll(ctx context.Context) {
	if s.agent == nil {
		return
	}
	stats, err := s.agent.SystemStats(ctx)
	if err != nil {
		s.logger.WarnContext(ctx, "dashboard: poll stats failed", "error", err)
		return
	}
	s.cachedStats.Store(stats)

	// Persist snapshot.
	diskPct := 0.0
	if len(stats.Disk) > 0 {
		diskPct = stats.Disk[0].UsedPercent
	}
	if err := s.saveSnapshot(ctx, stats.CPU.UsagePercent, stats.Memory.UsedPercent,
		diskPct, stats.LoadAverage[0]); err != nil {
		s.logger.WarnContext(ctx, "dashboard: save snapshot failed", "error", err)
	}

	// Broadcast to WebSocket room "dashboard".
	s.hub.BroadcastToRoom("dashboard", wsHub.Message{
		Type:    "stats_update",
		Payload: stats,
	})

	// Purge snapshots older than 24 hours.
	s.pruneSnapshots(ctx, 24*time.Hour)
}

// saveSnapshot inserts one stats_snapshots row.
func (s *Service) saveSnapshot(ctx context.Context, cpu, mem, disk, load float64) error {
	const q = `INSERT INTO stats_snapshots
	           (id, cpu_percent, mem_percent, disk_percent, load_1m, load_5m, load_15m,
	            net_rx_bytes, net_tx_bytes, recorded_at)
	           VALUES (?, ?, ?, ?, ?, 0, 0, 0, 0, ?)`
	_, err := s.db.ExecContext(ctx, q,
		uuid.New().String(),
		cpu, mem, disk, load,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// listSnapshots returns the most recent n snapshots ordered ascending (for charts).
func (s *Service) listSnapshots(ctx context.Context, n int) ([]StatSnapshot, error) {
	const q = `SELECT cpu_percent, mem_percent, disk_percent, load_1m, recorded_at
	           FROM stats_snapshots
	           ORDER BY recorded_at DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, n)
	if err != nil {
		return nil, fmt.Errorf("dashboard: list snapshots: %w", err)
	}
	defer rows.Close()

	var snaps []StatSnapshot
	for rows.Next() {
		var snap StatSnapshot
		var recordedAt string
		if err := rows.Scan(&snap.CPUPercent, &snap.MemPercent, &snap.DiskPercent, &snap.Load1m, &recordedAt); err != nil {
			return nil, err
		}
		snap.RecordedAt, _ = time.Parse(time.RFC3339, recordedAt)
		snaps = append(snaps, snap)
	}
	// Reverse to get ascending order for charts.
	for i, j := 0, len(snaps)-1; i < j; i, j = i+1, j-1 {
		snaps[i], snaps[j] = snaps[j], snaps[i]
	}
	return snaps, rows.Err()
}

// pruneSnapshots deletes snapshots older than the given duration.
func (s *Service) pruneSnapshots(ctx context.Context, maxAge time.Duration) {
	cutoff := time.Now().UTC().Add(-maxAge).Format(time.RFC3339)
	_, _ = s.db.ExecContext(ctx, `DELETE FROM stats_snapshots WHERE recorded_at < ?`, cutoff)
}

// recentJobs returns the n most recent jobs from the DB.
func (s *Service) recentJobs(ctx context.Context, n int) []JobSummary {
	const q = `SELECT id, type, state, created_at, finished_at
	           FROM jobs ORDER BY created_at DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, n)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var jobs []JobSummary
	for rows.Next() {
		var j JobSummary
		var createdAt string
		var finishedAt *string
		if err := rows.Scan(&j.ID, &j.Type, &j.State, &createdAt, &finishedAt); err != nil {
			continue
		}
		j.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if finishedAt != nil {
			t, _ := time.Parse(time.RFC3339, *finishedAt)
			j.FinishedAt = &t
		}
		jobs = append(jobs, j)
	}
	return jobs
}
