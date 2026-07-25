// Package server implements the OpenDeploy Core HTTP server.
//
// Responsibilities:
//   - Create and configure the chi router with the full middleware chain.
//   - Register all domain handler routes under /api/v1/.
//   - Serve the embedded Vue SPA for all non-API paths.
//   - Expose /health for load balancer probes.
//   - Provide graceful shutdown via Shutdown(ctx).
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/anrted/opendeploy/internal/core/auth"
	"github.com/anrted/opendeploy/internal/core/dashboard"
	"github.com/anrted/opendeploy/internal/core/module"
	coreMiddleware "github.com/anrted/opendeploy/internal/core/server/middleware"
	"github.com/anrted/opendeploy/internal/core/service"
	"github.com/anrted/opendeploy/internal/core/settings"
	"github.com/anrted/opendeploy/internal/core/site"
	"github.com/anrted/opendeploy/internal/core/webui"
	"github.com/anrted/opendeploy/internal/platform/config"
	"github.com/anrted/opendeploy/internal/platform/websocket"
)

// Server wraps net/http.Server with additional lifecycle management.
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// Dependencies groups all handler dependencies needed to build the router.
type Dependencies struct {
	Config           *config.Config
	AuthHandler      *auth.Handler
	JWTManager       *auth.JWTManager
	ModuleHandler    *module.Handler
	WSHub            *websocket.Hub
	DashboardHandler *dashboard.Handler
	SiteHandler      *site.Handler
	ServiceHandler   *service.Handler
	SettingsHandler  *settings.Handler
}

// New constructs a Server with the full middleware chain and route tree.
func New(deps Dependencies, logger *slog.Logger) *Server {
	r := buildRouter(deps, logger)

	addr := deps.Config.Addr()
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  deps.Config.Server.ReadTimeout,
		WriteTimeout: deps.Config.Server.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	return &Server{httpServer: srv, logger: logger}
}

// Start begins accepting connections. It blocks until the server closes.
func (s *Server) Start() error {
	s.logger.Info("http server started", "addr", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: listen: %w", err)
	}
	return nil
}

// Shutdown gracefully drains active connections within the given timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("http server shutting down")
	return s.httpServer.Shutdown(ctx)
}

// ─── Router ────────────────────────────────────────────────────────────────

func buildRouter(deps Dependencies, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	// ── Global middleware ─────────────────────────────────────────────────
	r.Use(coreMiddleware.Recover)
	r.Use(coreMiddleware.Logger(logger))
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	if deps.Config.Security.RateLimit.Enabled {
		r.Use(coreMiddleware.RateLimit(deps.Config.Security.RateLimit.RequestsPerMinute))
	}

	// ── Health check (unauthenticated) ────────────────────────────────────
	r.Get("/health", healthHandler)

	// ── API v1 ────────────────────────────────────────────────────────────
	r.Route("/api/v1", func(r chi.Router) {
		// Public auth endpoints
		r.Post("/auth/login", deps.AuthHandler.Login)
		r.Post("/auth/refresh", deps.AuthHandler.Refresh)

		// Protected endpoints — require valid JWT
		r.Group(func(r chi.Router) {
			r.Use(coreMiddleware.Auth(deps.JWTManager))

			r.Post("/auth/logout", deps.AuthHandler.Logout)
			r.Get("/auth/me", deps.AuthHandler.Me)

			// Module routes
			if deps.ModuleHandler != nil {
				r.Get("/modules", deps.ModuleHandler.List)
				r.Get("/modules/{id}", deps.ModuleHandler.Get)
				r.Post("/modules/{id}/install", deps.ModuleHandler.Install)
				r.Post("/modules/{id}/uninstall", deps.ModuleHandler.Uninstall)
				r.Post("/modules/{id}/enable", deps.ModuleHandler.Enable)
				r.Post("/modules/{id}/disable", deps.ModuleHandler.Disable)
				r.Post("/modules/{id}/restart", deps.ModuleHandler.Restart)
				r.Get("/jobs/{id}", deps.ModuleHandler.GetJob)
			}

			// Dashboard routes
			if deps.DashboardHandler != nil {
				r.Get("/dashboard", deps.DashboardHandler.Overview)
				r.Get("/dashboard/snapshots", deps.DashboardHandler.Snapshots)
				r.Get("/dashboard/ws", deps.DashboardHandler.WebSocket)
			}

			// Site routes
			if deps.SiteHandler != nil {
				r.Get("/sites", deps.SiteHandler.List)
				r.Post("/sites", deps.SiteHandler.Create)
				r.Get("/sites/{id}", deps.SiteHandler.Get)
				r.Put("/sites/{id}", deps.SiteHandler.Update)
				r.Delete("/sites/{id}", deps.SiteHandler.Delete)
				r.Post("/sites/{id}/enable", deps.SiteHandler.Enable)
				r.Post("/sites/{id}/disable", deps.SiteHandler.Disable)
			}

			// Service routes
			if deps.ServiceHandler != nil {
				r.Get("/services", deps.ServiceHandler.List)
				r.Post("/services", deps.ServiceHandler.Add)
				r.Post("/services/{id}/start", deps.ServiceHandler.Start)
				r.Post("/services/{id}/stop", deps.ServiceHandler.Stop)
				r.Post("/services/{id}/restart", deps.ServiceHandler.Restart)
				r.Get("/services/{id}/logs", deps.ServiceHandler.Logs)
				r.Delete("/services/{id}", deps.ServiceHandler.Remove)
			}

			// Settings routes
			if deps.SettingsHandler != nil {
				r.Get("/settings", deps.SettingsHandler.List)
				r.Put("/settings", deps.SettingsHandler.Update)
			}
		})
	})

	// ── SPA catch-all: serve index.html for all non-API routes ───────────
	// The Vue SPA handles its own routing via Vue Router.
	r.Handle("/*", webui.Handler())

	return r
}

// ─── Built-in handlers ─────────────────────────────────────────────────────

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok","service":"opendeploy-core"}`))
}
