// Command goshpanel runs the GoshPanel web control panel.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/schmorrison/goshpanel/internal/config"
	"github.com/schmorrison/goshpanel/internal/store"
	"github.com/schmorrison/goshpanel/internal/web"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := config.LoadFromEnv()
	if err != nil {
		logger.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	st, err := store.Open(filepath.Join(cfg.DataDir, "goshpanel.db"))
	if err != nil {
		logger.Error("open store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	srv, err := web.New(cfg, logger, st)
	if err != nil {
		logger.Error("start server", "err", err)
		os.Exit(1)
	}

	// Prune expired sessions periodically.
	go func() {
		for range time.Tick(time.Hour) {
			if err := st.PruneSessions(); err != nil {
				logger.Error("prune sessions", "err", err)
			}
		}
	}()

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	logger.Info("goshpanel listening", "addr", cfg.Addr)
	if err := httpServer.ListenAndServe(); err != nil {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
