package dashboard

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

const websocketTicketTTL = 30 * time.Second

type websocketTicket struct {
	userID    string
	serverID  string
	expiresAt time.Time
}

type ticketStore struct {
	mu      sync.Mutex
	tickets map[string]websocketTicket
}

func newTicketStore() *ticketStore {
	return &ticketStore{tickets: make(map[string]websocketTicket)}
}

func (s *ticketStore) Issue(userID, serverID string, now time.Time) (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expiresAt := now.Add(websocketTicketTTL)
	s.mu.Lock()
	for key, ticket := range s.tickets {
		if !ticket.expiresAt.After(now) {
			delete(s.tickets, key)
		}
	}
	s.tickets[token] = websocketTicket{userID: userID, serverID: serverID, expiresAt: expiresAt}
	s.mu.Unlock()
	return token, expiresAt, nil
}

func (s *ticketStore) Consume(token string, now time.Time) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ticket, ok := s.tickets[token]
	delete(s.tickets, token)
	return ticket.serverID, ok && ticket.expiresAt.After(now)
}
