// Package backup implements verified, portable OpenDeploy system backups.
package backup

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

const Schema = "opendeploy.backup/v1"

type Source struct {
	ID       string
	Path     string
	Required bool
	Database bool
	File     bool
}

type Config struct {
	BackupDir  string
	StateDir   string
	Sources    []Source
	MaxEntries int
	MaxBytes   int64
}

type RestoreRuntime interface {
	BeforeRestore(ctx context.Context) error
	AfterRestore(ctx context.Context) error
}

func DefaultConfig() Config {
	return Config{
		BackupDir:  "/var/lib/opendeploy/backups",
		StateDir:   "/var/lib/opendeploy/backup-state",
		MaxEntries: 1_000_000,
		MaxBytes:   500 << 30,
		Sources: []Source{
			{ID: "panel", Path: "/etc/opendeploy", Required: true},
			{ID: "database", Path: "/var/lib/opendeploy/data.db", Required: true, Database: true},
			{ID: "sites", Path: "/var/www"},
			{ID: "ssl", Path: "/etc/letsencrypt"},
			{ID: "nginx", Path: "/etc/nginx"},
			{ID: "fail2ban", Path: "/etc/fail2ban"},
			{ID: "cron", Path: "/etc/cron.d/opendeploy", File: true},
			{ID: "cron-state", Path: "/var/lib/opendeploy/cron"},
			{ID: "ufw", Path: "/etc/ufw"},
			{ID: "nftables", Path: "/etc/nftables.conf", File: true},
			{ID: "apache", Path: "/etc/apache2"},
			{ID: "php", Path: "/etc/php"},
			{ID: "mysql", Path: "/etc/mysql"},
			{ID: "postgresql", Path: "/etc/postgresql"},
			{ID: "systemd-core", Path: "/etc/systemd/system/opendeploy-core.service", File: true},
			{ID: "systemd-agent", Path: "/etc/systemd/system/opendeploy-agent.service", File: true},
			{ID: "systemd-update", Path: "/etc/systemd/system/opendeploy-update.service", File: true},
			{ID: "systemd-update-path", Path: "/etc/systemd/system/opendeploy-update.path", File: true},
		},
	}
}

type Manifest struct {
	Schema     string    `json:"schema"`
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	Reason     string    `json:"reason"`
	Host       string    `json:"host,omitempty"`
	OpenDeploy string    `json:"opendeploy_version,omitempty"`
	Entries    []Entry   `json:"entries"`
	TotalBytes int64     `json:"total_bytes"`
}

type Entry struct {
	SourceID string `json:"source_id"`
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Mode     uint32 `json:"mode"`
}

type Operation struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	BackupID   string    `json:"backup_id"`
	Archive    string    `json:"archive"`
	Reason     string    `json:"reason,omitempty"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Error      string    `json:"error,omitempty"`
}

func (c Config) validate() error {
	if !filepath.IsAbs(c.BackupDir) || !filepath.IsAbs(c.StateDir) {
		return fmt.Errorf("backup: backup and state directories must be absolute")
	}
	seen := make(map[string]bool, len(c.Sources))
	for _, source := range c.Sources {
		if !safeSourceID(source.ID) || seen[source.ID] || !filepath.IsAbs(source.Path) {
			return fmt.Errorf("backup: invalid or duplicate source %q", source.ID)
		}
		seen[source.ID] = true
	}
	if c.MaxEntries <= 0 || c.MaxBytes <= 0 {
		return fmt.Errorf("backup: extraction limits must be positive")
	}
	return nil
}

func safeSourceID(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < 'a' || character > 'z' {
			if character < '0' || character > '9' {
				if character != '-' && character != '_' {
					return false
				}
			}
		}
	}
	return true
}
