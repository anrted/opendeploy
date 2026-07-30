package controlplane

import (
	"context"
	"errors"
	"testing"
	"time"

	agentv1 "github.com/anrted/opendeploy/proto/agent/v1"
)

func TestDispatchCorrelatesResponse(t *testing.T) {
	manager := NewManager()
	session, err := manager.register(&agentv1.AgentHello{
		ServerId: "server-1", ProtocolVersion: ProtocolVersion,
		Capabilities: []string{"dashboard"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.unregister(session)

	go func() {
		message := <-session.send
		command := message.GetCommand()
		session.resolve(&agentv1.CommandResult{Id: command.GetId(), Success: true, Payload: []byte("ok")})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := manager.Dispatch(ctx, "server-1", "system.stats", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != "ok" {
		t.Fatalf("unexpected result %q", result)
	}
}

func TestDispatchRejectsMissingCapabilityBeforeSending(t *testing.T) {
	manager := NewManager()
	session, err := manager.register(&agentv1.AgentHello{
		ServerId: "server-1", ProtocolVersion: ProtocolVersion,
		Capabilities: []string{"dashboard"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.unregister(session)

	_, err = manager.Dispatch(context.Background(), "server-1", "service.status", nil)
	if !errors.Is(err, ErrCapabilityAbsent) {
		t.Fatalf("error = %v, want ErrCapabilityAbsent", err)
	}
	if len(session.send) != 0 {
		t.Fatal("unsupported command was sent to Agent")
	}
}

func TestDispatchRejectsUnregisteredCommand(t *testing.T) {
	manager := NewManager()
	session, err := manager.register(&agentv1.AgentHello{
		ServerId: "server-1", ProtocolVersion: ProtocolVersion,
		Capabilities: []string{"dashboard"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.unregister(session)

	_, err = manager.Dispatch(context.Background(), "server-1", "command.execute", nil)
	if !errors.Is(err, ErrCapabilityAbsent) {
		t.Fatalf("error = %v, want ErrCapabilityAbsent", err)
	}
}

func TestReconnectReplacesOldSession(t *testing.T) {
	manager := NewManager()
	first, _ := manager.register(&agentv1.AgentHello{ServerId: "server-1", ProtocolVersion: ProtocolVersion})
	second, _ := manager.register(&agentv1.AgentHello{ServerId: "server-1", ProtocolVersion: ProtocolVersion})
	select {
	case <-first.done:
	default:
		t.Fatal("old session was not closed")
	}
	if !manager.IsOnline(second.serverID) {
		t.Fatal("replacement session is not online")
	}
}
