// Package telemetry samples local host metrics into SQLite for historical charts.
package telemetry

import (
	"context"
	"log/slog"
	"time"

	"github.com/schmorrison/goshpanel/internal/store"
	"github.com/schmorrison/goshpanel/internal/system"
)

// Sampler records metric snapshots on an interval.
type Sampler struct {
	store    *store.Store
	diskPath string
	interval time.Duration
	log      *slog.Logger
}

// NewSampler creates a metrics sampler.
func NewSampler(st *store.Store, diskPath string, interval time.Duration, log *slog.Logger) *Sampler {
	return &Sampler{store: st, diskPath: diskPath, interval: interval, log: log}
}

// Run blocks until ctx is cancelled, sampling every interval.
func (s *Sampler) Run(ctx context.Context) {
	if s.interval <= 0 {
		s.interval = time.Minute
	}
	s.sample() // immediate first sample
	tick := time.NewTicker(s.interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.sample()
		}
	}
}

func (s *Sampler) sample() {
	st := system.Snapshot(s.diskPath)
	if err := s.store.RecordMetricSample(st.Load1, st.MemUsedPct, st.DiskUsedPct); err != nil && s.log != nil {
		s.log.Error("record metric sample", "err", err)
	}
}
