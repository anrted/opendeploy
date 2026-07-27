package events

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMemoryBusDispatchesSpecificAndWildcardSubscribers(t *testing.T) {
	bus := NewMemoryBus()
	calls := 0
	bus.Subscribe("site.created", func(context.Context, Event) error { calls++; return nil })
	bus.Subscribe("*", func(context.Context, Event) error { calls++; return nil })
	if err := bus.Publish(context.Background(), NewBaseEvent("site.created", "payload")); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestMemoryBusIsolatesPanickingHandlerAndContinuesFanout(t *testing.T) {
	bus := NewMemoryBus()
	called := false
	bus.Subscribe("site.created", func(context.Context, Event) error { panic("boom") })
	bus.Subscribe("site.created", func(context.Context, Event) error { called = true; return errors.New("failed") })
	err := bus.Publish(context.Background(), NewBaseEvent("site.created", nil))
	if !called {
		t.Fatal("fanout stopped after handler panic")
	}
	if err == nil || !strings.Contains(err.Error(), "panic") || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("unexpected aggregated error: %v", err)
	}
}

func TestBaseEventCarriesStableIdentityAndCorrelation(t *testing.T) {
	event := NewCorrelatedEvent("site.created", nil, "request-1")
	if event.ID() == "" || event.CorrelationID() != "request-1" {
		t.Fatalf("invalid event metadata: id=%q correlation=%q", event.ID(), event.CorrelationID())
	}
}
