// Package events provides an in-process publish/subscribe event bus for OpenDeploy.
//
// The EventBus decouples producers (services, modules) from consumers (WebSocket
// hub, audit logger, background tasks) without introducing an external broker.
// All handlers are invoked synchronously in the goroutine of the publisher.
// For fan-out to multiple async consumers use buffered channels.
package events

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Event is the interface that all domain events must satisfy.
type Event interface {
	// Type returns a dot-separated identifier, e.g. "module.installed".
	Type() string
	// Payload returns the event-specific data.
	Payload() any
	// OccurredAt returns the time the event was created.
	OccurredAt() time.Time
}

// Handler is a function that processes an event.
type Handler func(ctx context.Context, event Event) error

// UnsubscribeFn removes a subscription when called.
type UnsubscribeFn func()

// Bus is the interface for the in-process event bus.
type Bus interface {
	// Publish dispatches an event to all matching subscribers.
	Publish(ctx context.Context, event Event) error
	// Subscribe registers a handler for events of the given type.
	// Use "*" to receive all events.
	Subscribe(eventType string, handler Handler) UnsubscribeFn
}

// subscription holds a single registered handler.
type subscription struct {
	id      int64
	handler Handler
}

// MemoryBus is a thread-safe in-memory Bus implementation.
type MemoryBus struct {
	mu   sync.RWMutex
	subs map[string][]subscription
	seq  int64
}

// NewMemoryBus creates a ready-to-use MemoryBus.
func NewMemoryBus() *MemoryBus {
	return &MemoryBus{subs: make(map[string][]subscription)}
}

// Subscribe registers handler for eventType. Returns a function that removes
// the subscription. Safe to call concurrently.
func (b *MemoryBus) Subscribe(eventType string, handler Handler) UnsubscribeFn {
	b.mu.Lock()
	b.seq++
	id := b.seq
	b.subs[eventType] = append(b.subs[eventType], subscription{id: id, handler: handler})
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.subs[eventType]
		for i, s := range subs {
			if s.id == id {
				b.subs[eventType] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}
}

// Publish calls all handlers registered for event.Type() and for the wildcard
// "*". Errors from handlers are accumulated and returned as a combined error.
func (b *MemoryBus) Publish(ctx context.Context, event Event) error {
	b.mu.RLock()
	specific := make([]subscription, len(b.subs[event.Type()]))
	copy(specific, b.subs[event.Type()])
	wildcard := make([]subscription, len(b.subs["*"]))
	copy(wildcard, b.subs["*"])
	b.mu.RUnlock()

	var errs []error
	for _, s := range append(specific, wildcard...) {
		if err := s.handler(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("event bus: %d handler(s) failed: %v", len(errs), errs)
	}
	return nil
}

// — BaseEvent provides a reusable implementation of the Event interface —

// BaseEvent is an embeddable struct for creating domain events.
type BaseEvent struct {
	eventType  string
	payload    any
	occurredAt time.Time
}

// NewBaseEvent creates a BaseEvent with the current time.
func NewBaseEvent(eventType string, payload any) BaseEvent {
	return BaseEvent{eventType: eventType, payload: payload, occurredAt: time.Now().UTC()}
}

func (e BaseEvent) Type() string          { return e.eventType }
func (e BaseEvent) Payload() any          { return e.payload }
func (e BaseEvent) OccurredAt() time.Time { return e.occurredAt }
