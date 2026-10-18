package coreapi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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

	// Create exclusively: two daemons starting at once must not overwrite each
	// other's token, because clients read the file while the live daemon keeps
	// expecting the token it generated. The loser adopts the winner's token.
	err = createIPCToken(path, token)
	if err == nil {
		return token, nil
	}
	if !os.IsExist(err) {
		return "", fmt.Errorf("failed to write IPC token: %w", err)
	}
	for attempt := 0; attempt < 10; attempt++ {
		if existing, readErr := readIPCToken(path); readErr == nil {
			return existing, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return "", fmt.Errorf("failed to read the existing IPC token: %w", err)
}

// createIPCToken atomically creates the token file, failing if it already
// exists so a racing daemon's token is never replaced. A failed write removes
// the file rather than leaving a partial token behind.
func createIPCToken(path, token string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(token + "\n"); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return err
	}
	return nil
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
