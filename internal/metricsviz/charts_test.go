package metricsviz

import (
	"strings"
	"testing"
	"time"

	"github.com/schmorrison/goshpanel/internal/store"
)

func TestBuildChart(t *testing.T) {
	c := BuildChart("load", "Load", "", "#0f766e", []float64{0.5, 1.0, 0.8, 1.2}, 2, 400, 120)
	if c.Current != 1.2 || c.Min != 0.5 || c.Max != 1.2 {
		t.Errorf("stats = %+v", c)
	}
	if !strings.Contains(string(c.SVG), "<svg") || !strings.Contains(string(c.SVG), "polyline") {
		t.Errorf("svg = %s", c.SVG)
	}
}

func TestGaugeLevel(t *testing.T) {
	if GaugeLevel(50) != "gauge-ok" || GaugeLevel(80) != "gauge-warn" || GaugeLevel(95) != "gauge-crit" {
		t.Error("gauge levels")
	}
}

func TestSamplesExtractors(t *testing.T) {
	now := time.Now()
	samples := []store.MetricSample{
		{Load1: 1.1, MemUsedPct: 40, DiskUsedPct: 55, RecordedAt: now},
		{Load1: 2.2, MemUsedPct: 50, DiskUsedPct: 60, RecordedAt: now},
	}
	if got := SamplesLoad(samples); len(got) != 2 || got[1] != 2.2 {
		t.Errorf("load = %v", got)
	}
	if got := SamplesMem(samples); got[0] != 40 {
		t.Errorf("mem = %v", got)
	}
	if ymax := LoadYMax(samples, 4); ymax < 2.2 {
		t.Errorf("ymax = %v", ymax)
	}
}
