package nginx

import (
	"context"
	"fmt"

	"github.com/anrted/opendeploy/pkg/contract"
)

func (m *Module) Actions() []contract.ActionDef {
	return []contract.ActionDef{
		{ID: "start", Title: "Start", Icon: "play", Color: "success", RequiresConfirmation: false},
		{ID: "stop", Title: "Stop", Icon: "square", Color: "secondary", RequiresConfirmation: true},
		{ID: "reload", Title: "Reload Configuration", Icon: "refresh", Color: "primary", RequiresConfirmation: false},
		{ID: "restart", Title: "Restart", Icon: "rotate-cw", Color: "primary", RequiresConfirmation: true},
		{ID: "test_config", Title: "Test Configuration", Icon: "check-circle", Color: "secondary", RequiresConfirmation: false},
	}
}

func (m *Module) ExecuteAction(ctx context.Context, actionID string) error {
	m.log.InfoContext(ctx, "executing nginx action", "action", actionID)
	switch actionID {
	case "start":
		return m.deps.Agent.ServiceStart(ctx, "nginx")
	case "stop":
		return m.deps.Agent.ServiceStop(ctx, "nginx")
	case "reload":
		exitCode, stdout, stderr, err := m.deps.Agent.CommandExecute(ctx, "nginx", "-t")
		if err != nil || exitCode != 0 {
			return fmt.Errorf("configuration test failed, reload aborted:\n%s\n%s", stdout, stderr)
		}
		return m.deps.Agent.ServiceReload(ctx, "nginx")
	case "restart":
		return m.deps.Agent.ServiceRestart(ctx, "nginx")
	case "test_config":
		exitCode, stdout, stderr, err := m.deps.Agent.CommandExecute(ctx, "nginx", "-t")
		if err != nil || exitCode != 0 {
			return fmt.Errorf("nginx -t failed: %v\n%s", err, stderr)
		}
		m.log.Info("Config test passed", "output", stdout)
		return nil
	default:
		return fmt.Errorf("unknown action: %s", actionID)
	}
}

func (m *Module) ActionAvailability(ctx context.Context) map[string]bool {
	status, err := m.deps.Agent.ServiceStatus(ctx, "nginx")
	if err != nil || status == nil {
		return map[string]bool{"start": true, "stop": false, "reload": false, "restart": false, "test_config": true}
	}
	return map[string]bool{
		"start":       !status.Active,
		"stop":        status.Active,
		"reload":      status.Active,
		"restart":     status.Active,
		"test_config": true,
	}
}
