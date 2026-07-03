package metricsviz

import (
	"encoding/json"

	"github.com/schmorrison/goshpanel/internal/fleet"
	"github.com/schmorrison/goshpanel/internal/store"
)

// ChartsFromFleetHistory builds charts from stored fleet telemetry JSON payloads.
func ChartsFromFleetHistory(samples []store.FleetTelemetrySample, numCPU int) (load, mem, disk Chart) {
	loadVals := make([]float64, 0, len(samples))
	memVals := make([]float64, 0, len(samples))
	diskVals := make([]float64, 0, len(samples))
	for _, s := range samples {
		var t fleet.Telemetry
		if json.Unmarshal([]byte(s.Payload), &t) != nil {
			continue
		}
		loadVals = append(loadVals, t.Stats.Load1)
		memVals = append(memVals, t.Stats.MemUsedPct)
		diskVals = append(diskVals, t.Stats.DiskUsedPct)
		if numCPU <= 0 && t.Stats.NumCPU > 0 {
			numCPU = t.Stats.NumCPU
		}
	}
	if len(loadVals) == 0 {
		return Chart{}, Chart{}, Chart{}
	}
	if numCPU <= 0 {
		numCPU = 1
	}
	return BuildChart("fleet-load", "Load", "", "#22d3ee", loadVals, LoadYMaxFromValues(loadVals, numCPU), 480, 100),
		BuildChart("fleet-mem", "Memory", "%", "#818cf8", memVals, 100, 480, 100),
		BuildChart("fleet-disk", "Disk", "%", "#c084fc", diskVals, 100, 480, 100)
}

// LoadYMaxFromValues picks a Y-axis max for load value slices.
func LoadYMaxFromValues(values []float64, numCPU int) float64 {
	if numCPU <= 0 {
		numCPU = 1
	}
	max := 0.0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	ceiling := float64(numCPU) * 2
	if max > ceiling {
		return max * 1.15
	}
	return ceiling
}
