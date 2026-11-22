package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const clockStateKey = "clock_state"
const clockStateVersion = 1

var ErrMalformedClockState = errors.New("invalid persisted clock state")

type ClockCheckpoint struct {
	WallUTC        time.Time `json:"wall_utc"`
	InterruptTicks uint64    `json:"interrupt_ticks"`
	UnbiasedTicks  uint64    `json:"unbiased_ticks"`
	BootID         string    `json:"boot_id,omitempty"`
	ZoneKey        string    `json:"zone_key,omitempty"`
	OffsetSeconds  int       `json:"offset_seconds"`
}
type ClockIncident struct {
	ID         string          `json:"id"`
	Revision   int64           `json:"revision"`
	Reason     string          `json:"reason"`
	LastSafe   time.Time       `json:"last_safe"`
	Current    ClockCheckpoint `json:"current"`
	DetectedAt time.Time       `json:"detected_at"`
}
type ClockState struct {
	Version          int              `json:"version"`
	Accepted         *ClockCheckpoint `json:"accepted,omitempty"`
	Anchor           *ClockCheckpoint `json:"anchor,omitempty"`
	Incident         *ClockIncident   `json:"incident,omitempty"`
	RetentionHold    bool             `json:"retention_hold"`
	AcceptedExplicit bool             `json:"accepted_explicit,omitempty"`
	Epoch            uint64           `json:"epoch"`
}

// GetClockState distinguishes a legacy missing checkpoint from unreadable established state.
func GetClockState() (ClockState, error) {
	value, err := GetConfig(clockStateKey)
	if errors.Is(err, sql.ErrNoRows) {
		return ClockState{Version: clockStateVersion}, nil
	}
	if err != nil {
		return ClockState{}, err
	}
	var state ClockState
	if len(value) > 32*1024 || json.Unmarshal([]byte(value), &state) != nil || state.Version != clockStateVersion {
		return ClockState{}, ErrMalformedClockState
	}
	if state.Accepted != nil && state.Accepted.WallUTC.IsZero() {
		return ClockState{}, ErrMalformedClockState
	}
	if state.Incident != nil && (state.Incident.ID == "" || state.Incident.Revision < 1 || state.Incident.Current.WallUTC.IsZero()) {
		return ClockState{}, ErrMalformedClockState
	}
	return state, nil
}
func SaveClockState(state ClockState) error {
	state.Version = clockStateVersion
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode clock state: %w", err)
	}
	return SetConfig(clockStateKey, string(data))
}

// TimeGuarded fails closed when established clock state cannot be read.
func TimeGuarded() bool {
	state, err := GetClockState()
	return err != nil || state.Incident != nil
}
func RetentionHeld() bool {
	state, err := GetClockState()
	return err != nil || state.Incident != nil || state.RetentionHold
}
