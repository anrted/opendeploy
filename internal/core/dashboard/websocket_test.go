package dashboard_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gorilla "github.com/gorilla/websocket"

	"github.com/anrted/opendeploy/internal/core/auth"
	"github.com/anrted/opendeploy/internal/core/dashboard"
	coreMiddleware "github.com/anrted/opendeploy/internal/core/server/middleware"
	wsHub "github.com/anrted/opendeploy/internal/platform/websocket"
)

func TestWebSocketTicketUpgradeThroughLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := wsHub.NewHub(logger)
	handler := dashboard.NewHandler(nil, hub, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /ticket", func(w http.ResponseWriter, r *http.Request) {
		principal := &auth.Principal{ID: "test-user", Role: auth.RoleAdmin}
		handler.IssueWebSocketTicket(w, r.WithContext(auth.WithPrincipal(r.Context(), principal)))
	})
	mux.HandleFunc("GET /ws", handler.WebSocket)

	server := httptest.NewServer(coreMiddleware.Logger(logger)(mux))
	defer server.Close()

	response, err := http.Post(server.URL+"/ticket", "application/json", nil)
	if err != nil {
		t.Fatalf("issue ticket: %v", err)
	}
	defer response.Body.Close()
	var issued struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(response.Body).Decode(&issued); err != nil {
		t.Fatalf("decode ticket: %v", err)
	}

	header := http.Header{"Origin": []string{server.URL}}
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?ticket=" + issued.Ticket
	connection, upgradeResponse, err := gorilla.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		if upgradeResponse != nil {
			t.Fatalf("upgrade: %v (status %d)", err, upgradeResponse.StatusCode)
		}
		t.Fatalf("upgrade: %v", err)
	}
	_ = connection.Close()
}
