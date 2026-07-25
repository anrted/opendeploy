// Package module implements the module registry and lifecycle management for OpenDeploy.
//
// The core domain entity is Module (defined in pkg/contract). This package
// owns the registry, the database-backed state, and the async job system that
// runs Install/Uninstall/Enable/Disable operations in the background.
package module

import (
	"time"
)

// State mirrors contract.ModuleState and is used for database storage.
type State string

const (
	StateAvailable  State = "available"
	StateInstalling State = "installing"
	StateInstalled  State = "installed"
	StateEnabled    State = "enabled"
	StateDisabled   State = "disabled"
	StateRemoving   State = "removing"
	StateError      State = "error"
)

// Record is the database representation of a module's persistent state.
// It is separate from the contract.Module interface so we can store state
// without coupling to module implementation details.
type Record struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	State       State      `json:"state"`
	Version     *string    `json:"version,omitempty"`
	Config      string     `json:"config"` // JSON blob
	InstalledAt *time.Time `json:"installed_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// JobType enumerates the types of async module operations.
type JobType string

const (
	JobInstall   JobType = "install_module"
	JobUninstall JobType = "uninstall_module"
	JobEnable    JobType = "enable_module"
	JobDisable   JobType = "disable_module"
	JobRestart   JobType = "restart_module"
)

// JobState enumerates async job states.
type JobState string

const (
	JobPending JobState = "pending"
	JobRunning JobState = "running"
	JobSuccess JobState = "success"
	JobError   JobState = "error"
)

// Job represents a background operation.
type Job struct {
	ID         string     `json:"id"`
	Type       JobType    `json:"type"`
	Payload    string     `json:"payload"` // JSON
	State      JobState   `json:"state"`
	Output     string     `json:"output"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}
