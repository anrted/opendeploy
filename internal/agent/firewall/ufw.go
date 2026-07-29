package firewall

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/anrted/opendeploy/internal/agent/executor"
	agentv1 "github.com/anrted/opendeploy/proto/agent/v1"
)

// UFWManager implements Provider using UFW.
type UFWManager struct {
	shell  *executor.Shell
	logger *slog.Logger
}

// NewUFWManager creates a UFWManager.
func NewUFWManager(shell *executor.Shell, logger *slog.Logger) *UFWManager {
	return &UFWManager{shell: shell, logger: logger}
}

// Status returns the current UFW status and default policies.
func (m *UFWManager) Status(ctx context.Context) (*agentv1.FirewallStatusResponse, error) {
	res, err := m.shell.Run(ctx, "ufw", "status", "verbose")
	if err != nil {
		return nil, fmt.Errorf("ufw: status: %w", err)
	}

	status := &agentv1.FirewallStatusResponse{
		ProfileName: "ufw",
	}

	lines := strings.Split(res.Stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Status:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
			status.Active = val == "active"
		} else if strings.HasPrefix(line, "Logging:") {
			parts := strings.Split(line, " ")
			if len(parts) > 1 {
				status.Logging = parts[1]
			}
		} else if strings.HasPrefix(line, "Default:") {
			if strings.Contains(line, "(incoming)") {
				status.DefaultIncoming = extractPolicy(line, "incoming")
			}
			if strings.Contains(line, "(outgoing)") {
				status.DefaultOutgoing = extractPolicy(line, "outgoing")
			}
			if strings.Contains(line, "(routed)") {
				status.DefaultRouted = extractPolicy(line, "routed")
			}
		}
	}

	rules, err := m.List(ctx)
	if err == nil {
		status.RuleCount = int32(len(rules))
	}

	status.Ipv6Enabled = true

	return status, nil
}

func extractPolicy(line, direction string) string {
	parts := strings.Split(line, ",")
	for _, p := range parts {
		if strings.Contains(p, "("+direction+")") {
			policy := strings.TrimSpace(strings.Split(p, "(")[0])
			if strings.HasPrefix(policy, "Default: ") {
				policy = strings.TrimPrefix(policy, "Default: ")
			}
			return policy
		}
	}
	return ""
}

// List returns the current firewall rules.
func (m *UFWManager) List(ctx context.Context) ([]*agentv1.FirewallEntry, error) {
	result, err := m.shell.Run(ctx, "ufw", "status", "numbered")
	if err != nil {
		return nil, fmt.Errorf("ufw: list: %w", err)
	}
	return m.parseUFWStatus(result.Stdout), nil
}

// AddRule adds a firewall rule.
func (m *UFWManager) AddRule(ctx context.Context, req *agentv1.FirewallRuleRequest) error {
	if req.GetId() != "" {
		return m.updateRule(ctx, req)
	}
	return m.addRule(ctx, req)
}

func (m *UFWManager) addRule(ctx context.Context, req *agentv1.FirewallRuleRequest) error {
	args := []string{}

	switch req.Action {
	case agentv1.FirewallAction_FIREWALL_ACTION_ALLOW:
		args = append([]string{"allow"}, args...)
	case agentv1.FirewallAction_FIREWALL_ACTION_DENY:
		args = append([]string{"deny"}, args...)
	case agentv1.FirewallAction_FIREWALL_ACTION_REJECT:
		args = append([]string{"reject"}, args...)
	default:
		return fmt.Errorf("unsupported action")
	}

	if req.Direction == agentv1.FirewallDirection_FIREWALL_DIRECTION_IN {
		args = append(args, "in")
	} else if req.Direction == agentv1.FirewallDirection_FIREWALL_DIRECTION_OUT {
		args = append(args, "out")
	}

	if req.Source == "" && req.Destination == "" && req.IpVersion == agentv1.FirewallIPVersion_FIREWALL_IP_VERSION_BOTH {
		spec := req.Port
		if req.Protocol != "" && req.Protocol != "any" {
			spec = fmt.Sprintf("%s/%s", req.Port, req.Protocol)
		}
		args = append(args, spec)
	} else {
		if req.Protocol != "" && req.Protocol != "any" {
			args = append(args, "proto", req.Protocol)
		}

		src := req.Source
		if src == "" {
			src = "any"
		}
		args = append(args, "from", src)

		dst := req.Destination
		if dst == "" {
			dst = "any"
		}
		args = append(args, "to", dst)

		if req.Port != "" {
			args = append(args, "port", req.Port)
		}
	}

	if req.Comment != "" {
		args = append(args, "comment", req.Comment)
	}

	_, err := m.shell.Run(ctx, "ufw", args...)
	if err != nil {
		return fmt.Errorf("ufw: rule %v: %w", args, err)
	}
	m.logger.InfoContext(ctx, "firewall: added rule", "args", args)
	return nil
}

var ruleIDPattern = regexp.MustCompile(`^[1-9][0-9]{0,5}$`)

func (m *UFWManager) updateRule(ctx context.Context, req *agentv1.FirewallRuleRequest) error {
	id := req.GetId()
	if !ruleIDPattern.MatchString(id) {
		return fmt.Errorf("ufw: invalid rule id")
	}
	rules, err := m.List(ctx)
	if err != nil {
		return err
	}
	var previous *agentv1.FirewallEntry
	for _, rule := range rules {
		if rule.GetId() == id {
			previous = rule
			break
		}
	}
	if previous == nil {
		return fmt.Errorf("ufw: rule %s does not exist", id)
	}
	replacement := *req
	replacement.Id = ""
	if err := m.addRule(ctx, &replacement); err != nil {
		return fmt.Errorf("ufw: validate replacement for rule %s: %w", id, err)
	}
	if err := m.DeleteRule(ctx, id); err != nil {
		rollbackErr := m.deleteMatchingReplacement(ctx, id, &replacement)
		return fmt.Errorf("ufw: replace rule %s: %w", id, errors.Join(err, rollbackErr))
	}
	m.logger.InfoContext(ctx, "firewall: updated rule", "id", id)
	return nil
}

func (m *UFWManager) deleteMatchingReplacement(ctx context.Context, originalID string, replacement *agentv1.FirewallRuleRequest) error {
	rules, err := m.List(ctx)
	if err != nil {
		return err
	}
	for index := len(rules) - 1; index >= 0; index-- {
		rule := rules[index]
		if rule.Id == originalID || rule.Port != replacement.Port || rule.Protocol != replacement.Protocol ||
			rule.Action != replacement.Action || rule.Direction != replacement.Direction {
			continue
		}
		return m.DeleteRule(ctx, rule.Id)
	}
	return fmt.Errorf("ufw: replacement rollback rule was not found")
}

// DeleteRule removes a rule from the firewall by ID.
func (m *UFWManager) DeleteRule(ctx context.Context, id string) error {
	if !ruleIDPattern.MatchString(id) {
		return fmt.Errorf("ufw: invalid rule id")
	}
	_, err := m.shell.Run(ctx, "ufw", "--force", "delete", id)
	if err != nil {
		return fmt.Errorf("ufw: delete %s failed: %w", id, err)
	}
	m.logger.InfoContext(ctx, "firewall: deleted rule", "id", id)
	return nil
}

// Toggle enables or disables UFW.
func (m *UFWManager) Toggle(ctx context.Context, enable bool) error {
	action := "disable"
	if enable {
		action = "enable"
	}
	_, err := m.shell.Run(ctx, "ufw", "--force", action)
	if err != nil {
		return fmt.Errorf("ufw: %s failed: %w", action, err)
	}
	status, err := m.Status(ctx)
	if err != nil {
		return fmt.Errorf("ufw: verify %s: %w", action, err)
	}
	if status.Active != enable {
		return fmt.Errorf("ufw: %s did not reach requested state", action)
	}
	return nil
}

// Reset resets UFW.
func (m *UFWManager) Reset(ctx context.Context) error {
	_, err := m.shell.Run(ctx, "ufw", "--force", "reset")
	if err != nil {
		return fmt.Errorf("ufw: reset failed: %w", err)
	}
	return nil
}

func (m *UFWManager) parseUFWStatus(output string) []*agentv1.FirewallEntry {
	var rules []*agentv1.FirewallEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") {
			continue
		}

		bracketIdx := strings.Index(line, "]")
		if bracketIdx == -1 {
			continue
		}
		idStr := strings.TrimSpace(line[1:bracketIdx])
		rest := strings.TrimSpace(line[bracketIdx+1:])

		parts := strings.Fields(rest)
		if len(parts) < 3 {
			continue
		}

		entry := &agentv1.FirewallEntry{
			Id: idStr,
		}

		entry.Destination = parts[0]

		actionStr := ""
		for _, p := range parts[1:] {
			p = strings.ToUpper(p)
			if p == "ALLOW" || p == "DENY" || p == "REJECT" {
				actionStr = p
				break
			}
		}

		if actionStr == "ALLOW" {
			entry.Action = agentv1.FirewallAction_FIREWALL_ACTION_ALLOW
		} else if actionStr == "DENY" {
			entry.Action = agentv1.FirewallAction_FIREWALL_ACTION_DENY
		} else if actionStr == "REJECT" {
			entry.Action = agentv1.FirewallAction_FIREWALL_ACTION_REJECT
		}

		if strings.Contains(rest, " IN ") {
			entry.Direction = agentv1.FirewallDirection_FIREWALL_DIRECTION_IN
		} else if strings.Contains(rest, " OUT ") {
			entry.Direction = agentv1.FirewallDirection_FIREWALL_DIRECTION_OUT
		} else {
			entry.Direction = agentv1.FirewallDirection_FIREWALL_DIRECTION_IN
		}

		if strings.Contains(entry.Destination, "/") {
			pp := strings.SplitN(entry.Destination, "/", 2)
			entry.Port = pp[0]
			entry.Protocol = pp[1]
			entry.Destination = "Anywhere"
		}

		if strings.Contains(rest, "(v6)") {
			entry.IpVersion = agentv1.FirewallIPVersion_FIREWALL_IP_VERSION_V6
		} else {
			entry.IpVersion = agentv1.FirewallIPVersion_FIREWALL_IP_VERSION_V4
		}

		rules = append(rules, entry)
	}
	return rules
}
