package coreapi

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	ProtocolVersion = 1
	MaxIPCBytes     = 4 * 1024
)

// Request identifies the daemon and decision revision that may accept an action.
type Request struct {
	Version         int    `json:"version"`
	Command         string `json:"command"`
	Token           string `json:"token"`
	Generation      string `json:"generation"`
	RequestID       string `json:"request_id"`
	IncidentID      string `json:"incident_id,omitempty"`
	Revision        int64  `json:"revision,omitempty"`
	Action          string `json:"action,omitempty"`
	Confirm         bool   `json:"confirm,omitempty"`
	ConfirmToken    string `json:"confirm_token,omitempty"`
	ClientID        string `json:"client_id,omitempty"`
	LeaseSeconds    int    `json:"lease_seconds,omitempty"`
	BootID          string `json:"boot_id,omitempty"`
	PID             int    `json:"pid,omitempty"`
	InterruptTicks  uint64 `json:"interrupt_ticks,omitempty"`
	OperationID     string `json:"operation_id,omitempty"`
	DurationMinutes int    `json:"duration_minutes,omitempty"`
}

// Response acknowledges one request; Data holds a command-specific result.
type Response struct {
	Version     int             `json:"version"`
	OK          bool            `json:"ok"`
	Generation  string          `json:"generation"`
	RequestID   string          `json:"request_id"`
	State       string          `json:"state,omitempty"`
	Error       string          `json:"error,omitempty"`
	OperationID string          `json:"operation_id,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
}

type RPCError struct {
	Response Response
}

func (e *RPCError) Error() string {
	if e.Response.Error != "" {
		return e.Response.Error
	}
	return "daemon rejected the request"
}

// ReadRequest accepts one bounded JSON envelope. The caller owns connection deadlines.
func ReadRequest(reader io.Reader) (Request, error) {
	var request Request
	if err := readEnvelope(reader, &request); err != nil {
		return request, err
	}
	if err := ValidateRequest(request); err != nil {
		return Request{}, err
	}
	return request, nil
}

// ValidateRequest checks the transport identity and keeps unsupported commands explicit.
func ValidateRequest(request Request) error {
	if request.Version != ProtocolVersion {
		return fmt.Errorf("unsupported IPC protocol version %d", request.Version)
	}
	if request.Token == "" || request.Generation == "" || request.RequestID == "" {
		return fmt.Errorf("IPC identity is incomplete")
	}
	for _, value := range []string{request.Generation, request.RequestID, request.IncidentID, request.ClientID, request.OperationID} {
		if len(value) > 128 || strings.ContainsAny(value, "\r\n\x00") {
			return fmt.Errorf("invalid IPC identifier")
		}
	}
	if len(request.Token) > 128 || request.Revision < 0 || request.LeaseSeconds < 0 || request.LeaseSeconds > 30 || request.PID < 0 {
		return fmt.Errorf("invalid IPC request values")
	}
	switch request.Command {
	case "timer-start":
		if request.DurationMinutes < 1 || request.DurationMinutes > MaxPomodoroMinutes {
			return fmt.Errorf("timer duration must be between 1 and %d minutes", MaxPomodoroMinutes)
		}
		return nil
	case "timer-stop":
		return nil
	case "stop", "flush", "status", "clock-status", "tui-register", "tui-renew", "tui-release", "clock-resolve", "reset", "operation-status", "retention-preview", "retention-confirm", "shutdown-tui", "shutdown-clients":
		return nil
	default:
		return fmt.Errorf("unsupported IPC command %q", request.Command)
	}
}

// WriteResponse never emits an unbounded payload; commands paginate or use operation IDs.
func WriteResponse(writer io.Writer, response Response) error {
	response.Version = ProtocolVersion
	return writeEnvelope(writer, response)
}

func readEnvelope(reader io.Reader, value any) error {
	line, err := bufio.NewReader(io.LimitReader(reader, MaxIPCBytes+1)).ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("read IPC envelope: %w", err)
	}
	if len(line) > MaxIPCBytes {
		return fmt.Errorf("IPC envelope exceeds %d bytes", MaxIPCBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("decode IPC envelope: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("IPC envelope contains trailing data")
	}
	return nil
}

func writeEnvelope(writer io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > MaxIPCBytes {
		return fmt.Errorf("IPC envelope exceeds %d bytes", MaxIPCBytes)
	}
	n, err := writer.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}
