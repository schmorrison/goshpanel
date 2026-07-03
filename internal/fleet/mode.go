package fleet

import "strings"

// ParseMode normalizes a fleet mode string.
func ParseMode(raw string) Mode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "controller":
		return ModeController
	case "worker":
		return ModeWorker
	case "both":
		return ModeBoth
	default:
		return ModeStandalone
	}
}

// IsController reports whether this instance should manage a fleet.
func IsController(m Mode) bool { return m == ModeController || m == ModeBoth }

// IsWorker reports whether this instance should push telemetry upstream.
func IsWorker(m Mode) bool { return m == ModeWorker || m == ModeBoth }
