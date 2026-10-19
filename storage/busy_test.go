package storage

import (
	"errors"
	"testing"
	"time"
)

func TestIsBusyError(t *testing.T) {
	busy := []error{
		errors.New("SQLITE_BUSY: database is locked"),
		errors.New("database table is locked"),
		errors.New("database is locked (5) (SQLITE_BUSY)"),
	}
	for _, err := range busy {
		if !isBusyError(err) {
			t.Errorf("expected %v to be recognised as busy", err)
		}
	}

	notBusy := []error{
		nil,
		errors.New("no such table: apps_daily"),
		errors.New("constraint failed: UNIQUE"),
		errors.New("connection refused"),
	}
	for _, err := range notBusy {
		if isBusyError(err) {
			t.Errorf("expected %v not to be recognised as busy", err)
		}
	}
}

// TestWithBusyRetrySucceedsAfterRetry covers the case that matters: a transient
// lock clears and the read completes.
func TestWithBusyRetrySucceedsAfterRetry(t *testing.T) {
	attempts := 0
	err := withBusyRetry(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("database is locked")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected success after retrying, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d; want 3", attempts)
	}
}

// TestWithBusyRetryDoesNotRetryOtherErrors ensures a real failure is reported
// immediately rather than being hidden behind the retry budget.
func TestWithBusyRetryDoesNotRetryOtherErrors(t *testing.T) {
	attempts := 0
	sentinel := errors.New("no such table: apps_daily")

	err := withBusyRetry(func() error {
		attempts++
		return sentinel
	})

	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v; want the original error", err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d; a non-busy error should not be retried", attempts)
	}
}

func TestWithBusyRetryGivesUp(t *testing.T) {
	attempts := 0
	err := withBusyRetry(func() error {
		attempts++
		return errors.New("database is locked")
	})

	if err == nil {
		t.Error("expected an error after exhausting the retry budget")
	}
	if attempts != busyRetryAttempts {
		t.Errorf("attempts = %d; want %d", attempts, busyRetryAttempts)
	}
}

// TestWithBusyRetryImmediateSuccess checks the happy path does not sleep.
func TestWithBusyRetryImmediateSuccess(t *testing.T) {
	start := time.Now()
	if err := withBusyRetry(func() error { return nil }); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > busyRetryDelay {
		t.Errorf("a successful call waited %s; it should return immediately", elapsed)
	}
}

// TestBusyRetryBudgetIsBounded guards the UI from being blocked for long: the
// whole budget must stay well under a frame of human patience.
func TestBusyRetryBudgetIsBounded(t *testing.T) {
	budget := time.Duration(0)
	for attempt := 0; attempt < busyRetryAttempts; attempt++ {
		if attempt > 0 {
			budget += time.Duration(attempt) * busyRetryDelay
		}
	}
	if budget > 500*time.Millisecond {
		t.Errorf("retry budget is %s; too long to block the UI", budget)
	}
}
