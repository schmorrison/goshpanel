package analytics

import (
	"context"
	"log/slog"
	"time"

	"github.com/schmorrison/goshpanel/internal/store"
)

// StartIngestor tails the Caddy access log and stores parsed events.
func StartIngestor(ctx context.Context, st *store.Store, logPath string, interval time.Duration, log *slog.Logger) {
	if logPath == "" {
		return
	}
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		tick := time.NewTicker(interval)
		defer tick.Stop()
		ingest := func() {
			if _, err := IngestFile(st, logPath); err != nil && log != nil {
				log.Warn("analytics ingest", "err", err)
			}
		}
		ingest()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				ingest()
			}
		}
	}()
}
