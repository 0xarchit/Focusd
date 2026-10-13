package coreapi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IPC listens on a fixed loopback port, so a per-install token file keeps it
// closed to other local processes. The 0600 mode applies off Windows; on Windows
// the file inherits the %APPDATA% ACL.

const ipcTokenFile = "ipc.key"

const ipcTokenBytes = 32

func ipcTokenPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA environment variable not set")
	}
	return filepath.Join(appData, "focusd", ipcTokenFile), nil
}

// EnsureIPCToken returns the existing token, creating one if it is missing.
// Created owner-only because anyone who can read it can drive the daemon.
func EnsureIPCToken() (string, error) {
	path, err := ipcTokenPath()
	if err != nil {
		return "", err
	}

	if token, err := readIPCToken(path); err == nil {
		return token, nil
	}

	buf := make([]byte, ipcTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate IPC token: %w", err)
	}
	token := hex.EncodeToString(buf)

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(token+"\n"), 0600); err != nil {
		return "", fmt.Errorf("failed to write IPC token: %w", err)
	}
	return token, nil
}

func readIPCToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("IPC token file is empty")
	}
	return token, nil
}

// loadIPCToken returns the token for a client. It does not create one, so a
// client talking to a daemon that was never started fails cleanly.
func loadIPCToken() (string, error) {
	path, err := ipcTokenPath()
	if err != nil {
		return "", err
	}
	return readIPCToken(path)
}
