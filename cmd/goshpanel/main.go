// Command goshpanel runs the GoshPanel web control panel.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/schmorrison/goshpanel/internal/config"
	"github.com/schmorrison/goshpanel/internal/fleet"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fleet.StartBackground(ctx, cfg, st, logger)

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

	go func() {
		logger.Info("goshpanel listening", "addr", cfg.Addr, "fleet_mode", cfg.FleetMode, "node", cfg.FleetNodeName)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped", "err", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
}
