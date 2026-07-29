package cron

import "time"

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
