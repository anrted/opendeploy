// Package websocket provides a WebSocket hub for broadcasting real-time events
// to connected browser clients.
//
// The hub manages client connections, room-based subscriptions, and graceful
// cleanup on disconnect. Messages are serialized as JSON.
package websocket

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// writeWait is the time allowed to write a message to the client.
	writeWait = 10 * time.Second
	// pongWait is the time allowed to read the next pong from the client.
	pongWait = 60 * time.Second
	// pingPeriod is how often the server sends pings (must be < pongWait).
	pingPeriod = (pongWait * 9) / 10
	// maxMessageSize is the maximum allowed message size from the client.
	maxMessageSize = 512
)

// Message is a JSON payload sent over the WebSocket.
type Message struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

// Client represents a single connected WebSocket session.
// It decouples the hub from the concrete websocket library so the hub
// can be tested without a real TCP connection.
type Client interface {
	// ID returns the unique identifier for this connection.
	ID() string
	// Send queues a message for delivery. Non-blocking.
	Send(msg Message)
	// Close terminates the connection.
	Close()
	// Rooms returns the set of room names this client is subscribed to.
	Rooms() []string
}

// Hub manages all active WebSocket clients and message broadcasting.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]Client
	logger  *slog.Logger
}

// NewHub creates a Hub ready to accept clients.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients: make(map[string]Client),
		logger:  logger,
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(c Client) {
	h.mu.Lock()
	h.clients[c.ID()] = c
	h.mu.Unlock()
	h.logger.Info("websocket: client connected", "client_id", c.ID())
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(c Client) {
	h.mu.Lock()
	delete(h.clients, c.ID())
	h.mu.Unlock()
	h.logger.Info("websocket: client disconnected", "client_id", c.ID())
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.clients {
		c.Send(msg)
	}
}

// BroadcastToRoom sends a message to all clients subscribed to the given room.
func (h *Hub) BroadcastToRoom(room string, msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.clients {
		for _, r := range c.Rooms() {
			if r == room {
				c.Send(msg)
				break
			}
		}
	}
}

// ClientCount returns the number of currently connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// — JSON helper —

// MarshalMessage serialises a Message to JSON bytes.
func MarshalMessage(msg Message) ([]byte, error) {
	return json.Marshal(msg)
}

// GenerateClientID returns a new unique client identifier.
func GenerateClientID() string {
	return uuid.New().String()
}

// UpgradeFunc is a function that upgrades an HTTP connection to WebSocket.
// Injected as a dependency so the hub stays library-agnostic.
type UpgradeFunc func(w http.ResponseWriter, r *http.Request) (Client, error)
