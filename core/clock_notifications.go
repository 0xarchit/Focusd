package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"focusd/storage"
)

const clockToastXML = `<toast><visual><binding template="ToastGeneric"><text id="1"></text><text id="2"></text></binding></visual><actions><action content="Continue — keep history" arguments="continue" activationType="background"/><action content="Reset history" arguments="reset" activationType="background"/><action content="Turn off" arguments="off" activationType="background"/></actions></toast>`
const clockResetToastXML = `<toast><visual><binding template="ToastGeneric"><text id="1"></text><text id="2"></text></binding></visual><actions><action content="Back up and reset" arguments="reset-confirmed" activationType="background"/><action content="Cancel" arguments="cancel" activationType="background"/></actions></toast>`

// ShowClockRecoveryNotification never treats disappearance as a decision.
func ShowClockRecoveryNotification(ctx context.Context, state storage.ClockState, callback func(string, bool)) bool {
	if state.Incident == nil {
		return false
	}
	ensureToastAppID()
	incident := *state.Incident
	last := incident.LastSafe.Format(time.RFC3339)
	current := incident.Current.WallUTC.Format(time.RFC3339)
	message := fmt.Sprintf("Tracking paused. Last accepted: %s. System time: %s. Check Windows date/time, then choose.", last, current)
	workerCtx, cancel := context.WithTimeout(ctx, 125*time.Second)
	process := startToast(workerCtx, actionToastScript, toastEnv("Date/time changed unexpectedly", message, clockToastXML))
	if !process.started {
		cancel()
		return false
	}
	go func() {
		choice := strings.TrimSpace(string(process.wait(workerCtx)))
		cancel()
		if ctx.Err() != nil || callback == nil {
			return
		}
		switch choice {
		case "continue", "off":
			callback(choice, false)
		case "reset":
			confirmCtx, confirmCancel := context.WithTimeout(ctx, 125*time.Second)
			defer confirmCancel()
			confirmation := startToast(confirmCtx, actionToastScript, toastEnv("Reset tracking history?", "A verified backup will retain old history. Limits and settings stay. Confirm to clear usage and continue.", clockResetToastXML))
			if !confirmation.started {
				return
			}
			choice = strings.TrimSpace(string(confirmation.wait(confirmCtx)))
			if ctx.Err() == nil && choice == "reset-confirmed" {
				callback("reset", true)
			}
		}
	}()
	return true
}
