package nginx

import (
	"context"
	"fmt"
	"strings"

	"github.com/anrted/opendeploy/pkg/contract"
)

func (m *Module) HealthCheck(ctx context.Context) (*contract.HealthReport, error) {
	serviceStatus, err := m.deps.Agent.ServiceStatus(ctx, "nginx")
	if err != nil {
		return &contract.HealthReport{Status: contract.HealthError, Message: fmt.Sprintf("cannot query nginx service: %v", err)}, nil
	}
	checks := []contract.HealthCheck{{
		Name: "service_running", Status: boolHealth(serviceStatus.Active),
		Message: formatServiceMsg(serviceStatus.Active),
	}}
	installed, version, packageErr := m.deps.Agent.PackageInstalled(ctx, "nginx")
	switch {
	case packageErr != nil:
		checks = append(checks, contract.HealthCheck{Name: "version", Status: contract.HealthWarning, Message: "Cannot read installed Nginx version"})
	case !installed:
		checks = append(checks, contract.HealthCheck{Name: "version", Status: contract.HealthError, Message: "Nginx package is not installed"})
	default:
		checks = append(checks, contract.HealthCheck{Name: "version", Status: contract.HealthOK, Message: "Installed version: " + version})
	}
	if _, configErr := m.deps.Agent.FileRead(ctx, nginxMainConfigPath); configErr != nil {
		checks = append(checks, contract.HealthCheck{Name: "configuration_present", Status: contract.HealthError, Message: "nginx.conf is not readable"})
	} else {
		checks = append(checks, contract.HealthCheck{Name: "configuration_present", Status: contract.HealthOK, Message: "nginx.conf is readable"})
	}
	configExitCode, _, standardError, configError := m.deps.Agent.CommandExecute(ctx, "nginx", "-t")
	if configError == nil && configExitCode == 0 {
		checks = append(checks, contract.HealthCheck{Name: "config_valid", Status: contract.HealthOK, Message: "Configuration is valid"})
	} else {
		checks = append(checks, contract.HealthCheck{Name: "config_valid", Status: contract.HealthError, Message: "Configuration test failed:\n" + standardError})
	}
	_, portOutput, _, _ := m.deps.Agent.CommandExecute(ctx, "ss", "-tuln")
	if strings.Contains(portOutput, ":80 ") || strings.Contains(portOutput, ":443 ") {
		checks = append(checks, contract.HealthCheck{Name: "port_open", Status: contract.HealthOK, Message: "Listening on port 80/443"})
	} else {
		checks = append(checks, contract.HealthCheck{Name: "port_open", Status: contract.HealthWarning, Message: "Not listening on standard HTTP(S) ports"})
	}
	processStatus := contract.HealthError
	processMessage := "No Nginx processes found"
	if processes, processErr := m.deps.Agent.ProcessList(ctx); processErr != nil {
		processStatus = contract.HealthWarning
		processMessage = "Cannot query process list"
	} else {
		for _, process := range processes {
			if process.Name == "nginx" {
				processStatus = contract.HealthOK
				processMessage = "Nginx process is running"
				break
			}
		}
	}
	checks = append(checks, contract.HealthCheck{Name: "process", Status: processStatus, Message: processMessage})

	if certificates, certificateErr := m.certificateRows(ctx); certificateErr != nil {
		checks = append(checks, contract.HealthCheck{Name: "ssl", Status: contract.HealthWarning, Message: "Cannot inspect configured certificates"})
	} else {
		sslStatus := contract.HealthOK
		sslMessage := fmt.Sprintf("%d configured certificate(s) are valid", len(certificates))
		for _, certificate := range certificates {
			status, _ := certificate["status"].(string)
			if status == "expired" || status == "invalid" {
				sslStatus = contract.HealthError
				sslMessage = "One or more configured certificates are invalid or expired"
				break
			}
			if status == "expiring" {
				sslStatus = contract.HealthWarning
				sslMessage = "One or more configured certificates expire within 30 days"
			}
		}
		checks = append(checks, contract.HealthCheck{Name: "ssl", Status: sslStatus, Message: sslMessage})
	}
	return &contract.HealthReport{Status: aggregateHealth(checks), Checks: checks}, nil
}

func aggregateHealth(checks []contract.HealthCheck) contract.HealthStatus {
	overall := contract.HealthOK
	for _, check := range checks {
		if check.Status == contract.HealthError {
			return contract.HealthError
		}
		if check.Status == contract.HealthWarning {
			overall = contract.HealthWarning
		}
	}
	return overall
}

func boolHealth(ok bool) contract.HealthStatus {
	if ok {
		return contract.HealthOK
	}
	return contract.HealthError
}

func formatServiceMsg(active bool) string {
	if active {
		return "nginx service is running"
	}
	return "nginx service is not running"
}
