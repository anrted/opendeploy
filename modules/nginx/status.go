package nginx

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	} else if serviceStatus != nil && serviceStatus.Active {
		runtimeState = contract.ServiceRunning
	}
	properties := m.runtimeProperties(ctx, serviceStatus)
	return &contract.RuntimeStatus{
		PackageStatus: packageStatus, ServiceStatus: runtimeState,
		SoftwareVersion: version, Properties: properties,
	}, nil
}

func (m *Module) runtimeProperties(ctx context.Context, serviceStatus *contract.ServiceStatus) []contract.Property {
	systemdStatus := "unavailable"
	if serviceStatus != nil {
		systemdStatus = serviceStatus.SubState
		if systemdStatus == "" {
			if serviceStatus.Active {
				systemdStatus = "running"
			} else {
				systemdStatus = "stopped"
			}
		}
	}
	properties := []contract.Property{
		{Name: "Service", Value: "nginx", Group: "Overview"},
		{Name: "Systemd Status", Value: systemdStatus, Group: "Overview"},
		{Name: "Configuration", Value: nginxMainConfigPath, Group: "Configuration"},
		{Name: "Last Health Check", Value: time.Now().UTC().Format(time.RFC3339), Group: "Health"},
	}
	_, stdout, _, _ := m.deps.Agent.CommandExecute(ctx, "systemctl", "show", "nginx", "--property=MainPID")
	if pid := strings.TrimSpace(strings.TrimPrefix(stdout, "MainPID=")); pid != "" && pid != "0" {
		properties = append(properties, contract.Property{Name: "Main PID", Value: pid, Group: "Overview"})
	}
	if processes, err := m.deps.Agent.ProcessList(ctx); err == nil {
		var cpu, memory float64
		workers := 0
		for _, process := range processes {
			if process.Name != "nginx" {
				continue
			}
			workers++
			cpu += process.CpuPercent
			memory += process.MemPercent
		}
		properties = append(properties,
			contract.Property{Name: "Nginx Processes", Value: fmt.Sprint(workers), Group: "Performance"},
			contract.Property{Name: "CPU Usage", Value: fmt.Sprintf("%.2f%%", cpu), Group: "Performance"},
			contract.Property{Name: "Memory Usage", Value: fmt.Sprintf("%.2f%%", memory), Group: "Performance"},
		)
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
	if sites, err := m.deps.Agent.DirList(ctx, "/etc/nginx/sites-available"); err == nil {
		virtualHosts, certificates := 0, 0
		for _, site := range sites {
			if site.IsDir || !strings.HasPrefix(site.Name, "opendeploy-") || !strings.HasSuffix(site.Name, ".conf") {
				continue
			}
			virtualHosts++
			if content, readErr := m.deps.Agent.FileRead(ctx, site.Path); readErr == nil && nginxDirective(string(content), "ssl_certificate") != "" {
				certificates++
			}
		}
		properties = append(properties,
			contract.Property{Name: "Virtual Hosts", Value: fmt.Sprint(virtualHosts), Group: "Configuration"},
			contract.Property{Name: "SSL Certificates", Value: fmt.Sprint(certificates), Group: "Configuration"},
		)
	}
	values := m.currentSettings()
	properties = append(properties,
		contract.Property{Name: "Worker Processes", Value: fmt.Sprint(values["worker_processes"]), Group: "Performance"},
		contract.Property{Name: "Worker Connections", Value: fmt.Sprint(values["worker_connections"]), Group: "Performance"},
	)
	_, _, buildOutput, _ := m.deps.Agent.CommandExecute(ctx, "nginx", "-V")
	if firstLine := strings.Split(strings.TrimSpace(buildOutput), "\n"); len(firstLine) > 0 && firstLine[0] != "" {
		properties = append(properties, contract.Property{Name: "Build", Value: firstLine[0], Group: "Overview"})
	}
	return properties
}
