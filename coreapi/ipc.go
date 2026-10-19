package coreapi

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

const IPCAddress = "127.0.0.1:48321"

func SendIPCCmd(cmd string) bool {
	token, err := loadIPCToken()
	if err != nil {
		// No token means the daemon was never started from this install, or
		// the token was removed. Either way there is nothing to talk to.
		return false
	}

	conn, err := net.DialTimeout("tcp", IPCAddress, 5*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return false
	}

	// Commands that change state carry the token. Read-only status requests do
	// not, so they stay usable before the first daemon start.
	payload := cmd
	if IPCCommandNeedsToken(cmd) {
		payload = cmd + " " + token
	}

	if _, err := conn.Write([]byte(payload + "\n")); err != nil {
		return false
	}

	response, err := bufio.NewReader(conn).ReadString('\n')
	return err == nil && strings.TrimSpace(response) == "ok"
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
