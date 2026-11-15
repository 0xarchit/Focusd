package coreapi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
)

const IPCAddress = "127.0.0.1:48321"

func SendIPCCmd(cmd string) bool {
	response, err := Call(Request{Command: cmd})
	return err == nil && response.OK
}

// Call authenticates to the current profile owner without a fixed-port fallback.
func Call(request Request) (Response, error) {
	var response Response
	endpoint, err := ReadEndpoint()
	if err != nil {
		return response, err
	}
	token, err := loadIPCToken()
	if err != nil {
		return response, err
	}
	if request.Generation != "" && request.Generation != endpoint.Generation {
		return response, fmt.Errorf("daemon generation changed")
	}
	if request.Version != 0 && request.Version != ProtocolVersion {
		return response, fmt.Errorf("unsupported IPC protocol version %d", request.Version)
	}
	request.Version, request.Token, request.Generation = ProtocolVersion, token, endpoint.Generation
	if request.RequestID == "" {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return response, err
		}
		request.RequestID = hex.EncodeToString(random[:])
	}
	if err := ValidateRequest(request); err != nil {
		return response, err
	}
	conn, err := net.DialTimeout("tcp", endpoint.Address, 5*time.Second)
	if err != nil {
		return response, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return response, err
	}

	if err := writeEnvelope(conn, request); err != nil {
		return response, err
	}
	if err := readEnvelope(conn, &response); err != nil {
		return response, err
	}
	if response.Version != ProtocolVersion || response.Generation != request.Generation || response.RequestID != request.RequestID {
		return response, fmt.Errorf("daemon response identity does not match request")
	}
	if !response.OK {
		return response, &RPCError{Response: response}
	}
	return response, nil
}

// IPCCommandNeedsToken reports whether a command mutates daemon state and must
// therefore be authenticated.
func IPCCommandNeedsToken(cmd string) bool {
	switch strings.TrimSpace(cmd) {
	case "stop", "flush":
		return true
	}
	return false
}

// encodeIPCCmd builds the wire payload for a command.
func encodeIPCCmd(cmd, token string) string {
	if !IPCCommandNeedsToken(cmd) {
		return cmd
	}
	return cmd + " " + token
}

// ParseIPCCommand splits a wire payload into its command and optional token.
func ParseIPCCommand(raw string) (cmd string, token string, err error) {
	line := strings.TrimSpace(raw)
	if line == "" {
		return "", "", fmt.Errorf("empty command")
	}

	parts := strings.Fields(line)
	cmd = parts[0]
	if len(parts) > 1 {
		token = parts[1]
	}
	if len(parts) > 2 {
		return "", "", fmt.Errorf("unexpected trailing data")
	}

	switch cmd {
	case "stop", "flush":
		if token == "" {
			return "", "", fmt.Errorf("command %q requires a token", cmd)
		}
	default:
		if token != "" {
			return "", "", fmt.Errorf("command %q does not take a token", cmd)
		}
	}
	return cmd, token, nil
}

// TokensMatch compares two tokens without leaking length or content through
// timing.
func TokensMatch(got, want string) bool {
	if got == "" || want == "" {
		return false
	}
	if len(got) != len(want) {
		return false
	}
	var diff byte
	for i := 0; i < len(got); i++ {
		diff |= got[i] ^ want[i]
	}
	return diff == 0
}
