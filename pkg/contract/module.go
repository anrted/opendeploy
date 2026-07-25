// Package contract defines the public interfaces (contracts) for OpenDeploy.
//
// These interfaces are the only things that modules need to know about the core.
// They allow the module system to be truly decoupled: adding a new module
// requires zero changes to the core — only implementing these interfaces.
package contract

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// ─── Module ────────────────────────────────────────────────────────────────

// Module is the primary contract that every OpenDeploy module must implement.
// The core knows modules only through this interface; it never imports module
// packages directly.
type Module interface {
	// Metadata
	ID() string          // unique slug: "nginx", "php", "nodejs", "git"
	Name() string        // human-readable name: "Nginx Web Server"
	Version() string     // module spec version (not the installed package version)
	Description() string // one-line description for the UI

	// Lifecycle — called by the core during startup / shutdown.
	Bootstrap(deps ModuleDeps) error
	Shutdown(ctx context.Context) error

	// Registration — called by the core immediately after Bootstrap.
	RegisterRoutes(r Router)
	RegisterMenuItems() []MenuItem
	RegisterSettings() []SettingSpec

	// Management — every action is delegated to the Agent via deps.Agent.
	Install(ctx context.Context) error
	Uninstall(ctx context.Context) error
	Enable(ctx context.Context) error
	Disable(ctx context.Context) error
	Restart(ctx context.Context) error

	// Observability
	Status(ctx context.Context) (*ModuleStatus, error)
	HealthCheck(ctx context.Context) (*HealthReport, error)
}

// WebServerPlugin extends Module to provide web server configurations (Nginx, Apache).
type WebServerPlugin interface {
	Module
	ApplySite(ctx context.Context, action SiteAction, spec SiteSpec) error
}

// DatabasePlugin extends Module to provide database management (MySQL, PostgreSQL).
type DatabasePlugin interface {
	Module
	CreateDatabase(ctx context.Context, dbName, user, password string) error
	DeleteDatabase(ctx context.Context, dbName, user string) error
}

// ModuleDeps carries the infrastructure dependencies injected into every module
// by the core at Bootstrap time.
type ModuleDeps struct {
	Agent  AgentClient  // system operations through the Agent
	DB     *sql.DB      // direct database access for module-specific persistence
	Events EventBus     // in-process event bus
	Logger *slog.Logger // structured logger pre-configured with module context
	Config ModuleConfig // module-specific config (key-value map from YAML)
}

// ModuleConfig provides typed access to module-specific configuration.
type ModuleConfig map[string]string

// Get returns the value for key, or the provided default if absent.
func (mc ModuleConfig) Get(key, defaultVal string) string {
	if v, ok := mc[key]; ok {
		return v
	}
	return defaultVal
}

// ─── ModuleStatus / HealthReport ───────────────────────────────────────────

// ModuleState represents the lifecycle state of a module.
type ModuleState string

const (
	StateAvailable  ModuleState = "available"
	StateInstalling ModuleState = "installing"
	StateInstalled  ModuleState = "installed"
	StateEnabled    ModuleState = "enabled"
	StateDisabled   ModuleState = "disabled"
	StateRemoving   ModuleState = "removing"
	StateError      ModuleState = "error"
)

// ModuleStatus contains the runtime status of a module.
type ModuleStatus struct {
	State            ModuleState `json:"state"`
	InstalledVersion string      `json:"installed_version,omitempty"`
	ServiceRunning   bool        `json:"service_running"`
	Details          string      `json:"details,omitempty"`
}

// HealthStatus represents the result of a health check.
type HealthStatus string

const (
	HealthOK      HealthStatus = "ok"
	HealthWarning HealthStatus = "warning"
	HealthError   HealthStatus = "error"
)

// HealthReport is the result of Module.HealthCheck.
type HealthReport struct {
	Status  HealthStatus  `json:"status"`
	Message string        `json:"message,omitempty"`
	Checks  []HealthCheck `json:"checks,omitempty"`
}

// HealthCheck is a single named check within a HealthReport.
type HealthCheck struct {
	Name    string       `json:"name"`
	Status  HealthStatus `json:"status"`
	Message string       `json:"message,omitempty"`
}

// ─── Router ────────────────────────────────────────────────────────────────

// Router is a minimal interface for registering HTTP routes.
// Modules use this to add their own endpoints under /api/v1/modules/{id}/.
// The concrete implementation is chi.Router, but modules must not import chi.
type Router interface {
	// Get registers a GET handler at the given pattern.
	Get(pattern string, handlerFn func(w interface{}, r interface{}))
	// Post registers a POST handler at the given pattern.
	Post(pattern string, handlerFn func(w interface{}, r interface{}))
	// Put registers a PUT handler at the given pattern.
	Put(pattern string, handlerFn func(w interface{}, r interface{}))
	// Delete registers a DELETE handler at the given pattern.
	Delete(pattern string, handlerFn func(w interface{}, r interface{}))
}

// ─── MenuItem / SettingSpec ────────────────────────────────────────────────

// MenuItem describes a navigation item contributed by a module.
type MenuItem struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Icon     string `json:"icon,omitempty"` // icon name from the UI icon set
	Path     string `json:"path"`           // frontend route path
	Order    int    `json:"order"`
	ParentID string `json:"parent_id,omitempty"`
}

// SettingSpec describes a configurable setting exposed by a module.
type SettingSpec struct {
	Key          string      `json:"key"`
	Label        string      `json:"label"`
	Description  string      `json:"description,omitempty"`
	Type         SettingType `json:"type"` // "string" | "bool" | "int" | "select"
	DefaultValue string      `json:"default_value,omitempty"`
	Options      []string    `json:"options,omitempty"` // for type "select"
	Required     bool        `json:"required"`
	Secret       bool        `json:"secret"` // masked in UI
}

// SettingType defines the value type of a setting.
type SettingType string

const (
	SettingTypeString SettingType = "string"
	SettingTypeBool   SettingType = "bool"
	SettingTypeInt    SettingType = "int"
	SettingTypeSelect SettingType = "select"
)

// ─── AgentClient ───────────────────────────────────────────────────────────

// AgentClient is the interface through which the core and modules interact
// with the OpenDeploy Agent. All system-level operations must go through here.
// The Agent runs as root; the core runs as a normal user.
type AgentClient interface {
	// Service management (systemd)
	ServiceStart(ctx context.Context, name string) error
	ServiceStop(ctx context.Context, name string) error
	ServiceRestart(ctx context.Context, name string) error
	ServiceEnable(ctx context.Context, name string) error
	ServiceDisable(ctx context.Context, name string) error
	ServiceStatus(ctx context.Context, name string) (*ServiceStatus, error)
	ServiceLogs(ctx context.Context, name string, lines int) ([]string, error)

	// Package management
	PackageInstall(ctx context.Context, pkg string) (<-chan string, error)
	PackageRemove(ctx context.Context, pkg string) (<-chan string, error)
	PackageUpdate(ctx context.Context, pkg string) (<-chan string, error)
	PackageInstalled(ctx context.Context, pkg string) (bool, string, error)

	// File system
	FileRead(ctx context.Context, path string) ([]byte, error)
	FileWrite(ctx context.Context, path string, content []byte, mode uint32) error
	FileDelete(ctx context.Context, path string) error
	DirCreate(ctx context.Context, path string, mode uint32) error
	DirList(ctx context.Context, path string) ([]FileInfo, error)

	// Firewall
	FirewallAllow(ctx context.Context, port int, proto string) error
	FirewallDeny(ctx context.Context, port int, proto string) error
	FirewallList(ctx context.Context) ([]FirewallRule, error)

	// System information
	SystemStats(ctx context.Context) (*SystemStats, error)
}

type SiteAction string

const (
	SiteUpsert  SiteAction = "upsert"
	SiteDelete  SiteAction = "delete"
	SiteEnable  SiteAction = "enable"
	SiteDisable SiteAction = "disable"
)

type SiteSpec struct {
	Domain     string
	RootPath   string
	PHPVersion string
	SSLEnabled bool
	SSLCert    string
	SSLKey     string
}

// ServiceStatus represents the runtime state of a systemd service.
type ServiceStatus struct {
	Name        string `json:"name"`
	Active      bool   `json:"active"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description,omitempty"`
	SubState    string `json:"sub_state,omitempty"` // "running", "dead", etc.
}

// FileInfo represents metadata about a file or directory.
type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	Mode    uint32    `json:"mode"`
	ModTime time.Time `json:"mod_time"`
}

// FirewallRule represents a single firewall rule.
type FirewallRule struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // "tcp" | "udp"
	Action   string `json:"action"`   // "allow" | "deny"
}

// SystemStats contains a snapshot of system resource utilisation.
type SystemStats struct {
	CPU         CPUStats     `json:"cpu"`
	Memory      MemoryStats  `json:"memory"`
	Swap        SwapStats    `json:"swap"`
	Disk        []DiskStats  `json:"disk"`
	Network     NetworkStats `json:"network"`
	LoadAverage [3]float64   `json:"load_average"` // 1m, 5m, 15m
	Temperature float64      `json:"temperature"`  // CPU temp °C, 0 if unavailable
	Uptime      int64        `json:"uptime"`       // seconds
}

// CPUStats contains CPU utilisation metrics.
type CPUStats struct {
	UsagePercent float64 `json:"usage_percent"`
	Cores        int     `json:"cores"`
}

// MemoryStats contains RAM metrics.
type MemoryStats struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

// SwapStats contains swap metrics.
type SwapStats struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

// DiskStats contains per-mount-point disk metrics.
type DiskStats struct {
	Mountpoint  string  `json:"mountpoint"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

// NetworkStats contains network I/O metrics.
type NetworkStats struct {
	BytesSent   uint64 `json:"bytes_sent"`
	BytesRecv   uint64 `json:"bytes_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
}

// ─── EventBus ──────────────────────────────────────────────────────────────

// EventBus is the public contract for the in-process event bus.
// This mirrors events.Bus but lives in pkg/contract so modules can depend on
// it without importing the internal events package.
type EventBus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(eventType string, handler EventHandler) EventUnsubscribeFn
}

// Event is the interface all domain events must implement.
type Event interface {
	Type() string
	Payload() any
	OccurredAt() time.Time
}

// EventHandler processes an event.
type EventHandler func(ctx context.Context, event Event) error

// EventUnsubscribeFn removes an event subscription when called.
type EventUnsubscribeFn func()
