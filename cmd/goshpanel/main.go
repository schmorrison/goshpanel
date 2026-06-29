package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/schmorrison/goshpanel/internal/auth"
	"github.com/schmorrison/goshpanel/internal/config"
	"github.com/schmorrison/goshpanel/internal/dbmanager"
	"github.com/schmorrison/goshpanel/internal/domains"
	"github.com/schmorrison/goshpanel/internal/email"
	"github.com/schmorrison/goshpanel/internal/files"
	logpanel "github.com/schmorrison/goshpanel/internal/logging"
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
	if err := os.MkdirAll(cfg.Files.Root, 0o755); err != nil {
		logger.Error("failed to create files sandbox directory", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Caddy.ConfigPath), 0o755); err != nil {
		logger.Error("failed to create caddy config directory", "error", err)
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
	audit := store.NewAuditRepository(db)
	sites := store.NewSiteRepository(db)
	connections := store.NewConnectionRepository(db)
	mailboxes := store.NewMailboxRepository(db)

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

	filesService, err := files.NewService(cfg.Files.Root, audit)
	if err != nil {
		logger.Error("failed to initialize file manager", "error", err)
		os.Exit(1)
	}

	domainsService := domains.NewService(sites, cfg.Caddy.AdminURL, cfg.Caddy.ConfigPath)

	logSources := make([]logpanel.Source, 0, len(cfg.Logging.Sources))
	for _, source := range cfg.Logging.Sources {
		logSources = append(logSources, logpanel.Source{Name: source.Name, Path: source.Path})
	}
	loggingService, err := logpanel.NewService(logSources)
	if err != nil {
		logger.Error("failed to initialize logging service", "error", err)
		os.Exit(1)
	}

	dbmanagerService := dbmanager.NewService(connections, secret)
	emailService := email.NewService(mailboxes, secret, cfg.Email.DefaultDomain, cfg.Email.ConfigPath)

	uiHandler := ui.NewHandler(
		authService,
		filesService,
		domainsService,
		loggingService,
		dbmanagerService,
		emailService,
		cfg.Email.DefaultDomain,
		cfg.Terminal.Enabled,
		cfg.Terminal.Shell,
		cfg.Terminal.Workdir,
	)
	srv := server.New(cfg, logger, authService, uiHandler)
	if err := srv.Run(); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
