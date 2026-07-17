//go:build !windows

package system

func AttachParentConsole() {
}

func DisableConsoleScrollback() (int16, int16) {
	return 0, 0
}

func RestoreConsoleBufferSize(width, height int16) {
}
