package web

import (
	"net/http"

	"github.com/schmorrison/goshpanel/internal/fleet"
	"github.com/schmorrison/goshpanel/internal/store"
)

type metricsData struct {
	Samples  []store.MetricSample
	Mode     string
	NodeName string
	Interval int
}

func (s *Server) handleMetricsPage(w http.ResponseWriter, r *http.Request) {
	samples, err := s.store.MetricSamples(120)
	if err != nil {
		redirectError(w, r, "/", err)
		return
	}
	s.render(w, r, "metrics.html", "Metrics", "metrics", metricsData{
		Samples:  samples,
		Mode:     string(fleet.ParseMode(s.cfg.FleetMode)),
		NodeName: s.cfg.FleetNodeName,
		Interval: s.cfg.FleetIntervalSeconds,
	})
}
