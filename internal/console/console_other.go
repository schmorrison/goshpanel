//go:build !windows

package console

// EnsureVisible is a no-op on non-Windows platforms.
func EnsureVisible() {}
