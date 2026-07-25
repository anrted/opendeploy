package dashboard

import (
	"testing"
	"time"
)

func TestWebSocketTicketIsSingleUseAndExpires(t *testing.T) {
	store := newTicketStore()
	now := time.Now()
	ticket, _, err := store.Issue("user-1", now)
	if err != nil {
		t.Fatalf("Issue returned %v", err)
	}
	if !store.Consume(ticket, now.Add(time.Second)) {
		t.Fatal("fresh ticket was rejected")
	}
	if store.Consume(ticket, now.Add(2*time.Second)) {
		t.Fatal("ticket was accepted twice")
	}

	expired, _, err := store.Issue("user-1", now)
	if err != nil {
		t.Fatalf("Issue returned %v", err)
	}
	if store.Consume(expired, now.Add(websocketTicketTTL+time.Second)) {
		t.Fatal("expired ticket was accepted")
	}
}
