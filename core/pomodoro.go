package core

import (
	"fmt"
	"focusd/coreapi"
	"focusd/storage"
	"focusd/system"
	"time"
)

// pomodoroNotify is the toast channel for timer completion; tests swap it.
var pomodoroNotify = func(title, msg string) bool {
	return showNotification(notifyContext, title, msg)
}

// CheckPomodoroCompletion finishes an expired pomodoro and notifies through
// the toast channel. The daemon calls it on every tick. The CLI (status read)
// and the TUI (periodic refresh) call it only while the daemon is stopped, so
// a timer still completes without waiting forever and the alert goes out once.
func CheckPomodoroCompletion() {
	profileLock, err := system.AcquireProfileLock()
	if err != nil {
		return
	}
	defer profileLock.Close()
	guard, err := NewClockGuard()
	if err != nil {
		return
	}
	sample, err := system.ReadClockSample()
	if err != nil {
		_ = guard.ReportFault(time.Time{}, err.Error())
		return
	}
	event, err := guard.Observe(sample, true)
	if err != nil || event.Incident != nil {
		if err != nil {
			_ = guard.ReportFault(event.LastSafe, err.Error())
		}
		if accepted := guard.State().Accepted; accepted != nil {
			_ = coreapi.FreezePomodoroAtSample(system.ClockSample{WallUTC: accepted.WallUTC, InterruptTicks: accepted.InterruptTicks, UnbiasedTicks: accepted.UnbiasedTicks, BootID: accepted.BootID})
		}
		return
	}
	if storage.TimeGuarded() {
		return
	}
	coreapi.CheckPomodoroAndNotify(pomodoroNotify)
}

// ChangePomodoro routes active-daemon writes through its clock controller and
// owns the profile while changing a timer with the daemon stopped.
func ChangePomodoro(start bool, minutes int) error {
	command := "timer-stop"
	if start {
		command = "timer-start"
	}
	if _, err := coreapi.ReadEndpoint(); err == nil {
		response, err := coreapi.Call(coreapi.Request{Command: command, DurationMinutes: minutes})
		if err != nil {
			return err
		}
		if !response.OK {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	}
	profileLock, err := system.AcquireProfileLock()
	if err != nil {
		return fmt.Errorf("timer writer is busy: %w", err)
	}
	defer profileLock.Close()
	guard, err := NewClockGuard()
	if err != nil {
		return err
	}
	sample, err := system.ReadClockSample()
	if err != nil {
		_ = guard.ReportFault(time.Time{}, err.Error())
		return err
	}
	event, err := guard.Observe(sample, true)
	if err != nil {
		_ = guard.ReportFault(event.LastSafe, err.Error())
		return err
	}
	if start && event.Incident != nil {
		return fmt.Errorf("resolve the clock warning before starting a timer")
	}
	if start {
		return coreapi.StartPomodoro(minutes)
	}
	return coreapi.StopPomodoro()
}
