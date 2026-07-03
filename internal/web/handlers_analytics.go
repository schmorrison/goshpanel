package web

import (
	"net/http"
	"time"

	"github.com/schmorrison/goshpanel/internal/metricsviz"
	"github.com/schmorrison/goshpanel/internal/store"
)

type analyticsData struct {
	Range      string
	RangeLabel string
	Summary    store.AccessLogSummary
	TopPaths   []store.TopPathCount
	Chart      metricsviz.Chart
	LogPath    string
}

func (s *Server) handleAnalyticsPage(w http.ResponseWriter, r *http.Request) {
	rangeKey := r.URL.Query().Get("range")
	duration, rangeLabel := parseMetricsRange(rangeKey)
	since := time.Now().UTC().Add(-duration)

	summary, err := s.store.AccessLogSummarySince(since)
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	paths, err := s.store.TopAccessPaths(since, 15)
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	counts, _, err := s.store.AccessLogHourly(since)
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	floatCounts := make([]float64, len(counts))
	for i, c := range counts {
		floatCounts[i] = float64(c)
	}
	chart := metricsviz.BuildChart("analytics", "Requests", "", "#0f766e", floatCounts, 0, 640, 120)

	s.render(w, r, "analytics.html", "Analytics", "analytics", analyticsData{
		Range:      rangeKeyOrDefault(rangeKey),
		RangeLabel: rangeLabel,
		Summary:    summary,
		TopPaths:   paths,
		Chart:      chart,
		LogPath:    s.cfg.CaddyAccessLog,
	})
}
