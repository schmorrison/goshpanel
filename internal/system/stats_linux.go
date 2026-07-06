//go:build linux

package system

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

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
