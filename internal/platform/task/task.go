package task

import (
	"context"
	"sync"
	"time"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
	StatusPaused  Status = "paused"
)

// Task represents an asynchronous background job.
type Task struct {
	ID          string
	Type        string
	Status      Status
	Progress    int
	CurrentStep string
	Message     string
	StartedAt   *time.Time
	FinishedAt  *time.Time
	Error       string
	Result      any
	Logs        []string
	mu          sync.Mutex
}

// Store defines how tasks are persisted.
type Store interface {
	Save(t *Task) error
	Get(id string) (*Task, error)
	List() ([]*Task, error)
}

// Manager handles queuing and execution of tasks.
type Manager struct {
	store Store
}

func NewManager(s Store) *Manager {
	return &Manager{store: s}
}

// CreateTask registers a new task.
func (m *Manager) CreateTask(taskType string) *Task {
	t := &Task{
		ID:     "task-" + time.Now().Format("20060102150405"),
		Type:   taskType,
		Status: StatusPending,
	}
	_ = m.store.Save(t)
	return t
}

// Execute runs the function in the background, updating the task status.
func (m *Manager) Execute(t *Task, fn func(ctx context.Context, t *Task) error) {
	go func() {
		ctx := context.Background()
		t.mu.Lock()
		t.Status = StatusRunning
		now := time.Now()
		t.StartedAt = &now
		t.mu.Unlock()
		_ = m.store.Save(t)

		err := fn(ctx, t)

		t.mu.Lock()
		nowEnd := time.Now()
		t.FinishedAt = &nowEnd
		if err != nil {
			t.Status = StatusFailed
			t.Error = err.Error()
		} else {
			t.Status = StatusSuccess
		}
		t.mu.Unlock()
		_ = m.store.Save(t)
	}()
}

// Log adds a log message to the task.
func (t *Task) Log(msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Logs = append(t.Logs, msg)
}
