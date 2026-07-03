package fleet

import (
	"context"
	"log/slog"
	"time"

	"github.com/schmorrison/goshpanel/internal/config"
	"github.com/schmorrison/goshpanel/internal/store"
	"github.com/schmorrison/goshpanel/internal/telemetry"
)

// StartBackground launches fleet poll/push and metrics sampling goroutines.
func StartBackground(ctx context.Context, cfg config.Config, st *store.Store, log *slog.Logger) {
	interval := time.Duration(cfg.FleetIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = time.Minute
	}

	if cfg.MetricsEnabled {
		go telemetry.NewSampler(st, cfg.FilesRoot, interval, log).Run(ctx)
	}

	mode := ParseMode(cfg.FleetMode)
	collector := NewCollector(st, cfg.FleetNodeName, cfg.FilesRoot)
	client := NewClient()

	if IsController(mode) {
		ctrl := NewController(st)
		go func() {
			tick := time.NewTicker(interval)
			defer tick.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tick.C:
					for _, err := range ctrl.PollAll(ctx) {
						if log != nil {
							log.Warn("fleet poll", "err", err)
						}
					}
				}
			}
		}()
	}

	if IsWorker(mode) {
		go runWorker(ctx, cfg, collector, client, interval, log)
	}
}

func runWorker(ctx context.Context, cfg config.Config, collector *Collector, client *Client, interval time.Duration, log *slog.Logger) {
	tick := time.NewTicker(interval)
	defer tick.Stop()

	push := func(rt WorkerRuntime) {
		collector.nodeName = rt.NodeName
		t, err := collector.Snapshot()
		if err != nil {
			return
		}
		if err := client.PushTelemetry(ctx, rt.ControllerURL, rt.NodeToken, IngestRequest{
			NodeName: rt.NodeName, Telemetry: t,
		}); err != nil && log != nil {
			log.Warn("fleet push", "err", err)
		}
	}

	for {
		rt := ResolveWorkerRuntime(cfg)
		if WorkerNeedsEnroll(cfg, rt) {
			creds, err := EnrollWorker(ctx, cfg, client)
			if err != nil {
				if log != nil {
					log.Warn("fleet enroll", "err", err)
				}
			} else {
				rt.NodeName = creds.NodeName
				rt.NodeToken = creds.NodeToken
				rt.ControllerURL = creds.ControllerURL
				if log != nil {
					log.Info("fleet enrolled", "node", creds.NodeName, "controller", creds.ControllerURL)
				}
			}
		}

		if rt.ControllerURL != "" && rt.NodeToken != "" {
			push(rt)
		}

		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
