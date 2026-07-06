package system

import (
	"runtime"
	"testing"
)

func TestSnapshot(t *testing.T) {
	st := Snapshot("/")
	if st.Hostname == "" {
		t.Error("Hostname empty")
	}
	if st.NumCPU < 1 {
		t.Errorf("NumCPU = %d", st.NumCPU)
	}
	switch runtime.GOOS {
	case "linux":
		if st.MemTotalKB == 0 {
			t.Error("MemTotalKB = 0 (is /proc/meminfo readable?)")
		}
		if st.DiskTotalKB == 0 {
			t.Error("DiskTotalKB = 0")
		}
		if st.Uptime <= 0 {
			t.Error("Uptime not positive")
		}
	case "windows":
		if st.MemTotalKB == 0 {
			t.Error("MemTotalKB = 0")
		}
		if st.DiskTotalKB == 0 {
			t.Error("DiskTotalKB = 0")
		}
		if st.Uptime <= 0 {
			t.Error("Uptime not positive")
		}
	}
}

func TestHumanKB(t *testing.T) {
	cases := map[uint64]string{
		512:             "512.0 KiB",
		2048:            "2.0 MiB",
		3 * 1024 * 1024: "3.0 GiB",
	}
	for in, want := range cases {
		if got := HumanKB(in); got != want {
			t.Errorf("HumanKB(%d) = %q, want %q", in, got, want)
		}
	}
}
