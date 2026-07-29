package module

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anrted/opendeploy/internal/core/servercontext"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/google/uuid"
)

// ─── sqliteRepository ──────────────────────────────────────────────────────

type sqliteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a module Repository backed by SQLite.
func NewSQLiteRepository(db *sql.DB) Repository {
	return &sqliteRepository{db: db}
}

func (r *sqliteRepository) FindByID(ctx context.Context, id string) (*Record, error) {
	const q = `SELECT id, name, state, version, config, installed_at, updated_at
	           FROM modules WHERE id = ? AND server_id=?`
	row := r.db.QueryRowContext(ctx, q, id, servercontext.ID(ctx))
	rec, err := r.scanRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("module")
		}
		return nil, fmt.Errorf("module repo: find by id: %w", err)
	}
	return rec, nil
}

func (r *sqliteRepository) ListAll(ctx context.Context) ([]Record, error) {
	const q = `SELECT id, name, state, version, config, installed_at, updated_at
	           FROM modules WHERE server_id=? ORDER BY name`
	rows, err := r.db.QueryContext(ctx, q, servercontext.ID(ctx))
	if err != nil {
		return nil, fmt.Errorf("module repo: list: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var rec Record
		var version, installedAt *string
		var updatedAt string
		if err := rows.Scan(&rec.ID, &rec.Name, &rec.State, &version, &rec.Config, &installedAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("module repo: scan: %w", err)
		}
		rec.Version = version
		rec.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		if installedAt != nil {
			t, _ := time.Parse(time.RFC3339, *installedAt)
			rec.InstalledAt = &t
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (r *sqliteRepository) Upsert(ctx context.Context, rec *Record) error {
	const q = `INSERT INTO modules (id, server_id, name, state, version, config, installed_at, updated_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	           ON CONFLICT(id,server_id) DO UPDATE SET
	               name=excluded.name, state=excluded.state, version=excluded.version,
	               config=excluded.config, installed_at=excluded.installed_at,
	               updated_at=excluded.updated_at`
	var installedAt *string
	if rec.InstalledAt != nil {
		s := rec.InstalledAt.UTC().Format(time.RFC3339)
		installedAt = &s
	}
	_, err := r.db.ExecContext(ctx, q,
		rec.ID, servercontext.ID(ctx), rec.Name, string(rec.State), rec.Version, rec.Config,
		installedAt, rec.UpdatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

func (r *sqliteRepository) UpdateState(ctx context.Context, id string, state State) error {
	const q = `UPDATE modules SET state=?, updated_at=? WHERE id=? AND server_id=?`
	_, err := r.db.ExecContext(ctx, q, string(state), time.Now().UTC().Format(time.RFC3339), id, servercontext.ID(ctx))
	return err
}

func (r *sqliteRepository) scanRecord(row *sql.Row) (*Record, error) {
	var rec Record
	var version, installedAt *string
	var updatedAt string
	err := row.Scan(&rec.ID, &rec.Name, &rec.State, &version, &rec.Config, &installedAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	rec.Version = version
	rec.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if installedAt != nil {
		t, _ := time.Parse(time.RFC3339, *installedAt)
		rec.InstalledAt = &t
	}
	return &rec, nil
}

// ─── sqliteJobRepository ───────────────────────────────────────────────────

type sqliteJobRepository struct {
	db *sql.DB
}

// NewSQLiteJobRepository creates a JobRepository backed by SQLite.
func NewSQLiteJobRepository(db *sql.DB) JobRepository {
	return &sqliteJobRepository{db: db}
}

func (r *sqliteJobRepository) Create(ctx context.Context, job *Job) error {
	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	const q = `INSERT INTO jobs (id, server_id, name, type, payload, state, progress, output, created_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?, '', ?)`
	_, err := r.db.ExecContext(ctx, q,
		job.ID, servercontext.ID(ctx), job.Name, string(job.Type), job.Payload, string(job.State), job.Progress,
		job.CreatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

func (r *sqliteJobRepository) FindByID(ctx context.Context, id string) (*Job, error) {
	const q = `SELECT id, name, type, payload, state, progress, output, error, created_at, started_at, finished_at
	           FROM jobs WHERE id = ? AND server_id=?`
	row := r.db.QueryRowContext(ctx, q, id, servercontext.ID(ctx))
	return r.scanJob(row)
}

func (r *sqliteJobRepository) UpdateState(ctx context.Context, id string, state JobState, output, errMsg string) error {
	var q string
	var args []interface{}

	switch state {
	case JobRunning:
		q = `UPDATE jobs SET state=?, progress=10, started_at=? WHERE id=?`
		args = []interface{}{string(state), time.Now().UTC().Format(time.RFC3339), id}
	case JobSuccess:
		q = `UPDATE jobs SET state=?, progress=100, output=?, error=?, finished_at=? WHERE id=?`
		args = []interface{}{string(state), output, errMsg, time.Now().UTC().Format(time.RFC3339), id}
	case JobError, JobCanceled:
		q = `UPDATE jobs SET state=?, output=?, error=?, finished_at=? WHERE id=?`
		args = []interface{}{string(state), output, errMsg, time.Now().UTC().Format(time.RFC3339), id}
	default:
		q = `UPDATE jobs SET state=? WHERE id=?`
		args = []interface{}{string(state), id}
	}
	q += " AND server_id=?"
	args = append(args, servercontext.ID(ctx))
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *sqliteJobRepository) AppendOutput(ctx context.Context, id, line string) error {
	const q = `UPDATE jobs SET output = output || ? WHERE id = ? AND server_id=?`
	_, err := r.db.ExecContext(ctx, q, line+"\n", id, servercontext.ID(ctx))
	return err
}

func (r *sqliteJobRepository) ListByState(ctx context.Context, state JobState) ([]Job, error) {
	const q = `SELECT id, name, type, payload, state, progress, output, error, created_at, started_at, finished_at
	           FROM jobs WHERE state = ? AND server_id=? ORDER BY created_at DESC LIMIT 100`
	rows, err := r.db.QueryContext(ctx, q, string(state), servercontext.ID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		job, err := r.scanJobFromRows(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}
	return jobs, rows.Err()
}

func (r *sqliteJobRepository) List(ctx context.Context, filter JobFilter) (*JobPage, error) {
	where := []string{"server_id=?"}
	args := make([]any, 0, 5)
	args = append(args, servercontext.ID(ctx))
	if filter.Query != "" {
		where = append(where, "(LOWER(name) LIKE ? OR LOWER(id) LIKE ?)")
		query := "%" + strings.ToLower(filter.Query) + "%"
		args = append(args, query, query)
	}
	if filter.State != "" {
		where = append(where, "state = ?")
		args = append(args, string(filter.State))
	}
	if filter.Type != "" {
		where = append(where, "type = ?")
		args = append(args, string(filter.Type))
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs WHERE "+clause, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("job repo: list count: %w", err)
	}
	args = append(args, filter.Limit, filter.Offset)
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, type, payload, state, progress, output, error, created_at, started_at, finished_at
		FROM jobs WHERE `+clause+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("job repo: list: %w", err)
	}
	defer rows.Close()
	items := make([]Job, 0)
	for rows.Next() {
		job, err := r.scanJobFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *job)
	}
	return &JobPage{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, rows.Err()
}

func (r *sqliteJobRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM jobs WHERE id = ? AND server_id=? AND state IN ('success','error','canceled')`, id, servercontext.ID(ctx))
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return apperrors.New(409, apperrors.CodeConflict, "only completed task history can be deleted")
	}
	return nil
}

func (r *sqliteJobRepository) scanJob(row *sql.Row) (*Job, error) {
	var job Job
	var jobType, state, createdAt string
	var startedAt, finishedAt, errMsg *string

	err := row.Scan(
		&job.ID, &job.Name, &jobType, &job.Payload, &state, &job.Progress,
		&job.Output, &errMsg, &createdAt, &startedAt, &finishedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.NotFound("job")
		}
		return nil, fmt.Errorf("job repo: scan: %w", err)
	}
	job.Type = JobType(jobType)
	job.State = JobState(state)
	job.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if startedAt != nil {
		t, _ := time.Parse(time.RFC3339, *startedAt)
		job.StartedAt = &t
	}
	if finishedAt != nil {
		t, _ := time.Parse(time.RFC3339, *finishedAt)
		job.FinishedAt = &t
	}
	if errMsg != nil {
		job.Error = *errMsg
	}
	return &job, nil
}

func (r *sqliteJobRepository) scanJobFromRows(rows *sql.Rows) (*Job, error) {
	var job Job
	var jobType, state, createdAt string
	var startedAt, finishedAt, errMsg *string

	err := rows.Scan(
		&job.ID, &job.Name, &jobType, &job.Payload, &state, &job.Progress,
		&job.Output, &errMsg, &createdAt, &startedAt, &finishedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("job repo: scan row: %w", err)
	}
	job.Type = JobType(jobType)
	job.State = JobState(state)
	job.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if startedAt != nil {
		t, _ := time.Parse(time.RFC3339, *startedAt)
		job.StartedAt = &t
	}
	if finishedAt != nil {
		t, _ := time.Parse(time.RFC3339, *finishedAt)
		job.FinishedAt = &t
	}
	if errMsg != nil {
		job.Error = *errMsg
	}
	return &job, nil
}
