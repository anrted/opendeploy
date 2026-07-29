package module

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/pkg/contract"
)

// Registry is the central store of all registered modules.
// Modules are registered at application startup (compile-time) via Register.
// The registry is read-only after the application starts.
type Registry struct {
	mu      sync.RWMutex
	modules map[string]contract.Module
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{modules: make(map[string]contract.Module)}
}

// Register adds a Module to the registry. Panics if a module with the same ID
// is registered twice (programming error, caught at startup).
func (reg *Registry) Register(m contract.Module) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if _, exists := reg.modules[m.ID()]; exists {
		panic("module registry: duplicate module ID: " + m.ID())
	}
	reg.modules[m.ID()] = m
}

// Find returns a Module by its ID, or nil if not found.
func (reg *Registry) Find(id string) contract.Module {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return reg.modules[id]
}

// All returns all registered modules as a slice (order undefined).
func (reg *Registry) All() []contract.Module {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	result := make([]contract.Module, 0, len(reg.modules))
	for _, m := range reg.modules {
		result = append(result, m)
	}
	return result
}

// IDs returns the IDs of all registered modules.
func (reg *Registry) IDs() []string {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	ids := make([]string, 0, len(reg.modules))
	for id := range reg.modules {
		ids = append(ids, id)
	}
	return ids
}

// ─── Loader ────────────────────────────────────────────────────────────────

// Loader initializes registered modules and their persisted metadata.
type Loader struct {
	registry *Registry
	repo     Repository
	logger   *slog.Logger
}

// NewLoader creates a Loader.
func NewLoader(registry *Registry, repo Repository, logger *slog.Logger) *Loader {
	return &Loader{registry: registry, repo: repo, logger: logger}
}

// Bootstrap seeds the database and injects dependencies into every module.
// Runtime detection is kept out of the startup path because Agent calls may be
// slow and Fx applies one shared deadline to all startup hooks.
func (l *Loader) Bootstrap(ctx context.Context, deps contract.ModuleDeps) error {
	for _, m := range l.sortedModules() {
		// Ensure a database record exists for every registered module.
		existing, err := l.repo.FindByID(ctx, m.ID())
		if err != nil {
			if !apperrors.IsAppError(err, apperrors.CodeNotFound) {
				return fmt.Errorf("module loader: find record %q: %w", m.ID(), err)
			}
			// First time seeing this module — create its record.
			now := nowUTC()
			rec := &Record{
				ID:        m.ID(),
				Name:      m.Name(),
				State:     StateAvailable,
				Config:    "{}",
				UpdatedAt: now,
			}
			if err := l.repo.Upsert(ctx, rec); err != nil {
				return fmt.Errorf("module loader: upsert record %q: %w", m.ID(), err)
			}
			existing = rec
		}

		l.logger.Info("module loader: bootstrapping module", "module", m.ID(), "state", existing.State)
		if err := m.Bootstrap(deps); err != nil {
			l.logger.Error("module loader: bootstrap failed", "module", m.ID(), "error", err)
			_ = l.repo.UpdateState(ctx, m.ID(), StateError)
			continue
		}
	}
	return nil
}

// SyncRuntimeStates detects software installed outside OpenDeploy. It is safe
// to run after the server starts. Every module receives a fresh timeout so one
// slow Agent call cannot starve the modules that follow it.
func (l *Loader) SyncRuntimeStates(ctx context.Context, perModuleTimeout time.Duration) {
	for _, m := range l.sortedModules() {
		if ctx.Err() != nil {
			return
		}
		moduleCtx, cancel := context.WithTimeout(ctx, perModuleTimeout)
		l.syncRuntimeState(moduleCtx, m)
		cancel()
	}
}

func (l *Loader) syncRuntimeState(ctx context.Context, m contract.Module) {
	existing, err := l.repo.FindByID(ctx, m.ID())
	if err != nil {
		l.logger.Warn("module loader: load runtime state", "module", m.ID(), "error", err)
		return
	}
	needsSync := existing.State == StateAvailable || existing.Version == nil || *existing.Version == ""
	if needsSync {
		status, err := m.Status(ctx)
		if err != nil {
			l.logger.Warn("module loader: detect runtime state", "module", m.ID(), "error", err)
			return
		}
		if status != nil {
			detected := StateAvailable
			if status.PackageStatus == contract.PackageInstalled {
				detected = StateInstalled
				if m.Capabilities().SupportsService {
					if status.ServiceStatus == contract.ServiceRunning {
						detected = StateEnabled
					} else {
						detected = StateDisabled
					}
				}
			}

			switch detected {
			case StateInstalled, StateEnabled, StateDisabled:
				rec := *existing
				rec.State = detected
				if status.SoftwareVersion != "" {
					v := status.SoftwareVersion
					rec.Version = &v
				}
				if err := l.repo.Upsert(ctx, &rec); err != nil {
					l.logger.Warn("module loader: persist detected state", "module", m.ID(), "error", err)
				}
			}
		}
	}
}

func (l *Loader) sortedModules() []contract.Module {
	modules := l.registry.All()
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].ID() < modules[j].ID()
	})
	return modules
}
