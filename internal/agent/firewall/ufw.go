// Package firewall provides UFW firewall management for the Agent.
package firewall

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/anrted/opendeploy/internal/agent/executor"
)

// Rule represents a single firewall rule.
type Rule struct {
	Port     int
	Protocol string
	Action   string
}

// UFWManager manages firewall rules via UFW.
type UFWManager struct {
	shell  *executor.Shell
	logger *slog.Logger
}

// NewUFWManager creates a UFWManager.
func NewUFWManager(shell *executor.Shell, logger *slog.Logger) *UFWManager {
	return &UFWManager{shell: shell, logger: logger}
}

// Allow opens a port through the firewall.
func (m *UFWManager) Allow(ctx context.Context, port int, protocol string) error {
	spec := fmt.Sprintf("%d/%s", port, protocol)
	_, err := m.shell.Run(ctx, "ufw", "allow", spec)
	if err != nil {
		return fmt.Errorf("ufw: allow %s: %w", spec, err)
	}
	m.logger.InfoContext(ctx, "firewall: allowed port", "port", port, "proto", protocol)
	return nil
}

// Deny closes a port through the firewall.
func (m *UFWManager) Deny(ctx context.Context, port int, protocol string) error {
	spec := fmt.Sprintf("%d/%s", port, protocol)
	_, err := m.shell.Run(ctx, "ufw", "deny", spec)
	if err != nil {
		return fmt.Errorf("ufw: deny %s: %w", spec, err)
	}
	m.logger.InfoContext(ctx, "firewall: denied port", "port", port, "proto", protocol)
	return nil
}

// List returns the current firewall rules.
func (m *UFWManager) List(ctx context.Context) ([]Rule, error) {
	result, err := m.shell.Run(ctx, "ufw", "status", "numbered")
	if err != nil {
		return nil, fmt.Errorf("ufw: list: %w", err)
	}
	return parseUFWStatus(result.Stdout), nil
}

// parseUFWStatus parses the output of `ufw status numbered` into Rule structs.
func parseUFWStatus(output string) []Rule {
	var rules []Rule
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") {
			continue
		}
		// Example: [ 1] 80/tcp                     ALLOW IN    Anywhere
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		portProto := parts[1]
		action := strings.ToLower(parts[2])

		pp := strings.SplitN(portProto, "/", 2)
		if len(pp) != 2 {
			continue
		}
		port, err := strconv.Atoi(pp[0])
		if err != nil {
			continue
		}
		rules = append(rules, Rule{
			Port:     port,
			Protocol: pp[1],
			Action:   action,
		})
	}
	return rules
}
