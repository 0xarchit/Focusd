package core

import (
	"errors"
	"focusd/storage"
	"focusd/system"
	"testing"
	"time"
)

func testClockSample(at time.Time, seconds uint64, boot string) system.ClockSample {
	return system.ClockSample{WallUTC: at, InterruptTicks: seconds * clockTicksPerSecond, UnbiasedTicks: seconds * clockTicksPerSecond, BootID: boot, ZoneKey: "Test"}
}
func testClockGuard(t *testing.T) (*ClockGuard, system.ClockSample) {
	t.Helper()
	setupTestDB(t)
	guard, err := NewClockGuard()
	if err != nil {
		t.Fatal(err)
	}
	initial := testClockSample(time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC), 1000, "boot")
	event, err := guard.Observe(initial, true)
	if err != nil || event.Incident != nil {
		t.Fatalf("baseline = %+v, %v", event, err)
	}
	return guard, initial
}
func TestClockGuardRejectsWallJumpWithoutElapsedJump(t *testing.T) {
	for _, jump := range []time.Duration{-time.Hour, time.Hour} {
		t.Run(jump.String(), func(t *testing.T) {
			guard, initial := testClockGuard(t)
			changed := testClockSample(initial.WallUTC.Add(jump), 1001, "boot")
			event, err := guard.Observe(changed, false)
			if err != nil || !event.Created || event.Incident == nil || !event.LastSafe.Equal(initial.WallUTC) {
				t.Fatalf("jump event=%+v err=%v", event, err)
			}
			if !storage.TimeGuarded() || !storage.RetentionHeld() {
				t.Fatal("jump did not guard persisted state")
			}
		})
	}
}
func TestClockGuardChecksStopStartWithinSameBoot(t *testing.T) {
	guard, initial := testClockGuard(t)
	stopped := testClockSample(initial.WallUTC.Add(time.Minute), 1060, "boot")
	if _, err := guard.Observe(stopped, false); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewClockGuard()
	if err != nil {
		t.Fatal(err)
	}
	changed := testClockSample(stopped.WallUTC.Add(-time.Hour), 1120, "boot")
	event, err := reopened.Observe(changed, true)
	if err != nil || event.Incident == nil {
		t.Fatalf("restart event=%+v err=%v", event, err)
	}
}
func TestClockGuardOrdinarySleepAndDSTStayHealthy(t *testing.T) {
	guard, initial := testClockGuard(t)
	resumed := testClockSample(initial.WallUTC.Add(3*time.Hour), 11800, "boot")
	resumed.UnbiasedTicks = 1001 * clockTicksPerSecond
	resumed.OffsetSeconds = 3600
	event, err := guard.Observe(resumed, false)
	if err != nil || event.Incident != nil || !event.Slept || event.ZoneChanged {
		t.Fatalf("sleep/DST=%+v err=%v", event, err)
	}
}
func TestClockGuardUnknownBootRollbackAndLongGap(t *testing.T) {
	for _, delta := range []time.Duration{-3 * time.Minute, 31 * 24 * time.Hour} {
		t.Run(delta.String(), func(t *testing.T) {
			_, initial := testClockGuard(t)
			guard, err := NewClockGuard()
			if err != nil {
				t.Fatal(err)
			}
			changed := testClockSample(initial.WallUTC.Add(delta), 20, "new-boot")
			event, err := guard.Observe(changed, true)
			if err != nil || event.Incident == nil {
				t.Fatalf("unknown boot=%+v err=%v", event, err)
			}
		})
	}
}
func TestClockGuardSmallCorrectionsAccumulateAgainstAnchor(t *testing.T) {
	guard, initial := testClockGuard(t)
	first := testClockSample(initial.WallUTC.Add(80*time.Second), 1001, "boot")
	if event, err := guard.Observe(first, false); err != nil || event.Incident != nil {
		t.Fatalf("small correction=%+v %v", event, err)
	}
	second := testClockSample(initial.WallUTC.Add(160*time.Second), 1002, "boot")
	if event, err := guard.Observe(second, false); err != nil || event.Incident == nil {
		t.Fatalf("cumulative drift=%+v %v", event, err)
	}
}
func TestClockGuardAcceptPreservesHoldAndReanchorsBackwardTime(t *testing.T) {
	guard, initial := testClockGuard(t)
	changed := testClockSample(initial.WallUTC.Add(-time.Hour), 1001, "boot")
	event, err := guard.Observe(changed, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := guard.Accept(event.Incident.ID, event.Incident.Revision, changed); err != nil {
		t.Fatal(err)
	}
	if storage.TimeGuarded() || !storage.RetentionHeld() {
		t.Fatal("accept should clear incident and retain cleanup hold")
	}
	next := testClockSample(changed.WallUTC.Add(time.Second), 1002, "boot")
	if event, err := guard.Observe(next, false); err != nil || event.Incident != nil {
		t.Fatalf("accepted rollback repeated=%+v %v", event, err)
	}
	if err := guard.SetRetentionHold(false); err != nil {
		t.Fatal(err)
	}
	if storage.RetentionHeld() {
		t.Fatal("explicit cleanup release did not clear hold")
	}
}
func TestClockGuardPendingRestartDoesNotChangeRevisionForNormalTime(t *testing.T) {
	guard, initial := testClockGuard(t)
	changed := testClockSample(initial.WallUTC.Add(time.Hour), 1001, "boot")
	event, err := guard.Observe(changed, false)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := NewClockGuard()
	if err != nil {
		t.Fatal(err)
	}
	now := testClockSample(changed.WallUTC.Add(10*time.Second), 1011, "boot")
	current, err := reopened.Observe(now, true)
	if err != nil || current.Changed || current.Incident.Revision != event.Incident.Revision {
		t.Fatalf("pending restart=%+v %v", current, err)
	}
}
func TestClockGuardRejectsStaleChoiceAndChangedConfirmation(t *testing.T) {
	guard, initial := testClockGuard(t)
	changed := testClockSample(initial.WallUTC.Add(time.Hour), 1001, "boot")
	event, err := guard.Observe(changed, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := guard.Accept("other", event.Incident.Revision, changed); err == nil {
		t.Fatal("accepted stale incident")
	}
	again := testClockSample(changed.WallUTC.Add(time.Hour), 1002, "boot")
	if err := guard.Accept(event.Incident.ID, event.Incident.Revision, again); err == nil {
		t.Fatal("accepted changed clock")
	}
	if got := guard.State().Incident.Revision; got != 2 {
		t.Fatalf("revision=%d want 2", got)
	}
}
func TestClockGuardMalformedCheckpointCreatesWarning(t *testing.T) {
	setupTestDB(t)
	if err := storage.SetConfig("clock_state", "corrupt"); err != nil {
		t.Fatal(err)
	}
	guard, err := NewClockGuard()
	if err != nil {
		t.Fatal(err)
	}
	event, err := guard.Observe(testClockSample(time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC), 1000, "boot"), true)
	if err != nil || event.Incident == nil {
		t.Fatalf("corrupt state=%+v %v", event, err)
	}
}

func TestClockGuardToleranceBoundaryAndNewBootNormalGap(t *testing.T) {
	guard, initial := testClockGuard(t)
	boundary := testClockSample(initial.WallUTC.Add(121*time.Second), 1001, "boot")
	if event, err := guard.Observe(boundary, false); err != nil || event.Incident != nil {
		t.Fatalf("exact tolerance boundary=%+v %v", event, err)
	}
	if err := guard.Persist(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewClockGuard()
	if err != nil {
		t.Fatal(err)
	}
	nextBoot := testClockSample(boundary.WallUTC.Add(24*time.Hour), 10, "new-boot")
	if event, err := reopened.Observe(nextBoot, true); err != nil || event.Incident != nil {
		t.Fatalf("normal new boot=%+v %v", event, err)
	}
}

func TestClockGuardExplicitImplausibleDateDoesNotRepeat(t *testing.T) {
	setupTestDB(t)
	guard, err := NewClockGuard()
	if err != nil {
		t.Fatal(err)
	}
	sample := testClockSample(time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC), 1000, "boot")
	event, err := guard.Observe(sample, true)
	if err != nil || event.Incident == nil {
		t.Fatalf("implausible date=%+v %v", event, err)
	}
	if err := guard.Accept(event.Incident.ID, event.Incident.Revision, sample); err != nil {
		t.Fatal(err)
	}
	sample.WallUTC = sample.WallUTC.Add(time.Second)
	sample.InterruptTicks += clockTicksPerSecond
	sample.UnbiasedTicks += clockTicksPerSecond
	if event, err := guard.Observe(sample, false); err != nil || event.Incident != nil {
		t.Fatalf("accepted date repeated=%+v %v", event, err)
	}
}

func TestClockGuardFaultNeedsExplicitAcceptance(t *testing.T) {
	guard, initial := testClockGuard(t)
	if err := guard.ReportFault(initial.WallUTC, "elapsed clock unavailable"); err != nil {
		t.Fatal(err)
	}
	incident := guard.State().Incident
	if incident == nil || !storage.TimeGuarded() || !storage.RetentionHeld() {
		t.Fatal("sampling fault did not persist a guarded state")
	}
	next := testClockSample(initial.WallUTC.Add(time.Second), 1001, "boot")
	event, err := guard.Observe(next, false)
	if err != nil || event.Incident == nil || event.Incident.ID != incident.ID {
		t.Fatalf("healthy observation cleared the fault: %+v %v", event, err)
	}
	if err := guard.ReportFault(initial.WallUTC, "another sampling failure"); err != nil {
		t.Fatal(err)
	}
	if got := guard.State().Incident; got.ID != incident.ID || got.Revision != incident.Revision {
		t.Fatal("repeated sampling fault invalidated the pending choice")
	}
	if err := guard.Accept(incident.ID, incident.Revision, next); err != nil {
		t.Fatal(err)
	}
	if storage.TimeGuarded() {
		t.Fatal("explicit acceptance did not clear fault")
	}
}

func TestClockGuardFailedIncidentSaveRetainsMemoryGuard(t *testing.T) {
	guard, initial := testClockGuard(t)
	writeErr := errors.New("checkpoint write unavailable")
	guard.saveState = func(storage.ClockState) error { return writeErr }
	changed := testClockSample(initial.WallUTC.Add(time.Hour), 1001, "boot")
	if _, err := guard.Observe(changed, false); !errors.Is(err, writeErr) {
		t.Fatalf("write failure=%v", err)
	}
	incident := guard.State().Incident
	if incident == nil || !incident.LastSafe.Equal(initial.WallUTC) || !incident.Current.WallUTC.Equal(changed.WallUTC) {
		t.Fatalf("failed save lost detected incident: %+v", incident)
	}
	if err := guard.ReportFault(initial.WallUTC, writeErr.Error()); !errors.Is(err, writeErr) {
		t.Fatalf("fault write failure=%v", err)
	}
	if got := guard.State().Incident; got.ID != incident.ID || got.Revision != incident.Revision {
		t.Fatal("fault fallback lost the original incident")
	}
	guard.saveState = nil
	next := testClockSample(changed.WallUTC.Add(time.Second), 1002, "boot")
	if event, err := guard.Observe(next, false); err != nil || event.Incident == nil {
		t.Fatalf("write recovery silently accepted time: %+v %v", event, err)
	}
	if got := guard.State().Incident; got.Revision != incident.Revision {
		t.Fatal("retried incident write was mistaken for another time change")
	}
	if !storage.TimeGuarded() {
		t.Fatal("recovered storage did not persist the pending incident")
	}
}

func TestClockGuardFaultSaveFailureRetainsMemoryGuard(t *testing.T) {
	guard, initial := testClockGuard(t)
	guard.saveState = func(storage.ClockState) error { return errors.New("disk unavailable") }
	if err := guard.ReportFault(initial.WallUTC, "clock unavailable"); err == nil {
		t.Fatal("fault save failure was not reported")
	}
	if guard.State().Incident == nil {
		t.Fatal("fault save failure left the daemon unguarded")
	}
	guard.saveState = nil
	next := testClockSample(initial.WallUTC.Add(time.Second), 1001, "boot")
	if event, err := guard.Observe(next, false); err != nil || event.Incident == nil {
		t.Fatalf("recovered storage cleared incident: %+v %v", event, err)
	}
	if !storage.TimeGuarded() {
		t.Fatal("recovered storage did not retry the pending fault write")
	}
}
