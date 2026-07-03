package web

import (
	"net/http"
	"time"

	"github.com/schmorrison/goshpanel/internal/fleet"
	"github.com/schmorrison/goshpanel/internal/metricsviz"
	"github.com/schmorrison/goshpanel/internal/store"
	"github.com/schmorrison/goshpanel/internal/system"
)

type metricsData struct {
	Stats      system.Stats
	Samples    []store.MetricSample
	LoadChart  metricsviz.Chart
	MemChart   metricsviz.Chart
	DiskChart  metricsviz.Chart
	Range      string
	RangeLabel string
	Mode       string
	NodeName   string
	Interval   int
	FilesRoot  string
	HasHistory bool
}

func (s *Server) handleMetricsPage(w http.ResponseWriter, r *http.Request) {
	rangeKey := r.URL.Query().Get("range")
	duration, rangeLabel := parseMetricsRange(rangeKey)
	since := time.Now().UTC().Add(-duration)
	limit := sampleLimit(duration, s.cfg.FleetIntervalSeconds)

	samples, err := s.store.MetricSamplesSince(since, limit)
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	if len(samples) == 0 {
		// Fall back to most recent samples when the time window is empty.
		samples, err = s.store.MetricSamples(limit)
		if err != nil {
			redirectError(w, r, "/", err)
			return
		}
	}

	stats := system.Snapshot(s.files.Root())
	loadYMax := metricsviz.LoadYMax(samples, stats.NumCPU)
	loadValues := metricsviz.SamplesLoad(samples)
	memValues := metricsviz.SamplesMem(samples)
	diskValues := metricsviz.SamplesDisk(samples)

	s.render(w, r, "metrics.html", "Performance", "metrics", metricsData{
		Stats:      stats,
		Samples:    samples,
		LoadChart:  metricsviz.BuildChart("load", "Load average", "", "#0f766e", loadValues, loadYMax, 640, 140),
		MemChart:   metricsviz.BuildChart("memory", "Memory", "%", "#2563eb", memValues, 100, 640, 140),
		DiskChart:  metricsviz.BuildChart("disk", "Disk", "%", "#7c3aed", diskValues, 100, 640, 140),
		Range:      rangeKeyOrDefault(rangeKey),
		RangeLabel: rangeLabel,
		Mode:       string(fleet.ParseMode(s.cfg.FleetMode)),
		NodeName:   s.cfg.FleetNodeName,
		Interval:   s.cfg.FleetIntervalSeconds,
		FilesRoot:  s.files.Root(),
		HasHistory: len(samples) > 1,
	})
}

func parseMetricsRange(raw string) (time.Duration, string) {
	switch raw {
	case "6h":
		return 6 * time.Hour, "Last 6 hours"
	case "24h":
		return 24 * time.Hour, "Last 24 hours"
	case "7d":
		return 7 * 24 * time.Hour, "Last 7 days"
	default:
		return time.Hour, "Last hour"
	}
}

func rangeKeyOrDefault(raw string) string {
	switch raw {
	case "6h", "24h", "7d":
		return raw
	default:
		return "1h"
	}
}

func sampleLimit(duration time.Duration, intervalSec int) int {
	if intervalSec <= 0 {
		intervalSec = 60
	}
	n := int(duration.Seconds())/intervalSec + 4
	if n < 30 {
		return 30
	}
	if n > 10080 {
		return 10080
	}
	return n
}
