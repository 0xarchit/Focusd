package coreapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"focusd/system"
)

const endpointFile = "ipc.endpoint.json"

// Endpoint binds an ephemeral loopback listener to one profile owner.
type Endpoint struct {
	Version    int    `json:"version"`
	Address    string `json:"address"`
	Generation string `json:"generation"`
	PID        int    `json:"pid"`
	ExePath    string `json:"exe_path"`
}

func endpointPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA environment variable not set")
	}
	return filepath.Join(appData, "focusd", endpointFile), nil
}

func validateEndpoint(endpoint Endpoint) error {
	if endpoint.Version != ProtocolVersion {
		return fmt.Errorf("unsupported daemon protocol version %d", endpoint.Version)
	}
	host, port, err := net.SplitHostPort(endpoint.Address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return fmt.Errorf("daemon endpoint must be a literal loopback address")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return fmt.Errorf("invalid daemon endpoint port")
	}
	if endpoint.Generation == "" || len(endpoint.Generation) > 128 || strings.ContainsAny(endpoint.Generation, "\r\n\x00") || endpoint.PID < 1 {
		return fmt.Errorf("invalid daemon endpoint identity")
	}
	if !filepath.IsAbs(endpoint.ExePath) || strings.ContainsAny(endpoint.ExePath, "\r\n\x00") || !strings.EqualFold(filepath.Base(endpoint.ExePath), system.DaemonProcessName) {
		return fmt.Errorf("invalid daemon executable path")
	}
	return nil
}

// PublishEndpoint is called while the daemon holds the profile writer lock.
func PublishEndpoint(endpoint Endpoint) error {
	if err := validateEndpoint(endpoint); err != nil {
		return err
	}
	path, err := endpointPath()
	if err != nil {
		return err
	}
	return system.WritePrivateFileAtomic(path, func(writer io.Writer) error { return json.NewEncoder(writer).Encode(endpoint) })
}

// ReadEndpoint validates discovery before any connection can leave loopback.
func ReadEndpoint() (Endpoint, error) {
	var endpoint Endpoint
	path, err := endpointPath()
	if err != nil {
		return endpoint, err
	}
	file, err := os.Open(path)
	if err != nil {
		return endpoint, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxIPCBytes+1))
	if err != nil {
		return endpoint, err
	}
	if len(data) > MaxIPCBytes {
		return endpoint, fmt.Errorf("daemon endpoint exceeds size limit")
	}
	if err := json.Unmarshal(data, &endpoint); err != nil {
		return endpoint, fmt.Errorf("invalid daemon endpoint: %w", err)
	}
	return endpoint, validateEndpoint(endpoint)
}

// RemoveEndpoint cannot remove discovery published by a newer daemon generation.
func RemoveEndpoint(generation string) error {
	endpoint, err := ReadEndpoint()
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if endpoint.Generation != generation {
		return fmt.Errorf("daemon endpoint belongs to another generation")
	}
	path, err := endpointPath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}
