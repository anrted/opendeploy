package dashboard

import (
	"testing"
	"time"
)

func TestWebSocketTicketIsSingleUseAndExpires(t *testing.T) {
	store := newTicketStore()
	now := time.Now()
	ticket, _, err := store.Issue("user-1", "server-1", now)
	if err != nil {
		t.Fatalf("Issue returned %v", err)
	}
	serverID, valid := store.Consume(ticket, now.Add(time.Second))
	if !valid || serverID != "server-1" {
		t.Fatal("fresh ticket was rejected")
	}
	if _, valid := store.Consume(ticket, now.Add(2*time.Second)); valid {
		t.Fatal("ticket was accepted twice")
	}

	expired, _, err := store.Issue("user-1", "server-1", now)
	if err != nil {
		t.Fatalf("Issue returned %v", err)
	}
	if _, valid := store.Consume(expired, now.Add(websocketTicketTTL+time.Second)); valid {
		t.Fatal("expired ticket was accepted")
	}
}
