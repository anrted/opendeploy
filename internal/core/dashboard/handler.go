package dashboard

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	gorilla "github.com/gorilla/websocket"

	"github.com/anrted/opendeploy/internal/core/auth"
	"github.com/anrted/opendeploy/internal/platform/apperrors"
	wsHub "github.com/anrted/opendeploy/internal/platform/websocket"
)

// Handler exposes dashboard endpoints.
type Handler struct {
	service  *Service
	hub      *wsHub.Hub
	upgrader gorilla.Upgrader
	logger   *slog.Logger
	tickets  *ticketStore
}

// NewHandler constructs a dashboard Handler.
func NewHandler(service *Service, hub *wsHub.Hub, logger *slog.Logger) *Handler {
	return &Handler{
		service: service,
		hub:     hub,
		logger:  logger,
		tickets: newTicketStore(),
		upgrader: gorilla.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 4096,
		},
	}
}

// IssueWebSocketTicket creates a short-lived, single-use credential for a
// browser WebSocket upgrade. Access tokens are never placed in URLs.
func (h *Handler) IssueWebSocketTicket(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		writeError(w, apperrors.Unauthorized("not authenticated"))
		return
	}
	ticket, expiresAt, err := h.tickets.Issue(principal.ID, time.Now())
	if err != nil {
		writeError(w, apperrors.Internal("issue websocket ticket", err))
		return
	}
	respond(w, http.StatusCreated, map[string]any{
		"ticket":     ticket,
		"expires_at": expiresAt.UTC(),
	})
}

// Overview handles GET /api/v1/dashboard
func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	overview, err := h.service.Overview(r.Context())
	if err != nil {
		writeError(w, apperrors.Internal("build dashboard overview", err))
		return
	}
	respond(w, http.StatusOK, overview)
}

// Snapshots handles GET /api/v1/dashboard/snapshots
func (h *Handler) Snapshots(w http.ResponseWriter, r *http.Request) {
	snaps, err := h.service.listSnapshots(r.Context(), 60)
	if err != nil {
		writeError(w, apperrors.Internal("list snapshots", err))
		return
	}
	respond(w, http.StatusOK, snaps)
}

// WebSocket handles GET /api/v1/dashboard/ws
// Upgrades the connection and subscribes the client to real-time stats.
func (h *Handler) WebSocket(w http.ResponseWriter, r *http.Request) {
	if !h.tickets.Consume(r.URL.Query().Get("ticket"), time.Now()) {
		writeError(w, apperrors.Unauthorized("invalid or expired websocket ticket"))
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Warn("dashboard ws: upgrade failed", "error", err)
		return
	}

	// Create a hub-compatible client and register it.
	client := newGorillaClient(conn, "dashboard", h.hub, h.logger)
	h.hub.Register(client)

	// Run the client pump in the background; it unregisters on close.
	go client.writePump()
	go client.readPump()
}

// ─── gorilla client ────────────────────────────────────────────────────────

// gorillaClient adapts a *gorilla/websocket.Conn to the wsHub.Client interface.
type gorillaClient struct {
	id     string
	conn   *gorilla.Conn
	room   string
	hub    *wsHub.Hub
	send   chan wsHub.Message
	logger *slog.Logger
}

func newGorillaClient(conn *gorilla.Conn, room string, hub *wsHub.Hub, logger *slog.Logger) *gorillaClient {
	return &gorillaClient{
		id:     wsHub.GenerateClientID(),
		conn:   conn,
		room:   room,
		hub:    hub,
		send:   make(chan wsHub.Message, 32),
		logger: logger,
	}
}

func (c *gorillaClient) ID() string      { return c.id }
func (c *gorillaClient) Rooms() []string { return []string{c.room} }
func (c *gorillaClient) Send(msg wsHub.Message) {
	select {
	case c.send <- msg:
	default:
		// Drop if send buffer full (slow client).
	}
}
func (c *gorillaClient) Close() {
	c.conn.Close()
}

// writePump forwards messages from the send channel to the WebSocket connection.
func (c *gorillaClient) writePump() {
	ticker := time.NewTicker(54 * time.Second) // keep-alive ping
	defer func() {
		ticker.Stop()
		c.hub.Unregister(c)
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(gorilla.CloseMessage, []byte{})
				return
			}
			b, err := json.Marshal(msg)
			if err != nil {
				return
			}
			if err := c.conn.WriteMessage(gorilla.TextMessage, b); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(gorilla.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump discards incoming messages (we only push from server) and detects disconnects.
func (c *gorillaClient) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// ─── helpers ───────────────────────────────────────────────────────────────

func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, err error) {
	ae := apperrors.AsAppError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ae.HTTPStatus)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    ae.Code,
			"message": ae.Message,
		},
	})
}
