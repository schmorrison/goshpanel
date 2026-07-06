//go:build windows

package console

import "syscall"

var (
	kernel32      = syscall.NewLazyDLL("kernel32.dll")
	allocConsole  = kernel32.NewProc("AllocConsole")
	attachConsole = kernel32.NewProc("AttachConsole")
)

const attachParentProcess = ^uintptr(0)

// EnsureVisible attaches to a parent console or allocates one so output is
// visible when the binary is started from Explorer instead of a terminal.
func EnsureVisible() {
	if r, _, _ := attachConsole.Call(attachParentProcess); r != 0 {
		return
	}
	_, _, _ = allocConsole.Call()
}
