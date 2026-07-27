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
	_, _, standardError, configError := m.deps.Agent.CommandExecute(ctx, "nginx", "-t")
	if configError == nil {
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
