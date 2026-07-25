// Package systemd provides systemd service management for the Agent.
package systemd

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/anrted/opendeploy/internal/agent/executor"
)

// ServiceStatus is the result of a service status query.
type ServiceStatus struct {
	Name        string
	Active      bool
	Enabled     bool
	SubState    string
	Description string
	Since       *time.Time
}

// Manager handles systemd operations via systemctl.
type Manager struct {
	shell  *executor.Shell
	logger *slog.Logger
}

// NewManager creates a systemd Manager.
func NewManager(shell *executor.Shell, logger *slog.Logger) *Manager {
	return &Manager{shell: shell, logger: logger}
}

// Start starts a systemd service.
func (m *Manager) Start(ctx context.Context, name string) error {
	_, err := m.shell.Run(ctx, "systemctl", "start", name)
	if err != nil {
		// If it's a newly created service, systemd might need a reload.
		_, _ = m.shell.Run(ctx, "systemctl", "daemon-reload")
		_, errRetry := m.shell.Run(ctx, "systemctl", "start", name)
		if errRetry != nil {
			return fmt.Errorf("systemd: start %s: %w", name, errRetry)
		}
	}
	return nil
}

// Stop stops a systemd service.
func (m *Manager) Stop(ctx context.Context, name string) error {
	_, err := m.shell.Run(ctx, "systemctl", "stop", name)
	if err != nil {
		return fmt.Errorf("systemd: stop %s: %w", name, err)
	}
	return nil
}

// Restart restarts a systemd service.
func (m *Manager) Restart(ctx context.Context, name string) error {
	_, err := m.shell.Run(ctx, "systemctl", "restart", name)
	if err != nil {
		_, _ = m.shell.Run(ctx, "systemctl", "daemon-reload")
		_, errRetry := m.shell.Run(ctx, "systemctl", "restart", name)
		if errRetry != nil {
			return fmt.Errorf("systemd: restart %s: %w", name, errRetry)
		}
	}
	return nil
}

// Enable enables a service for automatic startup.
func (m *Manager) Enable(ctx context.Context, name string) error {
	_, err := m.shell.Run(ctx, "systemctl", "enable", name)
	if err != nil {
		_, _ = m.shell.Run(ctx, "systemctl", "daemon-reload")
		_, errRetry := m.shell.Run(ctx, "systemctl", "enable", name)
		if errRetry != nil {
			return fmt.Errorf("systemd: enable %s: %w", name, errRetry)
		}
	}
	return nil
}

// Disable disables automatic startup for a service.
func (m *Manager) Disable(ctx context.Context, name string) error {
	_, err := m.shell.Run(ctx, "systemctl", "disable", name)
	if err != nil {
		return fmt.Errorf("systemd: disable %s: %w", name, err)
	}
	return nil
}

// Status returns the runtime status of a service.
func (m *Manager) Status(ctx context.Context, name string) (*ServiceStatus, error) {
	activeResult, _ := m.shell.Run(ctx, "systemctl", "is-active", name)
	enabledResult, _ := m.shell.Run(ctx, "systemctl", "is-enabled", name)
	subStateResult, _ := m.shell.Run(ctx, "systemctl", "show", "-p", "SubState", "--value", name)

	active := false
	if activeResult != nil {
		active = strings.TrimSpace(activeResult.Stdout) == "active"
	}
	enabled := false
	if enabledResult != nil {
		enabled = strings.TrimSpace(enabledResult.Stdout) == "enabled"
	}
	subState := ""
	if subStateResult != nil {
		subState = strings.TrimSpace(subStateResult.Stdout)
	}

	return &ServiceStatus{
		Name:     name,
		Active:   active,
		Enabled:  enabled,
		SubState: subState,
	}, nil
}

// Logs returns the last n lines from the service journal.
func (m *Manager) Logs(ctx context.Context, name string, lines int) ([]string, error) {
	result, err := m.shell.Run(ctx, "journalctl",
		"-u", name,
		"-n", strconv.Itoa(lines),
		"--no-pager",
		"-o", "short",
	)
	if err != nil {
		return nil, fmt.Errorf("systemd: logs %s: %w", name, err)
	}
	return strings.Split(strings.TrimSpace(result.Stdout), "\n"), nil
}
