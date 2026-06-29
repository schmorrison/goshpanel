package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/schmorrison/goshpanel/internal/auth"
	"github.com/schmorrison/goshpanel/internal/config"
	"github.com/schmorrison/goshpanel/internal/server"
	"github.com/schmorrison/goshpanel/internal/store"
	"github.com/schmorrison/goshpanel/internal/ui"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.Database.Path), 0o755); err != nil {
		logger.Error("failed to create database directory", "error", err)
		os.Exit(1)
	}

	db, err := store.Open(cfg.Database.Path)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	users := store.NewUserRepository(db)
	sessions := store.NewSessionStore(db)

	secret, generated, err := cfg.SessionSecret()
	if err != nil {
		logger.Error("failed to resolve session secret", "error", err)
		os.Exit(1)
	}
	if generated {
		logger.Warn("auth.session_secret is not set; using an ephemeral secret for this process")
	}

	authService, err := auth.New(users, sessions, secret, logger)
	if err != nil {
		logger.Error("failed to initialize auth", "error", err)
		os.Exit(1)
	}

	if err := authService.Bootstrap(context.Background(), cfg.Auth.BootstrapUsername, cfg.Auth.BootstrapPassword); err != nil {
		logger.Error("failed to bootstrap admin user", "error", err)
		os.Exit(1)
	}

	uiHandler := ui.NewHandler(authService)
	srv := server.New(cfg, logger, authService, uiHandler)
	if err := srv.Run(); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
