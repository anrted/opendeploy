package module
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/anrted/opendeploy/internal/core/audit"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/internal/platform/events"
	"github.com/anrted/opendeploy/pkg/contract"
	"github.com/google/uuid"
)

const moduleJobTimeout = 30 * time.Minute

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
	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc
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
		cancels:  make(map[string]context.CancelFunc),
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

		var installedVersion string
		if rec.Version != nil {
			installedVersion = *rec.Version
		}

		views = append(views, ModuleView{
			ID:              m.ID(),
			Name:            m.Name(),
			ModuleVersion:   m.Version(),
			SoftwareVersion: installedVersion,
			Description:     m.Description(),
			Category:        m.Category(),
			Icon:            m.Icon(),
			Dependencies:    m.Dependencies(),
			Capabilities:    m.Capabilities(),
			State:           rec.State,
			InstalledAt:     rec.InstalledAt,
			Pages:           m.Pages(),
			Actions:         m.Actions(),
			Logs:            m.Logs(),
			SettingsSchema:  m.SettingsSchema(),
		})
	}
	return views, nil
}

// Get returns details for a single module (without running full dynamic status).
func (s *Service) Get(ctx context.Context, id string) (*ModuleView, error) {
	m := s.registry.Find(id)
	if m == nil {
		return nil, apperrors.New(404, apperrors.CodeModuleNotFound, "module not found: "+id)
	}
	rec, err := s.repo.FindByID(ctx, id)
	if err != nil {
		rec = &Record{ID: id, Name: m.Name(), State: StateAvailable}
	}

	var installedVersion string
	if rec.Version != nil {
		installedVersion = *rec.Version
	}

	return &ModuleView{
		ID:              m.ID(),
		Name:            m.Name(),
		ModuleVersion:   m.Version(),
		SoftwareVersion: installedVersion,
		Description:     m.Description(),
		Category:        m.Category(),
		Icon:            m.Icon(),
		Dependencies:    m.Dependencies(),
		Capabilities:    m.Capabilities(),
		State:           rec.State,
		InstalledAt:     rec.InstalledAt,
		Pages:           m.Pages(),
		Actions:         m.Actions(),
		Logs:            m.Logs(),
		SettingsSchema:  m.SettingsSchema(),
	}, nil
}

// Status explicitly fetches the runtime status of a module via the Agent.
func (s *Service) Status(ctx context.Context, id string) (*contract.RuntimeStatus, error) {
	m := s.registry.Find(id)
	if m == nil {
		return nil, apperrors.New(404, apperrors.CodeModuleNotFound, "module not found: "+id)
	}

	status, err := m.Status(ctx)
	if err != nil {
		s.logger.Warn("module service: runtime status failed", "module", m.ID(), "error", err)
		return nil, apperrors.Internal("failed to fetch status", err)
	}

	// Optionally update the DB with the actual status here,
	// e.g. updating rec.State or rec.Version if they have drifted.

	return status, nil
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
		var version *string
		if status, err := m.Status(ctx); err == nil && status != nil && status.SoftwareVersion != "" {
			v := status.SoftwareVersion
			version = &v
		}
		_ = s.repo.Upsert(ctx, &Record{
			ID: id, Name: m.Name(), State: StateInstalled,
			Version:     version,
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

// ExecuteAction executes a dynamic module action.
func (s *Service) ExecuteAction(ctx context.Context, id, actionID, userID, ip string) error {
	m := s.registry.Find(id)
	if m == nil {
		return apperrors.New(404, apperrors.CodeModuleNotFound, "module not found: "+id)
	}
	if err := m.ExecuteAction(ctx, actionID); err != nil {
		return apperrors.Internal(fmt.Sprintf("execute action %s on module %s", actionID, id), err)
	}
	s.recordAudit(ctx, userID, "module.action."+actionID, id, ip, audit.StatusSuccess)
	return nil
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
		Name:      fmt.Sprintf("%s %s", jobType, moduleID),
		Type:      jobType,
		Payload:   string(payload),
		State:     JobPending,
		CreatedAt: nowUTC(),
		Progress:  0,
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		return "", apperrors.Internal("create job", err)
	}

	// Detached goroutine — use background context so it outlives the request.
	bgCtx, cancel := context.WithTimeout(context.Background(), moduleJobTimeout)
	s.cancelMu.Lock()
	s.cancels[job.ID] = cancel
	s.cancelMu.Unlock()
	go func() {
		defer func() {
			cancel()
			s.cancelMu.Lock()
			delete(s.cancels, job.ID)
			s.cancelMu.Unlock()
		}()
		_ = s.jobs.UpdateState(bgCtx, job.ID, JobRunning, "", "")

		if err := work(bgCtx); err != nil {
			s.logger.Error("module job failed", "job_id", job.ID, "type", jobType, "error", err)
			persistCtx, persistCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer persistCancel()
			state := JobError
			if errors.Is(err, context.Canceled) {
				state = JobCanceled
			}
			_ = s.jobs.UpdateState(persistCtx, job.ID, state, "", err.Error())
			s.publishEvent(persistCtx, "job.error", map[string]string{"job_id": job.ID})
			return
		}
		if bgCtx.Err() != nil {
			persistCtx, persistCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer persistCancel()
			_ = s.jobs.UpdateState(persistCtx, job.ID, JobCanceled, "", bgCtx.Err().Error())
			s.publishEvent(persistCtx, "job.canceled", map[string]string{"job_id": job.ID})
			return
		}

		persistCtx, persistCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer persistCancel()
		onSuccess(persistCtx)
		_ = s.jobs.UpdateState(persistCtx, job.ID, JobSuccess, "", "")
		s.publishEvent(persistCtx, "job.done", map[string]string{"job_id": job.ID})
	}()

	return job.ID, nil
}
