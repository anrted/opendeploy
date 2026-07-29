package module

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/anrted/opendeploy/pkg/contract"
)

type loaderTestModule struct {
	contract.Module
	id       string
	statusFn func(context.Context) (*contract.RuntimeStatus, error)
}

func (m *loaderTestModule) ID() string   { return m.id }
func (m *loaderTestModule) Name() string { return m.id }
func (m *loaderTestModule) Bootstrap(contract.ModuleDeps) error {
	return nil
}
func (m *loaderTestModule) Status(ctx context.Context) (*contract.RuntimeStatus, error) {
	return m.statusFn(ctx)
}
func (m *loaderTestModule) Capabilities() contract.ModuleCapabilities {
	return contract.ModuleCapabilities{}
}

type loaderTestRepository struct {
	mu       sync.Mutex
	records  map[string]*Record
	findErr  error
	upserted int
}

func (r *loaderTestRepository) FindByID(_ context.Context, id string) (*Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.findErr != nil {
		return nil, r.findErr
	}
	rec := *r.records[id]
	return &rec, nil
}
func (r *loaderTestRepository) ListAll(context.Context) ([]Record, error) { return nil, nil }
func (r *loaderTestRepository) Upsert(_ context.Context, rec *Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.upserted++
	copy := *rec
	r.records[rec.ID] = &copy
	return nil
}
func (r *loaderTestRepository) UpdateState(context.Context, string, State) error { return nil }

func newLoaderTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestLoaderBootstrapDoesNotDetectRuntimeState(t *testing.T) {
	statusCalled := false
	mod := &loaderTestModule{id: "slow", statusFn: func(context.Context) (*contract.RuntimeStatus, error) {
		statusCalled = true
		return nil, nil
	}}
	registry := NewRegistry()
	registry.Register(mod)
	repo := &loaderTestRepository{records: map[string]*Record{
		"slow": {ID: "slow", Name: "slow", State: StateAvailable},
	}}

	if err := NewLoader(registry, repo, newLoaderTestLogger()).Bootstrap(context.Background(), contract.ModuleDeps{}); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if statusCalled {
		t.Fatal("Bootstrap() called Status() on the startup path")
	}
}

func TestLoaderBootstrapDoesNotTreatReadFailureAsMissingRecord(t *testing.T) {
	readErr := errors.New("database unavailable")
	mod := &loaderTestModule{id: "test", statusFn: func(context.Context) (*contract.RuntimeStatus, error) {
		return nil, nil
	}}
	registry := NewRegistry()
	registry.Register(mod)
	repo := &loaderTestRepository{records: map[string]*Record{}, findErr: readErr}

	err := NewLoader(registry, repo, newLoaderTestLogger()).Bootstrap(context.Background(), contract.ModuleDeps{})
	if !errors.Is(err, readErr) {
		t.Fatalf("Bootstrap() error = %v, want wrapped read error", err)
	}
	if repo.upserted != 0 {
		t.Fatalf("Bootstrap() performed %d upserts after a read failure", repo.upserted)
	}
}

func TestLoaderRuntimeSyncContinuesAfterModuleTimeout(t *testing.T) {
	var secondCalled bool
	first := &loaderTestModule{id: "a-slow", statusFn: func(ctx context.Context) (*contract.RuntimeStatus, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	second := &loaderTestModule{id: "b-fast", statusFn: func(context.Context) (*contract.RuntimeStatus, error) {
		secondCalled = true
		return &contract.RuntimeStatus{}, nil
	}}
	registry := NewRegistry()
	registry.Register(second)
	registry.Register(first)
	repo := &loaderTestRepository{records: map[string]*Record{
		"a-slow": {ID: "a-slow", Name: "a-slow", State: StateAvailable},
		"b-fast": {ID: "b-fast", Name: "b-fast", State: StateAvailable},
	}}

	NewLoader(registry, repo, newLoaderTestLogger()).SyncRuntimeStates(context.Background(), 10*time.Millisecond)
	if !secondCalled {
		t.Fatal("runtime sync did not continue after the preceding module timed out")
	}
}
