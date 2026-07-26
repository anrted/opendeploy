package app

import (
	"testing"

	"github.com/anrted/opendeploy/pkg/contract"
	"go.uber.org/fx"
)

func TestModuleDependencyGraph(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply("", []contract.Module{}),
		Module,
	)
	if err != nil {
		t.Fatalf("invalid Fx dependency graph: %v", err)
	}
}
