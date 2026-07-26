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
	ModuleRegistry   *module.Registry
}

type chiRouterWrapper struct {
	chi.Router
	prefix string
}

func (w chiRouterWrapper) Get(pattern string, handlerFn func(interface{}, interface{})) {
	w.Router.Get(w.prefix+pattern, func(rw http.ResponseWriter, r *http.Request) { handlerFn(rw, r) })
}
func (w chiRouterWrapper) Post(pattern string, handlerFn func(interface{}, interface{})) {
	w.Router.Post(w.prefix+pattern, func(rw http.ResponseWriter, r *http.Request) { handlerFn(rw, r) })
}
func (w chiRouterWrapper) Put(pattern string, handlerFn func(interface{}, interface{})) {
	w.Router.Put(w.prefix+pattern, func(rw http.ResponseWriter, r *http.Request) { handlerFn(rw, r) })
}
func (w chiRouterWrapper) Delete(pattern string, handlerFn func(interface{}, interface{})) {
	w.Router.Delete(w.prefix+pattern, func(rw http.ResponseWriter, r *http.Request) { handlerFn(rw, r) })
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
	if deps.DashboardHandler != nil {
		r.Get("/api/v1/dashboard/ws", deps.DashboardHandler.WebSocket)
	}

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
				r.With(coreMiddleware.RequirePermission(auth.PermModuleView)).Get("/modules", deps.ModuleHandler.List)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleView)).Get("/modules/{id}", deps.ModuleHandler.Get)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleView)).Get("/modules/{id}/status", deps.ModuleHandler.Status)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleInstall)).Post("/modules/{id}/install", deps.ModuleHandler.Install)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleUninstall)).Post("/modules/{id}/uninstall", deps.ModuleHandler.Uninstall)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleEnable)).Post("/modules/{id}/enable", deps.ModuleHandler.Enable)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleDisable)).Post("/modules/{id}/disable", deps.ModuleHandler.Disable)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleConfigure)).Post("/modules/{id}/restart", deps.ModuleHandler.Restart)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleConfigure)).Post("/modules/{id}/actions/{actionId}", deps.ModuleHandler.ExecuteAction)
				r.With(coreMiddleware.RequirePermission(auth.PermModuleView)).Get("/jobs/{id}", deps.ModuleHandler.GetJob)
			}

			// Register custom routes for each module
			if deps.ModuleRegistry != nil {
				for _, m := range deps.ModuleRegistry.All() {
					m.RegisterRoutes(chiRouterWrapper{r, "/modules/" + m.ID()})
				}
			}

			// Dashboard routes
			if deps.DashboardHandler != nil {
				r.With(coreMiddleware.RequirePermission(auth.PermDashboardView)).Get("/dashboard", deps.DashboardHandler.Overview)
				r.With(coreMiddleware.RequirePermission(auth.PermDashboardView)).Get("/dashboard/snapshots", deps.DashboardHandler.Snapshots)
				r.With(coreMiddleware.RequirePermission(auth.PermDashboardView)).Post("/dashboard/ws-ticket", deps.DashboardHandler.IssueWebSocketTicket)
			}

			// Site routes
			if deps.SiteHandler != nil {
				r.With(coreMiddleware.RequirePermission(auth.PermSiteView)).Get("/sites", deps.SiteHandler.List)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteCreate)).Post("/sites", deps.SiteHandler.Create)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteView)).Get("/sites/{id}", deps.SiteHandler.Get)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteUpdate)).Put("/sites/{id}", deps.SiteHandler.Update)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteDelete)).Delete("/sites/{id}", deps.SiteHandler.Delete)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteUpdate)).Post("/sites/{id}/enable", deps.SiteHandler.Enable)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteUpdate)).Post("/sites/{id}/disable", deps.SiteHandler.Disable)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteView)).Get("/sites/{id}/files", deps.SiteHandler.ListFiles)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteView)).Get("/sites/{id}/file", deps.SiteHandler.ReadFile)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteUpdate)).Post("/sites/{id}/file", deps.SiteHandler.WriteFile)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteUpdate)).Delete("/sites/{id}/file", deps.SiteHandler.DeleteFile)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteUpdate)).Post("/sites/{id}/directory", deps.SiteHandler.CreateDirectory)
				r.With(coreMiddleware.RequirePermission(auth.PermSiteUpdate)).Post("/sites/{id}/files/batch", deps.SiteHandler.BatchOperations)
			}

			// Service routes
			if deps.ServiceHandler != nil {
				r.With(coreMiddleware.RequirePermission(auth.PermServiceView)).Get("/services", deps.ServiceHandler.List)
				r.With(coreMiddleware.RequirePermission(auth.PermServiceManage)).Post("/services", deps.ServiceHandler.Add)
				r.With(coreMiddleware.RequirePermission(auth.PermServiceManage)).Post("/services/{id}/start", deps.ServiceHandler.Start)
				r.With(coreMiddleware.RequirePermission(auth.PermServiceManage)).Post("/services/{id}/stop", deps.ServiceHandler.Stop)
				r.With(coreMiddleware.RequirePermission(auth.PermServiceManage)).Post("/services/{id}/restart", deps.ServiceHandler.Restart)
				r.With(coreMiddleware.RequirePermission(auth.PermServiceView)).Get("/services/{id}/logs", deps.ServiceHandler.Logs)
				r.With(coreMiddleware.RequirePermission(auth.PermServiceManage)).Delete("/services/{id}", deps.ServiceHandler.Remove)
			}

			// Settings routes
			if deps.SettingsHandler != nil {
				r.With(coreMiddleware.RequirePermission(auth.PermSettingsView)).Get("/settings", deps.SettingsHandler.List)
				r.With(coreMiddleware.RequirePermission(auth.PermSettingsUpdate)).Put("/settings", deps.SettingsHandler.Update)
				r.With(coreMiddleware.RequirePermission(auth.PermSettingsView)).Get("/updates", deps.SettingsHandler.UpdateStatus)
				r.With(coreMiddleware.RequirePermission(auth.PermSettingsSecurity)).Post("/updates/apply", deps.SettingsHandler.ApplyUpdate)
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
