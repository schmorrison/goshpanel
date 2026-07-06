//go:build !linux && !windows

package system

import "time"

func readUptime() time.Duration {
	return 0
}

func readLoadAvg() (l1, l5, l15 float64) {
	return 0, 0, 0
}

func readMemInfo() (totalKB, availKB uint64) {
	return 0, 0
}

func readDiskUsage(path string) (totalKB, freeKB uint64) {
	return 0, 0
}

func readKernelVersion() string {
	return ""
}
