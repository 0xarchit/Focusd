package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"focusd/coreapi"
	"focusd/system"
)

const installerShutdownTimeout = 10 * time.Second

type shutdownControl struct {
	endpoint func() (coreapi.Endpoint, error)
	call     func(coreapi.Request) (coreapi.Response, error)
	count    func(string, string) (int, error)
	now      func() time.Time
	wait     func(time.Duration)
}

func runShutdownClients(args []string) {
	if len(args) != 4 || args[2] != "--install-dir" {
		fmt.Fprintln(os.Stderr, "Usage: focusd shutdown-clients --install-dir <absolute directory>")
		os.Exit(1)
	}
	control := shutdownControl{coreapi.ReadEndpoint, coreapi.Call, system.CountInstancesInDirChecked, time.Now, time.Sleep}
	if err := control.prepare(args[3]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// prepare waits for terminal restoration before stopping the authenticated
// daemon. A legacy installation must be closed manually; it is never killed.
func (control shutdownControl) prepare(directory string) error {
	if !filepath.IsAbs(directory) || strings.TrimSpace(directory) != directory || strings.ContainsAny(directory, "\r\n\x00") {
		return fmt.Errorf("installation directory must be an absolute path")
	}
	directory = filepath.Clean(directory)
	clients, err := control.count("focusd.exe", directory)
	if err != nil {
		return fmt.Errorf("verify Focusd terminals: %w", err)
	}
	daemons, err := control.count(system.DaemonProcessName, directory)
	if err != nil {
		return fmt.Errorf("verify Focusd daemon: %w", err)
	}
	if clients == 0 && daemons == 0 {
		return nil
	}
	endpoint, err := control.endpoint()
	if err != nil {
		return fmt.Errorf("close Focusd terminals and stop the old daemon manually, then retry; authenticated shutdown unavailable: %w", err)
	}
	if !strings.EqualFold(filepath.Clean(filepath.Dir(endpoint.ExePath)), directory) {
		return fmt.Errorf("the running profile daemon belongs to another directory; close Focusd in %s manually, then retry", directory)
	}
	if _, err := control.request(endpoint, "shutdown-clients"); err != nil {
		return fmt.Errorf("request Focusd terminal cleanup: %w", err)
	}
	deadline := control.now().Add(installerShutdownTimeout)
	for {
		response, err := control.request(endpoint, "status")
		if err != nil {
			return fmt.Errorf("verify terminal cleanup: %w", err)
		}
		var status struct {
			ActiveClients *int `json:"active_clients"`
		}
		if err := json.Unmarshal(response.Data, &status); err != nil || status.ActiveClients == nil || *status.ActiveClients < 0 {
			return fmt.Errorf("daemon did not provide valid terminal cleanup status")
		}
		clients, err := control.count("focusd.exe", directory)
		if err != nil {
			return fmt.Errorf("verify terminal process exit: %w", err)
		}
		if *status.ActiveClients == 0 && clients == 0 {
			break
		}
		if !control.now().Before(deadline) {
			return fmt.Errorf("a Focusd terminal has not exited; close it manually and retry")
		}
		control.wait(100 * time.Millisecond)
	}
	if _, err := control.request(endpoint, "stop"); err != nil {
		return fmt.Errorf("stop Focusd daemon: %w", err)
	}
	deadline = control.now().Add(installerShutdownTimeout)
	for {
		daemons, err := control.count(system.DaemonProcessName, directory)
		if err != nil {
			return fmt.Errorf("verify daemon process exit: %w", err)
		}
		clients, err := control.count("focusd.exe", directory)
		if err != nil {
			return fmt.Errorf("verify terminal process exit: %w", err)
		}
		if daemons == 0 && clients == 0 {
			return nil
		}
		if !control.now().Before(deadline) {
			return fmt.Errorf("Focusd processes have not exited; installation cancelled")
		}
		control.wait(100 * time.Millisecond)
	}
}

func (control shutdownControl) request(endpoint coreapi.Endpoint, command string) (coreapi.Response, error) {
	response, err := control.call(coreapi.Request{Command: command, Generation: endpoint.Generation})
	if err != nil {
		return response, err
	}
	if !response.OK || response.Version != coreapi.ProtocolVersion || response.Generation != endpoint.Generation {
		return response, fmt.Errorf("daemon shutdown response identity does not match")
	}
	return response, nil
}
