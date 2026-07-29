package firewall

import (
	"testing"

	"github.com/anrted/opendeploy/pkg/contract"
)

func TestValidateRule(t *testing.T) {
	valid := []contract.FirewallRuleRequest{
		{Port: "443", Protocol: "tcp", Action: "allow", Direction: "in", IPVersion: "both"},
		{Port: "8000:8100", Protocol: "tcp", Action: "deny", Source: "10.0.0.0/8", IPVersion: "ipv4"},
		{Port: "53", Protocol: "udp", Action: "allow", Source: "2001:db8::/32", IPVersion: "ipv6"},
	}
	for index := range valid {
		if err := validateRule(&valid[index]); err != nil {
			t.Errorf("valid rule rejected: %v", err)
		}
	}
}

func TestValidateRuleRejectsUnsafeValues(t *testing.T) {
	for _, rule := range []contract.FirewallRuleRequest{
		{Port: "0", Action: "allow"},
		{Port: "9000:8000", Action: "allow"},
		{Port: "80", Protocol: "icmp", Action: "allow"},
		{Port: "80", Action: "allow;reset"},
		{Port: "80", Action: "allow", Source: "not-an-ip"},
		{Port: "80", Action: "allow", Comment: "bad\ncomment"},
	} {
		if err := validateRule(&rule); err == nil {
			t.Errorf("unsafe rule accepted: %#v", rule)
		}
	}
}
