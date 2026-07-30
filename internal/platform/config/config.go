// Package config handles loading and validation of OpenDeploy configuration.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration structure for OpenDeploy Core.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Agent    AgentConfig    `yaml:"agent"`
	Auth     AuthConfig     `yaml:"auth"`
	Security SecurityConfig `yaml:"security"`
	Logging  LoggingConfig  `yaml:"logging"`
	Modules  ModulesConfig  `yaml:"modules"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host             string        `yaml:"host"`
	Port             int           `yaml:"port"`
	ControlPlanePort int           `yaml:"control_plane_port"`
	TLSCertificate   string        `yaml:"tls_certificate"`
	TLSPrivateKey    string        `yaml:"tls_private_key"`
	ReadTimeout      time.Duration `yaml:"read_timeout"`
	WriteTimeout     time.Duration `yaml:"write_timeout"`
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

// AgentConfig holds settings for the gRPC connection to the Agent.
type AgentConfig struct {
	// Socket is the path to the Unix domain socket used by the Agent.
	Socket                 string        `yaml:"socket"`
	Timeout                time.Duration `yaml:"timeout"`
	CoreURL                string        `yaml:"core_url"`
	ServerID               string        `yaml:"server_id"`
	CertificateFile        string        `yaml:"certificate_file"`
	PrivateKeyFile         string        `yaml:"private_key_file"`
	CertificateFingerprint string        `yaml:"certificate_fingerprint"`
	HeartbeatInterval      time.Duration `yaml:"heartbeat_interval"`
	ControlPlaneAddress    string        `yaml:"control_plane_address"`
	ControlPlaneCAFile     string        `yaml:"control_plane_ca_file"`
	ControlPlaneServerName string        `yaml:"control_plane_server_name"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	JWTSecret       string        `yaml:"jwt_secret"`
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl"`
}

// SecurityConfig holds security-related settings.
type SecurityConfig struct {
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	CSRF      CSRFConfig      `yaml:"csrf"`
}

// RateLimitConfig configures request rate limiting.
type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerMinute int  `yaml:"requests_per_minute"`
}

// CSRFConfig configures CSRF protection.
type CSRFConfig struct {
	Enabled bool `yaml:"enabled"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	File   string `yaml:"file"`
}

// ModulesConfig defines which modules are enabled at startup.
type ModulesConfig struct {
	Enabled []string `yaml:"enabled"`
}

// defaults returns a Config with sensible default values.
func defaults() Config {
	return Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         5888,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 60 * time.Second,
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    "/var/lib/opendeploy/data.db",
		},
		Agent: AgentConfig{
			Socket:            "/run/opendeploy-agent/agent.sock",
			Timeout:           120 * time.Second,
			HeartbeatInterval: 30 * time.Second,
		},
		Auth: AuthConfig{
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 7 * 24 * time.Hour,
		},
		Security: SecurityConfig{
			RateLimit: RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 300,
			},
			CSRF: CSRFConfig{Enabled: true},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Modules: ModulesConfig{
			Enabled: []string{"nginx", "php", "nodejs", "git", "certbot"},
		},
	}
}

// Load reads a YAML configuration file from the given path and merges it
// with default values. Environment variable OD_JWT_SECRET overrides
// auth.jwt_secret when present.
func Load(path string) (*Config, error) {
	cfg := defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse yaml: %w", err)
	}

	// Allow overriding the JWT secret via environment (safer than config file).
	if secret := os.Getenv("OD_JWT_SECRET"); secret != "" {
		cfg.Auth.JWTSecret = secret
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: validation: %w", err)
	}

	return &cfg, nil
}

// validate performs basic sanity checks on the loaded configuration.
func (c *Config) validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port)
	}
	if c.Server.ControlPlanePort < 0 || c.Server.ControlPlanePort > 65535 {
		return fmt.Errorf("server.control_plane_port must be between 0 and 65535, got %d", c.Server.ControlPlanePort)
	}
	if c.Server.ControlPlanePort > 0 && (c.Server.TLSCertificate == "" || c.Server.TLSPrivateKey == "") {
		return fmt.Errorf("server.tls_certificate and server.tls_private_key are required when the control plane is enabled")
	}
	if c.Database.DSN == "" {
		return fmt.Errorf("database.dsn must not be empty")
	}
	if c.Agent.Socket == "" {
		return fmt.Errorf("agent.socket must not be empty")
	}
	return nil
}

// Addr returns the combined host:port string for the HTTP server.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
