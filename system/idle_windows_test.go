package system

import "testing"

func TestGetIdleSecondsIsSane(t *testing.T) {
	seconds, err := GetIdleSeconds()
	if err != nil {
		t.Skipf("idle detection unavailable on this system: %v", err)
	}

	// A negative value would mean the tick counter wrapped mid-call, which the
	// uint32 subtraction is meant to prevent.
	if seconds < 0 {
		t.Errorf("idle seconds = %d; want a non-negative value", seconds)
	}

	// A machine cannot have been idle for years in a test run. This catches a
	// mis-sized structure or a wrong struct layout rather than a real idle.
	const maxPlausible = 60 * 60 * 24 * 365
	if seconds > maxPlausible {
		t.Errorf("idle seconds = %d; implausibly large, struct layout is likely wrong", seconds)
	}
}

func TestGetTickCountAdvances(t *testing.T) {
	first := GetTickCount()
	if first == 0 {
		t.Skip("tick count unavailable")
	}
	// No sleep: just confirm a second read is not smaller, which would suggest
	// a wrong calling convention.
	second := GetTickCount()
	if second < first {
		t.Errorf("tick count went backwards: %d then %d", first, second)
	}
}