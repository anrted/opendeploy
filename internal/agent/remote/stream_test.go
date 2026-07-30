package remote

import (
	"context"
	"testing"
	"time"

	agentv1 "github.com/anrted/opendeploy/proto/agent/v1"
)

func TestSendMessageStopsWhenConnectionCloses(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	outbound := make(chan *agentv1.AgentMessage)
	cancel()

	done := make(chan bool, 1)
	go func() {
		done <- sendMessage(ctx, outbound, &agentv1.AgentMessage{})
	}()
	select {
	case sent := <-done:
		if sent {
			t.Fatal("message reported as sent after cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("send did not stop after cancellation")
	}
}
