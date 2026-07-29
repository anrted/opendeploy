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
	Category() string
	Icon() string
	Dependencies() ModuleDependencies
	Capabilities() ModuleCapabilities

	// Pages returns the dynamic pages (tabs) this module provides in the UI.
	Pages() []ModulePage

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
	Status(ctx context.Context) (*RuntimeStatus, error)
	HealthCheck(ctx context.Context) (*HealthReport, error)

	// Universal Page (Metadata-driven UI)
	Actions() []ActionDef
	ExecuteAction(ctx context.Context, actionID string) error
	Logs() []LogDef
	SettingsSchema() []SettingField
}

// WebServerPlugin extends Module to provide web server configurations (Nginx, Apache).
type WebServerPlugin interface {
	Module
	ApplySite(ctx context.Context, action SiteAction, spec SiteSpec) error
}

// ModuleDependencies describes the requirements and conflicts of a module.
type ModuleDependencies struct {
	Required    []string `json:"required"`
	Recommended []string `json:"recommended"`
	Conflicts   []string `json:"conflicts"`
}

// ModuleCapabilities defines what features the module supports.
// Deprecated: Moving towards dynamic metadata (Actions, Logs, SettingsSchema)
type ModuleCapabilities struct {
	SupportsService  bool `json:"supports_service"`
	SupportsSettings bool `json:"supports_settings"`
	SupportsLogs     bool `json:"supports_logs"`
	SupportsRestart  bool `json:"supports_restart"`
	SupportsUpdate   bool `json:"supports_update"`
}

// ActionDef describes a dynamically available action on the module page.
type ActionDef struct {
	ID                   string           `json:"id"`
	Title                string           `json:"title"`
	Description          string           `json:"description,omitempty"`
	Icon                 string           `json:"icon"`  // e.g. "play", "stop", "refresh", "trash"
	Color                string           `json:"color"` // e.g. "primary", "danger", "warning", "success"
	RequiresConfirmation bool             `json:"requiresConfirmation"`
	Dangerous            bool             `json:"dangerous"`
	Disabled             bool             `json:"disabled,omitempty"`
	Inputs               []ActionInputDef `json:"inputs,omitempty"`
}

// ActionInputDef describes a value collected by the generic action UI and
// passed to DataGridAction in its payload.
type ActionInputDef struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Placeholder string `json:"placeholder,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// ActionAvailabilityProvider lets a module expose runtime availability for
// metadata-driven actions. Missing action IDs remain available by default.
type ActionAvailabilityProvider interface {
	ActionAvailability(ctx context.Context) map[string]bool
}

// PageType defines the type of page in the UI.
type PageType string

const (
	PageTypeOverview PageType = "overview"
	PageTypeSettings PageType = "settings"
	PageTypeLogs     PageType = "logs"
	PageTypeDataGrid PageType = "datagrid"
)

// ModulePage describes a tab/page contributed by a module.
type ModulePage struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Type  PageType `json:"type"`
}

// LogDef describes a log source exposed by the module.
type LogDef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`           // "systemd" or "file"
	Path string `json:"path,omitempty"` // For file type logs
}

// SettingField describes a dynamic configuration field.
type SettingField struct {
	ID              string   `json:"id"`
	Type            string   `json:"type"` // "string", "number", "boolean", "select"
	Label           string   `json:"label"`
	Description     string   `json:"description,omitempty"`
	Value           any      `json:"value"`
	Options         []string `json:"options,omitempty"`
	Category        string   `json:"category,omitempty"`
	Advanced        bool     `json:"advanced,omitempty"`
	RequiresRestart bool     `json:"requires_restart,omitempty"`
	ValidationRegex string   `json:"validation_regex,omitempty"`
	Placeholder     string   `json:"placeholder,omitempty"`
}

// SettingsProvider is implemented by modules that want to support dynamic setting updates.
type SettingsProvider interface {
	SaveSettings(ctx context.Context, settings map[string]any) error
}

// ProtectionPreset describes a managed security configuration exposed by a
// module. Values are read from the actual managed files when they exist.
type ProtectionPreset struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Icon         string         `json:"icon"`
	Enabled      bool           `json:"enabled"`
	NeedsUpdate  bool           `json:"needs_update,omitempty"`
	Jails        []string       `json:"jails"`
	RuleCount    int            `json:"rule_count"`
	LastModified *time.Time     `json:"last_modified,omitempty"`
	Settings     map[string]any `json:"settings"`
	Defaults     map[string]any `json:"defaults"`
	Files        []string       `json:"files"`
	Filters      []string       `json:"filters"`
	LogPaths     []string       `json:"log_paths"`
	Actions      []string       `json:"actions"`
	BlockedIPs   string         `json:"blocked_ips"`
	Exceptions   string         `json:"exceptions"`
}

type ProtectionPresetPreview struct {
	PresetID      string         `json:"preset_id"`
	Jails         []string       `json:"jails"`
	Parameters    map[string]any `json:"parameters"`
	Files         []string       `json:"files"`
	Services      []string       `json:"services"`
	Configuration string         `json:"configuration"`
	Filter        string         `json:"filter,omitempty"`
}

// ProtectionPresetProvider powers the compact protection cards and their
// details/configuration workflows.
type ProtectionPresetProvider interface {
	ProtectionPresets(ctx context.Context) ([]ProtectionPreset, error)
	PreviewProtectionPreset(ctx context.Context, presetID string, settings map[string]any) (*ProtectionPresetPreview, error)
	SaveProtectionPreset(ctx context.Context, presetID string, settings map[string]any) error
	ResetProtectionPreset(ctx context.Context, presetID string) error
	SetProtectionPresetEnabled(ctx context.Context, presetID string, enabled bool) error
}

// LogProvider is implemented by modules that want to expose log reading and clearing.
type LogProvider interface {
	ReadLog(ctx context.Context, logID string, lines int) ([]string, error)
	ClearLog(ctx context.Context, logID string) error
}

// DatabasePlugin extends Module to provide database management (MySQL, PostgreSQL).
type DatabasePlugin interface {
	Module
	CreateDatabase(ctx context.Context, dbName, user, password string) error
	DeleteDatabase(ctx context.Context, dbName, user string) error
}

// CertbotPlugin extends Module to provide SSL certificate provisioning.
type CertbotPlugin interface {
	Module
	ObtainCert(ctx context.Context, domain, webroot string) error
}

// ModuleDeps carries the infrastructure dependencies injected into every module
// by the core at Bootstrap time.
type ModuleDeps struct {
	Agent  AgentClient  // system operations through the Agent
	DB     *sql.DB      // direct database access for module-specific persistence
	Events EventBus     // in-process event bus
	Logger *slog.Logger // structured logger pre-configured with module context
	Config ModuleConfig // module-specific config (key-value map from YAML)
	Tasks  TaskRunner   // background operations exposed in Task Manager
}

type TaskRunner interface {
	StartTask(ctx context.Context, name, taskType string, payload map[string]string, work func(context.Context) (string, error)) (string, error)
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

// PackageStatus represents the OS package state.
type PackageStatus string

const (
	PackageInstalled    PackageStatus = "installed"
	PackageNotInstalled PackageStatus = "not_installed"
	PackageBroken       PackageStatus = "broken"
)

// ServiceStatusState represents the systemd service state.
type ServiceStatusState string

const (
	ServiceRunning ServiceStatusState = "running"
	ServiceStopped ServiceStatusState = "stopped"
	ServiceFailed  ServiceStatusState = "failed"
)

// Property represents a generic key-value metric or status property.
type Property struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Group string `json:"group,omitempty"`
}

// RuntimeStatus contains the runtime status of a module.
type RuntimeStatus struct {
	PackageStatus   PackageStatus      `json:"packageStatus"`
	ServiceStatus   ServiceStatusState `json:"serviceStatus,omitempty"`
	SoftwareVersion string             `json:"softwareVersion"`
	Health          HealthStatus       `json:"health"`
	Details         string             `json:"details,omitempty"`
	Properties      []Property         `json:"properties,omitempty"`
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
	Key             string      `json:"key"`
	Label           string      `json:"label"`
	Description     string      `json:"description,omitempty"`
	Type            SettingType `json:"type"` // "string" | "bool" | "int" | "select"
	DefaultValue    string      `json:"default_value,omitempty"`
	Options         []string    `json:"options,omitempty"` // for type "select"
	Required        bool        `json:"required"`
	Secret          bool        `json:"secret"` // masked in UI
	ValidationRegex string      `json:"validation_regex,omitempty"`
	ValidationMsg   string      `json:"validation_msg,omitempty"`
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
	ServiceReload(ctx context.Context, name string) error
	ServiceStatus(ctx context.Context, name string) (*ServiceStatus, error)
	ServiceLogs(ctx context.Context, name string, lines int) ([]string, error)
	ServiceStreamLogs(ctx context.Context, name string) (<-chan string, error)
	FileLogs(ctx context.Context, path string, lines int) ([]string, error)
	StreamLogs(ctx context.Context, path string) (<-chan string, error)

	CommandExecute(ctx context.Context, command string, args ...string) (int, string, string, error)

	// Package management
	PackageInstall(ctx context.Context, pkg string) (<-chan string, error)
	PackageRemove(ctx context.Context, pkg string) (<-chan string, error)
	PackageUpdate(ctx context.Context, pkg string) (<-chan string, error)
	PackageInstalled(ctx context.Context, pkg string) (bool, string, error)

	// File system
	FileRead(ctx context.Context, path string) ([]byte, error)
	FileWrite(ctx context.Context, path string, content []byte, mode uint32) error
	FileDelete(ctx context.Context, path string) error
	FileRename(ctx context.Context, oldPath, newPath string) error
	FileCopy(ctx context.Context, srcPath, dstPath string) error
	FileChmod(ctx context.Context, path string, mode uint32) error
	FileChown(ctx context.Context, path string, uid, gid int) error
	DirCreate(ctx context.Context, path string, mode uint32) error
	DirList(ctx context.Context, path string) ([]FileInfo, error)
	ArchiveCreate(ctx context.Context, paths []string, dstPath string) error
	ArchiveExtract(ctx context.Context, srcPath, dstDir string) error

	// Firewall
	FirewallStatus(ctx context.Context) (*FirewallStatus, error)
	FirewallRule(ctx context.Context, req *FirewallRuleRequest) error
	FirewallDelete(ctx context.Context, id string) error
	FirewallList(ctx context.Context) ([]*FirewallRule, error)
	FirewallToggle(ctx context.Context, enable bool) error
	FirewallReset(ctx context.Context) error

	// System information
	SystemStats(ctx context.Context) (*SystemStats, error)
	ProcessList(ctx context.Context) ([]ProcessStats, error)
	ProcessKill(ctx context.Context, pid int, force bool) error
}

// CronAgentClient is an optional typed extension implemented by the real
// Agent client. Keeping it separate preserves compatibility for external
// modules and test doubles that implement AgentClient.
type CronAgentClient interface {
	CronList(ctx context.Context) ([]CronJob, error)
	CronGet(ctx context.Context, id string) (*CronJob, error)
	CronCreate(ctx context.Context, job CronJob) (*CronJob, error)
	CronUpdate(ctx context.Context, job CronJob) (*CronJob, error)
	CronDelete(ctx context.Context, id string) error
	CronEnable(ctx context.Context, id string) (*CronJob, error)
	CronDisable(ctx context.Context, id string) (*CronJob, error)
	CronRun(ctx context.Context, id, trigger, actor string) (*CronRun, error)
	CronHistory(ctx context.Context, id string, limit int) ([]CronRun, error)
	CronValidate(ctx context.Context, job CronJob) (*CronValidation, error)
}

type CronJob struct {
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

type CronRun struct {
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

type CronValidation struct {
	Valid    bool     `json:"valid"`
	Warnings []string `json:"warnings,omitempty"`
	Error    string   `json:"error,omitempty"`
}

type SiteAction string

const (
	SiteUpsert  SiteAction = "upsert"
	SiteDelete  SiteAction = "delete"
	SiteEnable  SiteAction = "enable"
	SiteDisable SiteAction = "disable"
)

type SiteSpec struct {
	ID            string
	Name          string
	PrimaryDomain string
	Aliases       []string
	RootPath      string
	AppType       string // "php", "static", "proxy"
	AppVersion    string // e.g. "8.3"
	ProxyTarget   string
	SSLEnabled    bool
	SSLCert       string
	SSLKey        string
	ForceHTTPS    bool
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
	Owner   string    `json:"owner"`
	Group   string    `json:"group"`
}

// FirewallStatus represents the state of the firewall.
type FirewallStatus struct {
	Active          bool   `json:"active"`
	DefaultIncoming string `json:"default_incoming"`
	DefaultOutgoing string `json:"default_outgoing"`
	DefaultRouted   string `json:"default_routed"`
	IPv6Enabled     bool   `json:"ipv6_enabled"`
	Logging         string `json:"logging"`
	RuleCount       int    `json:"rule_count"`
	ProfileName     string `json:"profile_name"`
}

// FirewallRuleRequest is used to add a new rule.
type FirewallRuleRequest struct {
	ID          string `json:"id,omitempty"`
	Port        string `json:"port"`
	Protocol    string `json:"protocol"`
	Action      string `json:"action"`
	Direction   string `json:"direction,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	Comment     string `json:"comment,omitempty"`
	IPVersion   string `json:"ip_version,omitempty"`
}

// FirewallRule represents a single firewall rule.
type FirewallRule struct {
	ID          string `json:"id"`
	Port        string `json:"port"`
	Protocol    string `json:"protocol"` // "tcp" | "udp" | "any"
	Action      string `json:"action"`   // "allow" | "deny" | "reject"
	Direction   string `json:"direction"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Comment     string `json:"comment"`
	IPVersion   string `json:"ip_version"`
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

// ProcessStats contains metrics for a single running process.
type ProcessStats struct {
	Pid        int     `json:"pid"`
	Ppid       int     `json:"ppid"`
	Name       string  `json:"name"`
	User       string  `json:"user"`
	CpuPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
	MemRss     uint64  `json:"mem_rss"`
	NumThreads int     `json:"num_threads"`
	CreateTime int64   `json:"create_time"`
	Cmdline    string  `json:"cmdline"`
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

// ─── DataGrid ──────────────────────────────────────────────────────────────

// DataGridProvider is implemented by modules that want to expose tabular data
// via the PageTypeDataGrid system.
type DataGridProvider interface {
	// DataGridSchema returns the schema (columns, supported actions) for a given pageID.
	DataGridSchema(pageID string) (DataGridSchema, error)
	// DataGridData returns the actual rows of data for a given pageID.
	DataGridData(ctx context.Context, pageID string) ([]map[string]any, error)
	// DataGridAction executes an action (like Delete, Unban, Create) on the datagrid.
	DataGridAction(ctx context.Context, pageID string, actionID string, payload map[string]any) error
}

// DataGridSchema describes the layout and available actions of a datagrid page.
type DataGridSchema struct {
	Columns    []DataGridColumn `json:"columns"`
	Actions    []ActionDef      `json:"actions"`
	RowActions []ActionDef      `json:"row_actions,omitempty"`
}

// DataGridColumn describes a single column in the datagrid.
type DataGridColumn struct {
	Key      string `json:"key"`
	Title    string `json:"title"`
	Type     string `json:"type"` // "text", "badge", "number", "date"
	Sortable bool   `json:"sortable"`
}
