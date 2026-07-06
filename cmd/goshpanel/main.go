// Command goshpanel runs the GoshPanel web control panel.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/schmorrison/goshpanel/internal/analytics"
	"github.com/schmorrison/goshpanel/internal/config"
	"github.com/schmorrison/goshpanel/internal/console"
	"github.com/schmorrison/goshpanel/internal/files"
	"github.com/schmorrison/goshpanel/internal/fleet"
	"github.com/schmorrison/goshpanel/internal/sftpserver"
	"github.com/schmorrison/goshpanel/internal/scheduler"
	"github.com/schmorrison/goshpanel/internal/store"
	"github.com/schmorrison/goshpanel/internal/version"
	"github.com/schmorrison/goshpanel/internal/web"
	"github.com/schmorrison/goshpanel/internal/webdavsrv"
)

func main() {
	console.EnsureVisible()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		fatal(nil, "invalid configuration", err)
	}

	logger, logPath := newLogger(cfg.DataDir)
	logger.Info("goshpanel starting", "version", version.Version, "data_dir", cfg.DataDir, "addr", cfg.Addr, "log_file", logPath)

	st, err := store.Open(filepath.Join(cfg.DataDir, "goshpanel.db"))
	if err != nil {
		fatal(logger, "open store", err)
	}
	defer st.Close()
	logger.Info("database ready")

	srv, err := web.New(cfg, logger, st)
	if err != nil {
		fatal(logger, "start server", err)
	}
	logger.Info("modules initialized")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fleet.StartBackground(ctx, cfg, st, logger)

	if cfg.AnalyticsEnabled {
		interval := time.Duration(cfg.FleetIntervalSeconds) * time.Second
		if interval <= 0 {
			interval = time.Minute
		}
		analytics.StartIngestor(ctx, st, cfg.CaddyAccessLog, interval, logger)
	}

	if cfg.SFTPEnabled {
		fileSvc, err := files.New(cfg.FilesRoot)
		if err != nil {
			fatal(logger, "sftp files root", err)
		}
		sftpSrv, err := sftpserver.New(cfg.SFTPAddr, cfg.SFTPHostKeyPath, fileSvc, st, logger)
		if err != nil {
			fatal(logger, "sftp server", err)
		}
		go func() {
			if err := sftpSrv.Run(ctx); err != nil {
				logger.Error("sftp stopped", "err", err)
			}
		}()
	}

	if cfg.WebDAVEnabled {
		fileSvc, err := files.New(cfg.FilesRoot)
		if err != nil {
			fatal(logger, "webdav files root", err)
		}
		wd := webdavsrv.New(cfg.WebDAVAddr, fileSvc, logger)
		go func() {
			if err := wd.Run(ctx); err != nil {
				logger.Error("webdav stopped", "err", err)
			}
		}()
	}

	go scheduler.Start(ctx, cfg, st, srv.Backups(), srv.Functions(), logger)

	go func() {
		for range time.Tick(time.Hour) {
			if err := st.PruneSessions(); err != nil {
				logger.Error("prune sessions", "err", err)
			}
			if err := st.PruneLoginPending(); err != nil {
				logger.Error("prune login pending", "err", err)
			}
		}
	}()

	httpServer := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		fatal(logger, "listen", fmt.Errorf("%w (is another process using %s?)", err, cfg.Addr))
	}

	openURL := listenURL(cfg.Addr)
	logger.Info("goshpanel listening",
		"addr", ln.Addr().String(),
		"url", openURL,
		"fleet_mode", cfg.FleetMode,
		"node", cfg.FleetNodeName,
		"sftp", cfg.SFTPEnabled,
	)
	fmt.Fprintf(os.Stderr, "GoshPanel is running — open %s in your browser\n", openURL)

	go func() {
		if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped", "err", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logger.Info("shutting down")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

func newLogger(dataDir string) (*slog.Logger, string) {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	writers := []io.Writer{os.Stderr}
	logPath := ""
	if err := os.MkdirAll(dataDir, 0o755); err == nil {
		logPath = filepath.Join(dataDir, "goshpanel.log")
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			writers = append(writers, f)
		} else {
			logPath = ""
		}
	}
	return slog.New(slog.NewTextHandler(io.MultiWriter(writers...), opts)), logPath
}

func fatal(logger *slog.Logger, msg string, err error) {
	if logger != nil {
		logger.Error(msg, "err", err)
	}
	fmt.Fprintf(os.Stderr, "GoshPanel failed: %s: %v\n", msg, err)
	os.Exit(1)
}

func listenURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, ":") {
			return "http://127.0.0.1" + addr
		}
		return "http://" + addr
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}
