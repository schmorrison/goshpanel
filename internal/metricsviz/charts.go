// Package metricsviz builds inline SVG charts from stored metric samples.
package metricsviz

import (
	"fmt"
	"html/template"
	"math"
	"strings"

	"github.com/schmorrison/goshpanel/internal/store"
)

// Chart is a labeled time series with summary stats and rendered SVG.
type Chart struct {
	ID      string
	Label   string
	Unit    string
	Current float64
	Min     float64
	Max     float64
	Avg     float64
	YMax    float64
	SVG     template.HTML
	Color   string
}

// BuildChart renders one metric series as an area + line SVG.
func BuildChart(id, label, unit, color string, values []float64, yMax float64, width, height int) Chart {
	c := Chart{
		ID:    id,
		Label: label,
		Unit:  unit,
		Color: color,
		YMax:  yMax,
	}
	if len(values) == 0 {
		c.SVG = template.HTML(emptyChartSVG(width, height, color))
		return c
	}
	c.Current = values[len(values)-1]
	c.Min, c.Max, c.Avg = summarize(values)
	if c.YMax <= 0 {
		c.YMax = c.Max * 1.15
		if c.YMax < 1 {
			c.YMax = 1
		}
	}
	c.SVG = template.HTML(areaLineSVG(id, values, c.YMax, width, height, color))
	return c
}

// SamplesLoad extracts 1-minute load averages.
func SamplesLoad(samples []store.MetricSample) []float64 {
	out := make([]float64, len(samples))
	for i, s := range samples {
		out[i] = s.Load1
	}
	return out
}

// SamplesMem extracts memory utilization percentages.
func SamplesMem(samples []store.MetricSample) []float64 {
	out := make([]float64, len(samples))
	for i, s := range samples {
		out[i] = s.MemUsedPct
	}
	return out
}

// SamplesDisk extracts disk utilization percentages.
func SamplesDisk(samples []store.MetricSample) []float64 {
	out := make([]float64, len(samples))
	for i, s := range samples {
		out[i] = s.DiskUsedPct
	}
	return out
}

// LoadYMax picks a sensible Y-axis maximum for load charts.
func LoadYMax(samples []store.MetricSample, numCPU int) float64 {
	if numCPU <= 0 {
		numCPU = 1
	}
	ceiling := float64(numCPU) * 2
	max := 0.0
	for _, s := range samples {
		if s.Load1 > max {
			max = s.Load1
		}
	}
	if max > ceiling {
		return math.Ceil(max*1.15*10) / 10
	}
	return ceiling
}

// GaugeLevel returns a CSS class for utilization gauges.
func GaugeLevel(pct float64) string {
	switch {
	case pct >= 90:
		return "gauge-crit"
	case pct >= 75:
		return "gauge-warn"
	default:
		return "gauge-ok"
	}
}

// LoadUtilPct normalizes load average against CPU count for gauge display.
func LoadUtilPct(load1 float64, numCPU int) float64 {
	if numCPU <= 0 {
		numCPU = 1
	}
	pct := load1 / float64(numCPU) * 100
	if pct > 100 {
		return 100
	}
	return pct
}

func summarize(values []float64) (min, max, avg float64) {
	if len(values) == 0 {
		return
	}
	min = values[0]
	max = values[0]
	var sum float64
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}
	avg = sum / float64(len(values))
	return
}

func areaLineSVG(id string, values []float64, yMax float64, width, height int, color string) string {
	if len(values) == 0 {
		return emptyChartSVG(width, height, color)
	}
	const pad = 6.0
	w := float64(width) - 2*pad
	h := float64(height) - 2*pad
	denom := float64(len(values) - 1)
	if denom < 1 {
		denom = 1
	}

	points := make([]string, len(values))
	for i, v := range values {
		x := pad + w*float64(i)/denom
		y := pad + h - (clamp(v, 0, yMax)/yMax)*h
		points[i] = fmt.Sprintf("%.1f,%.1f", x, y)
	}

	line := strings.Join(points, " ")
	area := fmt.Sprintf("M %.1f,%.1f L %s L %.1f,%.1f Z",
		pad, pad+h, line, pad+w, pad+h)
	gradID := "grad-" + id

	return fmt.Sprintf(
		`<svg class="perf-chart" viewBox="0 0 %d %d" preserveAspectRatio="none" role="img" aria-label="%s chart">`+
			`<defs><linearGradient id="%s" x1="0" y1="0" x2="0" y2="1">`+
			`<stop offset="0%%" stop-color="%s" stop-opacity="0.35"/>`+
			`<stop offset="100%%" stop-color="%s" stop-opacity="0.03"/></linearGradient></defs>`+
			`<path d="%s" fill="url(#%s)"/>`+
			`<polyline points="%s" fill="none" stroke="%s" stroke-width="2" vector-effect="non-scaling-stroke"/>`+
			`</svg>`,
		width, height, id, gradID, color, color, area, gradID, line, color)
}

func emptyChartSVG(width, height int, color string) string {
	return fmt.Sprintf(
		`<svg class="perf-chart perf-chart-empty" viewBox="0 0 %d %d" preserveAspectRatio="none" role="img" aria-hidden="true">`+
			`<line x1="6" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-opacity="0.2" stroke-dasharray="4 6"/></svg>`,
		width, height, height/2, width-6, height/2, color)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Sparkline renders a compact chart for dashboard previews.
func Sparkline(values []float64, yMax float64, width, height int, color string) template.HTML {
	return BuildChart("spark", "", "", color, values, yMax, width, height).SVG
}
