package firewall

import (
	"context"

	agentv1 "github.com/anrted/opendeploy/proto/agent/v1"
)

// Provider is the generic interface for firewall management.
type Provider interface {
	Status(ctx context.Context) (*agentv1.FirewallStatusResponse, error)
	List(ctx context.Context) ([]*agentv1.FirewallEntry, error)
	AddRule(ctx context.Context, req *agentv1.FirewallRuleRequest) error
	DeleteRule(ctx context.Context, id string) error
	Toggle(ctx context.Context, enable bool) error
	Reset(ctx context.Context) error
}
