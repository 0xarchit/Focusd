//go:build race

package system

// queryVersionLabel is disabled under the race detector.
//
// The race detector instruments memory access with checkptr, which validates
// that a pointer arithmetic result lands inside a Go allocation. VerQueryValueW
// returns a pointer into a buffer owned by the Windows loader, so checkptr
// aborts the process even though the read is bounds checked against that same
// buffer. The bounds are correct; the instrumentation simply cannot see them.
//
// Everything else still runs under -race, which is where the tracker and config
// races that matter are caught. Losing version labels for the duration of a
// race build is a fair trade for being able to run the detector at all.

func queryVersionLabel(exePath string) string {
	return ""
}