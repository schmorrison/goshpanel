// Package system reads host metrics (CPU, memory, disk, uptime, load)
// directly from /proc and syscalls — the cPanel "Server Information" and
// dashboard statistics equivalents, with no external tooling.
package system

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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

func readUptime() time.Duration {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return time.Duration(secs * float64(time.Second))
}

func readLoadAvg() (l1, l5, l15 float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return
	}
	l1, _ = strconv.ParseFloat(fields[0], 64)
	l5, _ = strconv.ParseFloat(fields[1], 64)
	l15, _ = strconv.ParseFloat(fields[2], 64)
	return
}

func readMemInfo() (totalKB, availKB uint64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		var target *uint64
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			target = &totalKB
		case strings.HasPrefix(line, "MemAvailable:"):
			target = &availKB
		default:
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			*target, _ = strconv.ParseUint(fields[1], 10, 64)
		}
	}
	return
}

func readDiskUsage(path string) (totalKB, freeKB uint64) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return
	}
	bs := uint64(fs.Bsize)
	return fs.Blocks * bs / 1024, fs.Bavail * bs / 1024
}

func readKernelVersion() string {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
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
