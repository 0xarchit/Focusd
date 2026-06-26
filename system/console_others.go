//go:build !windows

package system

// AttachParentConsole is a no-op on non-Windows platforms.
func AttachParentConsole() {
}
