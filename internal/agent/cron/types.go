package cron

import "time"

type JobType string

const (
	JobTypeUnknown    JobType = "UNKNOWN"
	JobTypeSystem     JobType = "SYSTEM"
	JobTypePackage    JobType = "PACKAGE"
	JobTypeOpenDeploy JobType = "OPENDEPLOY"
	JobTypeUser       JobType = "USER"
)

type Job struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Command     string            `json:"command"`
	WorkingDir  string            `json:"working_dir,omitempty"`
	User        string            `json:"user"`
	Environment map[string]string `json:"environment,omitempty"`
	Expression  string            `json:"expression"`
	Timezone    string            `json:"timezone,omitempty"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Source      string            `json:"source,omitempty"`
	ReadOnly    bool              `json:"read_only,omitempty"`
	Type        JobType           `json:"type,omitempty"`
	PackageName string            `json:"package_name,omitempty"`
	IsProtected bool              `json:"is_protected,omitempty"`
	CanEdit     bool              `json:"can_edit,omitempty"`
	CanDelete   bool              `json:"can_delete,omitempty"`
	LockReason  string            `json:"lock_reason,omitempty"`
}

type Run struct {
	ID         string        `json:"id"`
	JobID      string        `json:"job_id"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt time.Time     `json:"finished_at"`
	Duration   time.Duration `json:"duration"`
	ExitCode   int           `json:"exit_code"`
	Stdout     string        `json:"stdout,omitempty"`
	Stderr     string        `json:"stderr,omitempty"`
	Triggered  string        `json:"triggered"`
	Actor      string        `json:"actor,omitempty"`
}

type Store struct {
	Jobs    []Job `json:"jobs"`
	History []Run `json:"history"`
}

type Validation struct {
	Valid    bool     `json:"valid"`
	Warnings []string `json:"warnings,omitempty"`
}
