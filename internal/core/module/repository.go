package module

import "context"

// Repository defines persistence operations for module Records.
type Repository interface {
	FindByID(ctx context.Context, id string) (*Record, error)
	ListAll(ctx context.Context) ([]Record, error)
	Upsert(ctx context.Context, record *Record) error
	UpdateState(ctx context.Context, id string, state State) error
}

// JobRepository defines persistence operations for async Jobs.
type JobRepository interface {
	Create(ctx context.Context, job *Job) error
	FindByID(ctx context.Context, id string) (*Job, error)
	UpdateState(ctx context.Context, id string, state JobState, output, errMsg string) error
	AppendOutput(ctx context.Context, id, line string) error
	ListByState(ctx context.Context, state JobState) ([]Job, error)
	List(ctx context.Context, filter JobFilter) (*JobPage, error)
	Delete(ctx context.Context, id string) error
}
