package remote

import "time"

type Server struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Hostname      string     `json:"hostname"`
	Description   string     `json:"description"`
	MachineID     string     `json:"machine_id"`
	Status        string     `json:"status"`
	AgentVersion  string     `json:"agent_version"`
	OS            string     `json:"os"`
	Distribution  string     `json:"distribution"`
	OSVersion     string     `json:"os_version"`
	Kernel        string     `json:"kernel"`
	Architecture  string     `json:"architecture"`
	CPUModel      string     `json:"cpu_model"`
	CPUCores      int        `json:"cpu_cores"`
	RAMTotal      int64      `json:"ram_total"`
	DiskTotal     int64      `json:"disk_total"`
	PublicIP      string     `json:"public_ip"`
	PrivateIP     string     `json:"private_ip"`
	Uptime        int64      `json:"uptime"`
	LatencyMS     int64      `json:"latency_ms"`
	Tags          []string   `json:"tags"`
	UpdateChannel string     `json:"update_channel"`
	HealthStatus  string     `json:"health_status"`
	Maintenance   bool       `json:"maintenance"`
	LastHeartbeat *time.Time `json:"last_heartbeat,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type CreateRequest struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Tags            []string `json:"tags"`
	OperatingSystem string   `json:"operating_system"`
	UpdateChannel   string   `json:"update_channel"`
}

type Enrollment struct {
	Server              Server    `json:"server"`
	RegistrationToken   string    `json:"registration_token"`
	RegistrationCode    string    `json:"registration_code"`
	ExpiresAt           time.Time `json:"expires_at"`
	InstallationCommand string    `json:"installation_command"`
}

type RegistrationRequest struct {
	Token        string `json:"token"`
	MachineID    string `json:"machine_id"`
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Distribution string `json:"distribution"`
	OSVersion    string `json:"os_version"`
	Kernel       string `json:"kernel"`
	Architecture string `json:"architecture"`
	CPUModel     string `json:"cpu_model"`
	CPUCores     int    `json:"cpu_cores"`
	RAMTotal     int64  `json:"ram_total"`
	DiskTotal    int64  `json:"disk_total"`
	PublicIP     string `json:"public_ip"`
	PrivateIP    string `json:"private_ip"`
	AgentVersion string `json:"agent_version"`
	CSRPEM       string `json:"csr"`
}

type RegistrationResponse struct {
	ServerID          string   `json:"server_id"`
	CertificatePEM    string   `json:"certificate"`
	Fingerprint       string   `json:"fingerprint"`
	HeartbeatInterval int      `json:"heartbeat_interval"`
	UpdateChannel     string   `json:"update_channel"`
	AllowedFeatures   []string `json:"allowed_features"`
}

type HeartbeatRequest struct {
	State        string       `json:"state"`
	CPUUsage     float64      `json:"cpu_usage"`
	MemoryUsage  float64      `json:"memory_usage"`
	DiskUsage    float64      `json:"disk_usage"`
	Uptime       int64        `json:"uptime"`
	RunningTasks int          `json:"running_tasks"`
	AgentVersion string       `json:"agent_version"`
	TaskResults  []TaskResult `json:"task_results,omitempty"`
}

type Task struct {
	ID         string     `json:"id"`
	ServerID   string     `json:"server_id"`
	Action     string     `json:"action"`
	Payload    string     `json:"payload"`
	State      string     `json:"state"`
	Output     string     `json:"output"`
	Error      string     `json:"error"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

type TaskResult struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Output string `json:"output"`
	Error  string `json:"error"`
}

type HeartbeatResponse struct {
	HeartbeatInterval int    `json:"heartbeat_interval"`
	Tasks             []Task `json:"tasks"`
}

type Page struct {
	Items  []Server `json:"items"`
	Total  int      `json:"total"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
}

type Event struct {
	ID        int64     `json:"id"`
	ServerID  string    `json:"server_id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}

type Heartbeat struct {
	ID           int64     `json:"id"`
	ServerID     string    `json:"server_id"`
	State        string    `json:"state"`
	CPUUsage     float64   `json:"cpu_usage"`
	MemoryUsage  float64   `json:"memory_usage"`
	DiskUsage    float64   `json:"disk_usage"`
	Uptime       int64     `json:"uptime"`
	RunningTasks int       `json:"running_tasks"`
	AgentVersion string    `json:"agent_version"`
	LatencyMS    int64     `json:"latency_ms"`
	CreatedAt    time.Time `json:"created_at"`
}
