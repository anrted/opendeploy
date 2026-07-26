package health

import (
	"context"
	"time"
)

type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

type CheckResult struct {
	Name        string
	Status      Status
	Message     string
	Duration    time.Duration
	LastChecked time.Time
}

type Report struct {
	Overall Status
	Checks  []CheckResult
}

// Checker must be implemented by modules to report their health.
type Checker interface {
	HealthCheck(ctx context.Context) (*Report, error)
}
