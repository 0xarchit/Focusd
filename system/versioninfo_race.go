//go:build race

package system

// queryVersionLabel returns nothing under the race detector: checkptr cannot
// verify a pointer into a loader-owned buffer, so it aborts the process. The
// bounds are correct; the instrumentation cannot see them.
func queryVersionLabel(exePath string) string {
	return ""
}
