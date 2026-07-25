package module

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/anrted/opendeploy/internal/core/audit"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/internal/platform/events"
	"github.com/anrted/opendeploy/pkg/contract"
	"github.com/google/uuid"
)

// Service orchestrates module lifecycle operations.
// Long-running operations (Install, Uninstall) run asynchronously and return
// a Job ID that the client polls or receives via WebSocket.
type Service struct {
	registry *Registry
	repo     Repository
	jobs     JobRepository
	bus      *events.MemoryBus
	audit    *audit.Service
	logger   *slog.Logger
}

// NewService constructs a module Service.
func NewService(
	registry *Registry,
	repo Repository,
	jobs JobRepository,
	bus *events.MemoryBus,
	audit *audit.Service,
	logger *slog.Logger,
) *Service {
	return &Service{
		registry: registry,
		repo:     repo,
		jobs:     jobs,
		bus:      bus,
		audit:    audit,
		logger:   logger,
	}
}

// List returns metadata for all registered modules merged with their DB state.
func (s *Service) List(ctx context.Context) ([]ModuleView, error) {
	records, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("module service: list: %w", err)
	}

	// Index by ID for fast lookup.
	byID := make(map[string]Record, len(records))
	for _, r := range records {
		byID[r.ID] = r
	}

	var views []ModuleView
	for _, m := range s.registry.All() {
		rec, ok := byID[m.ID()]
		if !ok {
			rec = Record{ID: m.ID(), Name: m.Name(), State: StateAvailable}
		}
		views = append(views, ModuleView{
			ID:          m.ID(),
			Name:        m.Name(),
			Version:     m.Version(),
			Description: m.Description(),
			State:       rec.State,
			InstalledAt: rec.InstalledAt,
		})
	}
	return views, nil
}

// Get returns details for a single module.
func (s *Service) Get(ctx context.Context, id string) (*ModuleView, error) {
	m := s.registry.Find(id)
	if m == nil {
		return nil, apperrors.New(404, apperrors.CodeModuleNotFound, "module not found: "+id)
	}
	rec, err := s.repo.FindByID(ctx, id)
	if err != nil {
		rec = &Record{ID: id, Name: m.Name(), State: StateAvailable}
	}

	var status *contract.ModuleStatus
	if rec.State == StateEnabled {
		status, _ = m.Status(ctx)
	}

	return &ModuleView{
		ID:          m.ID(),
		Name:        m.Name(),
		Version:     m.Version(),
		Description: m.Description(),
		State:       rec.State,
		InstalledAt: rec.InstalledAt,
		Status:      status,
	}, nil
}

// Install starts the async installation of a module. Returns the Job ID.
func (s *Service) Install(ctx context.Context, id, userID, ip string) (string, error) {
	m := s.registry.Find(id)
	if m == nil {
		return "", apperrors.New(404, apperrors.CodeModuleNotFound, "module not found: "+id)
	}
	rec, _ := s.repo.FindByID(ctx, id)
	if rec != nil && (rec.State == StateInstalled || rec.State == StateEnabled) {
		return "", apperrors.New(409, apperrors.CodeModuleInstalled, "module already installed")
	}
	if rec != nil && (rec.State == StateInstalling || rec.State == StateRemoving) {
		return "", apperrors.New(409, apperrors.CodeModuleBusy, "module operation in progress")
	}

	jobID, err := s.startJob(ctx, JobInstall, id, func(jobCtx context.Context) error {
		return m.Install(jobCtx)
	}, func(ctx context.Context) {
		now := nowUTC()
		_ = s.repo.Upsert(ctx, &Record{
			ID: id, Name: m.Name(), State: StateInstalled,
			InstalledAt: &now, UpdatedAt: now,
		})
		s.publishEvent(ctx, "module.installed", map[string]string{"module_id": id})
	})
	if err != nil {
		return "", err
	}

	_ = s.repo.UpdateState(ctx, id, StateInstalling)
	s.recordAudit(ctx, userID, "module.install", id, ip, audit.StatusSuccess)
	return jobID, nil
}

// Uninstall starts the async removal of a module. Returns the Job ID.
func (s *Service) Uninstall(ctx context.Context, id, userID, ip string) (string, error) {
	m := s.registry.Find(id)
	if m == nil {
		return "", apperrors.New(404, apperrors.CodeModuleNotFound, "module not found: "+id)
	}
	rec, err := s.repo.FindByID(ctx, id)
	if err != nil || rec.State == StateAvailable {
		return "", apperrors.New(409, apperrors.CodeModuleNotInstalled, "module is not installed")
	}

	jobID, err := s.startJob(ctx, JobUninstall, id, func(jobCtx context.Context) error {
		return m.Uninstall(jobCtx)
	}, func(ctx context.Context) {
		_ = s.repo.UpdateState(ctx, id, StateAvailable)
		s.publishEvent(ctx, "module.uninstalled", map[string]string{"module_id": id})
	})
	if err != nil {
		return "", err
	}

	_ = s.repo.UpdateState(ctx, id, StateRemoving)
	s.recordAudit(ctx, userID, "module.uninstall", id, ip, audit.StatusSuccess)
	return jobID, nil
}

// Enable activates an installed module (starts its service etc.)
func (s *Service) Enable(ctx context.Context, id, userID, ip string) error {
	m := s.registry.Find(id)
	if m == nil {
		return apperrors.New(404, apperrors.CodeModuleNotFound, "module not found: "+id)
	}
	rec, err := s.repo.FindByID(ctx, id)
	if err != nil || (rec.State != StateInstalled && rec.State != StateDisabled) {
		return apperrors.New(409, apperrors.CodeModuleNotInstalled, "module must be installed before enabling")
	}

	if err := m.Enable(ctx); err != nil {
		return apperrors.Internal("enable module", err)
	}
	if err := s.repo.UpdateState(ctx, id, StateEnabled); err != nil {
		return apperrors.Internal("update module state", err)
	}
	s.publishEvent(ctx, "module.enabled", map[string]string{"module_id": id})
	s.recordAudit(ctx, userID, "module.enable", id, ip, audit.StatusSuccess)
	return nil
}

// Disable deactivates an enabled module.
func (s *Service) Disable(ctx context.Context, id, userID, ip string) error {
	m := s.registry.Find(id)
	if m == nil {
		return apperrors.New(404, apperrors.CodeModuleNotFound, "module not found: "+id)
	}
	if err := m.Disable(ctx); err != nil {
		return apperrors.Internal("disable module", err)
	}
	if err := s.repo.UpdateState(ctx, id, StateDisabled); err != nil {
		return apperrors.Internal("update module state", err)
	}
	s.publishEvent(ctx, "module.disabled", map[string]string{"module_id": id})
	s.recordAudit(ctx, userID, "module.disable", id, ip, audit.StatusSuccess)
	return nil
}

// Restart restarts a running module's service.
func (s *Service) Restart(ctx context.Context, id, userID, ip string) error {
	m := s.registry.Find(id)
	if m == nil {
		return apperrors.New(404, apperrors.CodeModuleNotFound, "module not found: "+id)
	}
	if err := m.Restart(ctx); err != nil {
		return apperrors.Internal("restart module", err)
	}
	s.recordAudit(ctx, userID, "module.restart", id, ip, audit.StatusSuccess)
	return nil
}

// GetJob returns the current state of a background job.
func (s *Service) GetJob(ctx context.Context, jobID string) (*Job, error) {
	return s.jobs.FindByID(ctx, jobID)
}

// ─── ModuleView ────────────────────────────────────────────────────────────

// ModuleView is the API response DTO for a module.
type ModuleView struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	State       State                  `json:"state"`
	InstalledAt *time.Time             `json:"installed_at,omitempty"`
	Status      *contract.ModuleStatus `json:"status,omitempty"`
}

// ─── internal helpers ──────────────────────────────────────────────────────

// startJob creates a Job record and launches an async goroutine.
func (s *Service) startJob(
	ctx context.Context,
	jobType JobType,
	moduleID string,
	work func(ctx context.Context) error,
	onSuccess func(ctx context.Context),
) (string, error) {
	payload, _ := json.Marshal(map[string]string{"module_id": moduleID})
	job := &Job{
		ID:        uuid.New().String(),
		Type:      jobType,
		Payload:   string(payload),
		State:     JobPending,
		CreatedAt: nowUTC(),
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		return "", apperrors.Internal("create job", err)
	}

	// Detached goroutine — use background context so it outlives the request.
	go func() {
		bgCtx := context.Background()
		_ = s.jobs.UpdateState(bgCtx, job.ID, JobRunning, "", "")

		if err := work(bgCtx); err != nil {
			s.logger.Error("module job failed", "job_id", job.ID, "type", jobType, "error", err)
			_ = s.jobs.UpdateState(bgCtx, job.ID, JobError, "", err.Error())
			s.publishEvent(bgCtx, "job.error", map[string]string{"job_id": job.ID})
			return
		}

		onSuccess(bgCtx)
		_ = s.jobs.UpdateState(bgCtx, job.ID, JobSuccess, "", "")
		s.publishEvent(bgCtx, "job.done", map[string]string{"job_id": job.ID})
	}()

	return job.ID, nil
}

func (s *Service) publishEvent(ctx context.Context, eventType string, payload any) {
	ev := events.NewBaseEvent(eventType, payload)
	if err := s.bus.Publish(ctx, ev); err != nil {
		s.logger.Warn("module service: publish event failed", "type", eventType, "error", err)
	}
}

func (s *Service) recordAudit(ctx context.Context, userID, action, moduleID, ip string, status audit.Status) {
	uid := &userID
	resource := moduleID
	ipPtr := &ip
	_ = s.audit.Record(ctx, audit.Entry{
		UserID:    uid,
		Action:    action,
		Resource:  &resource,
		IPAddress: ipPtr,
		Status:    status,
	})
}

func nowUTC() time.Time { return time.Now().UTC() }
