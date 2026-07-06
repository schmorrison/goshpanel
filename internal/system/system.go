// Package system reads host metrics (CPU, memory, disk, uptime, load)
// directly from the OS — the cPanel "Server Information" and dashboard
// statistics equivalents, with no external tooling.
package system

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// Stats is a snapshot of host resource usage.
type Stats struct {
	Hostname     string
	OS           string
	Arch         string
	NumCPU       int
	GoVersion    string
	Uptime       time.Duration
	Load1        float64
	Load5        float64
	Load15       float64
	MemTotalKB   uint64
	MemFreeKB    uint64
	MemUsedKB    uint64
	MemUsedPct   float64
	DiskTotalKB  uint64
	DiskFreeKB   uint64
	DiskUsedKB   uint64
	DiskUsedPct  float64
	KernelVer    string
	PanelPID     int
	PanelStarted time.Time
}

var panelStart = time.Now()

// Snapshot gathers current stats. Individual probe failures leave zero
// values rather than failing the whole snapshot.
func Snapshot(diskPath string) Stats {
	st := Stats{
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		NumCPU:       runtime.NumCPU(),
		GoVersion:    runtime.Version(),
		PanelPID:     os.Getpid(),
		PanelStarted: panelStart,
	}
	st.Hostname, _ = os.Hostname()
	st.Uptime = readUptime()
	st.Load1, st.Load5, st.Load15 = readLoadAvg()
	st.MemTotalKB, st.MemFreeKB = readMemInfo()
	if st.MemTotalKB > 0 {
		st.MemUsedKB = st.MemTotalKB - st.MemFreeKB
		st.MemUsedPct = float64(st.MemUsedKB) / float64(st.MemTotalKB) * 100
	}
	st.DiskTotalKB, st.DiskFreeKB = readDiskUsage(diskPath)
	if st.DiskTotalKB > 0 {
		st.DiskUsedKB = st.DiskTotalKB - st.DiskFreeKB
		st.DiskUsedPct = float64(st.DiskUsedKB) / float64(st.DiskTotalKB) * 100
	}
	st.KernelVer = readKernelVersion()
	return st
}

// HumanKB formats a KiB quantity like "1.5 GiB".
func HumanKB(kb uint64) string {
	const unit = 1024
	v := float64(kb)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB"} {
		if v < unit {
			return fmt.Sprintf("%.1f %s", v, suffix)
		}
		v /= unit
	}
	return fmt.Sprintf("%.1f PiB", v)
}
