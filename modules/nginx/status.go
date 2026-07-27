package nginx

import (
	"context"
	"strings"

	"github.com/anrted/opendeploy/pkg/contract"
)

func (m *Module) Status(ctx context.Context) (*contract.RuntimeStatus, error) {
	serviceStatus, err := m.deps.Agent.ServiceStatus(ctx, "nginx")
	installed, version, _ := m.deps.Agent.PackageInstalled(ctx, "nginx")
	packageStatus := contract.PackageNotInstalled
	if installed {
		packageStatus = contract.PackageInstalled
	}
	runtimeState := contract.ServiceStopped
	if err != nil {
		runtimeState = contract.ServiceFailed
	} else if serviceStatus.Active {
		runtimeState = contract.ServiceRunning
	}
	properties := make([]contract.Property, 0)
	if runtimeState == contract.ServiceRunning {
		properties = append(properties, m.runtimeProperties(ctx)...)
	}
	return &contract.RuntimeStatus{
		PackageStatus: packageStatus, ServiceStatus: runtimeState,
		SoftwareVersion: version, Properties: properties,
	}, nil
}

func (m *Module) runtimeProperties(ctx context.Context) []contract.Property {
	properties := make([]contract.Property, 0)
	_, stdout, _, _ := m.deps.Agent.CommandExecute(ctx, "systemctl", "show", "nginx", "--property=MainPID")
	if pid := strings.TrimSpace(strings.TrimPrefix(stdout, "MainPID=")); pid != "" && pid != "0" {
		properties = append(properties, contract.Property{Name: "Main PID", Value: pid, Group: "Overview"})
		_, processOutput, _, _ := m.deps.Agent.CommandExecute(ctx, "ps", "-p", pid, "-o", "%cpu,%mem", "--no-headers")
		fields := strings.Fields(processOutput)
		if len(fields) >= 2 {
			properties = append(properties,
				contract.Property{Name: "CPU Usage", Value: fields[0] + "%", Group: "Performance"},
				contract.Property{Name: "Memory Usage", Value: fields[1] + "%", Group: "Performance"},
			)
		}
	}
	_, uptimeOutput, _, _ := m.deps.Agent.CommandExecute(ctx, "systemctl", "show", "nginx", "--property=ActiveEnterTimestamp")
	if startedAt := strings.TrimSpace(strings.TrimPrefix(uptimeOutput, "ActiveEnterTimestamp=")); startedAt != "" {
		properties = append(properties, contract.Property{Name: "Started At", Value: startedAt, Group: "Overview"})
	}
	_, statusOutput, _, _ := m.deps.Agent.CommandExecute(ctx, "curl", "-s", "--max-time", "1", "http://127.0.0.1/nginx_status")
	if strings.Contains(statusOutput, "Active connections:") {
		for _, line := range strings.Split(statusOutput, "\n") {
			if strings.HasPrefix(line, "Active connections:") {
				properties = append(properties, contract.Property{Name: "Active Connections", Value: strings.TrimSpace(strings.TrimPrefix(line, "Active connections:")), Group: "Performance"})
			} else if strings.Contains(line, "Reading:") {
				properties = append(properties, contract.Property{Name: "Connection Stats", Value: strings.TrimSpace(line), Group: "Performance"})
			}
		}
	}
	return properties
}
