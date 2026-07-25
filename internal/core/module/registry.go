package module

import (
	"context"
	"log/slog"
	"sync"

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
// Available modules also need Agent access to detect externally installed
// software.
func (l *Loader) Bootstrap(ctx context.Context, deps contract.ModuleDeps) error {
	for _, m := range l.registry.All() {
		// Ensure a database record exists for every registered module.
		existing, err := l.repo.FindByID(ctx, m.ID())
		if err != nil {
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
				l.logger.Error("module loader: upsert record", "module", m.ID(), "error", err)
				continue
			}
			existing = rec
		}

		l.logger.Info("module loader: bootstrapping module", "module", m.ID(), "state", existing.State)
		if err := m.Bootstrap(deps); err != nil {
			l.logger.Error("module loader: bootstrap failed", "module", m.ID(), "error", err)
			_ = l.repo.UpdateState(ctx, m.ID(), StateError)
			continue
		}
		if existing.State == StateAvailable {
			status, err := m.Status(ctx)
			if err != nil {
				l.logger.Warn("module loader: detect runtime state", "module", m.ID(), "error", err)
				continue
			}
			if status != nil {
				detected := State(status.State)
				switch detected {
				case StateInstalled, StateEnabled, StateDisabled:
					if err := l.repo.UpdateState(ctx, m.ID(), detected); err != nil {
						l.logger.Warn("module loader: persist detected state", "module", m.ID(), "error", err)
					}
				}
			}
		}
	}
	return nil
}
