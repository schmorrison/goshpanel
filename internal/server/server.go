package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/schmorrison/goshpanel/internal/auth"
	"github.com/schmorrison/goshpanel/internal/config"
	"github.com/schmorrison/goshpanel/internal/ui"
	"github.com/schmorrison/goshpanel/web"
)

// Server is the GoshPanel HTTP server.
type Server struct {
	cfg    config.Config
	logger *slog.Logger
	http   *http.Server
}

// New constructs a configured HTTP server.
func New(cfg config.Config, logger *slog.Logger, authService *auth.Service, uiHandler *ui.Handler) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	staticFS, err := fs.Sub(web.Static, "static")
	if err != nil {
		panic(fmt.Errorf("static filesystem: %w", err))
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(logger))
	r.Use(authService.SessionMiddleware())

	r.Get("/healthz", healthHandler)
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	r.Group(func(r chi.Router) {
		r.Get("/", uiHandler.Home)
	})

	r.Group(func(r chi.Router) {
		r.Use(authService.RedirectIfAuthenticated)
		r.Get("/login", uiHandler.LoginGet)
		r.With(authService.CSRFProtect).Post("/login", uiHandler.LoginPost)
	})

	r.Group(func(r chi.Router) {
		r.Use(authService.RequireAuth)
		r.With(authService.CSRFProtect).Post("/logout", uiHandler.LogoutPost)
		r.Get("/dashboard", uiHandler.Dashboard)
		r.Get("/files", uiHandler.Files)
		r.Get("/files/list", uiHandler.FilesList)
		r.With(authService.CSRFProtect).Post("/files/upload", uiHandler.FilesUpload)
		r.With(authService.CSRFProtect).Post("/files/delete", uiHandler.FilesDelete)
		r.Get("/domains", uiHandler.Domains)
		r.With(authService.CSRFProtect).Post("/domains/create", uiHandler.DomainsCreate)
		r.With(authService.CSRFProtect).Post("/domains/delete", uiHandler.DomainsDelete)
		r.With(authService.CSRFProtect).Post("/domains/apply", uiHandler.DomainsApply)
		r.Get("/logging", uiHandler.Logging)
		r.Get("/logging/tail", uiHandler.LoggingTail)
		r.Get("/terminal", uiHandler.Terminal)
		r.Get("/terminal/ws", uiHandler.TerminalWS)
	})

	return &Server{
		cfg:    cfg,
		logger: logger,
		http: &http.Server{
			Addr:        cfg.Addr(),
			Handler:     r,
			ReadTimeout: 30 * time.Second,
			IdleTimeout: 120 * time.Second,
		},
	}
}

// Run starts the HTTP server and blocks until it stops.
func (s *Server) Run() error {
	s.logger.Info("starting goshpanel", "addr", s.cfg.Addr())
	if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration", time.Since(start),
			)
		})
	}
}
