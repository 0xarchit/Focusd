package cli

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"focusd/coreapi"
	"focusd/system"
)

func fakeShutdown(t *testing.T) (shutdownControl, *[]string, *int, *int) {
	t.Helper()
	commands := []string{}
	clients, daemons := 1, 1
	clock := time.Unix(0, 0)
	control := shutdownControl{
		endpoint: func() (coreapi.Endpoint, error) {
			return coreapi.Endpoint{Generation: "daemon-1", ExePath: `C:\Focusd\focusd_daemon.exe`}, nil
		},
		call: func(request coreapi.Request) (coreapi.Response, error) {
			if request.Generation != "daemon-1" {
				t.Fatalf("unbound shutdown request: %#v", request)
			}
			commands = append(commands, request.Command)
			if request.Command == "shutdown-clients" {
				clients = 0
			}
			if request.Command == "stop" {
				daemons = 0
			}
			return coreapi.Response{Version: coreapi.ProtocolVersion, Generation: "daemon-1", OK: true, Data: []byte(`{"active_clients":0}`)}, nil
		},
		count: func(name, directory string) (int, error) {
			if directory != `C:\Focusd` {
				t.Fatalf("unexpected process scope %q", directory)
			}
			if name == system.DaemonProcessName {
				return daemons, nil
			}
			return clients, nil
		},
		now:  func() time.Time { return clock },
		wait: func(delay time.Duration) { clock = clock.Add(delay) },
	}
	return control, &commands, &clients, &daemons
}

func TestInstallerShutdownRestoresTerminalsBeforeDaemonStop(t *testing.T) {
	control, commands, _, _ := fakeShutdown(t)
	if err := control.prepare(`C:\Focusd`); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*commands, []string{"shutdown-clients", "status", "stop"}) {
		t.Fatalf("shutdown sequence %v", *commands)
	}
}

func TestInstallerShutdownWaitsForActualTerminalProcessExit(t *testing.T) {
	control, commands, clients, _ := fakeShutdown(t)
	call := control.call
	control.call = func(request coreapi.Request) (coreapi.Response, error) {
		response, err := call(request)
		*clients = 1
		return response, err
	}
	if err := control.prepare(`C:\Focusd`); err == nil || !strings.Contains(err.Error(), "terminal has not exited") {
		t.Fatalf("lingering terminal error %v", err)
	}
	for _, command := range *commands {
		if command == "stop" {
			t.Fatal("daemon stopped before terminal restored")
		}
	}
}

func TestInstallerShutdownFailsClosedForLegacyAndUnrelatedDaemon(t *testing.T) {
	for _, scenario := range []string{"legacy", "different directory"} {
		t.Run(scenario, func(t *testing.T) {
			control, commands, _, _ := fakeShutdown(t)
			control.endpoint = func() (coreapi.Endpoint, error) {
				if scenario == "legacy" {
					return coreapi.Endpoint{}, os.ErrNotExist
				}
				return coreapi.Endpoint{Generation: "other", ExePath: `C:\Other\focusd_daemon.exe`}, nil
			}
			if err := control.prepare(`C:\Focusd`); err == nil || !strings.Contains(err.Error(), "manually") {
				t.Fatalf("unsafe owner error %v", err)
			}
			if len(*commands) > 0 {
				t.Fatalf("sent command to unverified daemon %v", *commands)
			}
		})
	}
}

func TestInstallerShutdownAllowsAbsentProcessesWithoutEndpoint(t *testing.T) {
	control, commands, clients, daemons := fakeShutdown(t)
	*clients, *daemons = 0, 0
	control.endpoint = func() (coreapi.Endpoint, error) {
		t.Fatal("unnecessary endpoint lookup with no installed processes")
		return coreapi.Endpoint{}, os.ErrNotExist
	}
	if err := control.prepare(`C:\Focusd`); err != nil || len(*commands) != 0 {
		t.Fatalf("empty installation shutdown %v, %v", err, *commands)
	}
}

func TestInstallerShutdownRejectsUnknownStateAndChangedGeneration(t *testing.T) {
	for _, scenario := range []string{"process enumeration", "unknown clients", "changed generation", "RPC failure"} {
		t.Run(scenario, func(t *testing.T) {
			control, commands, _, _ := fakeShutdown(t)
			switch scenario {
			case "process enumeration":
				control.count = func(string, string) (int, error) { return 0, errors.New("snapshot failed") }
			default:
				call := control.call
				control.call = func(request coreapi.Request) (coreapi.Response, error) {
					response, err := call(request)
					if request.Command == "status" {
						switch scenario {
						case "unknown clients":
							response.Data = []byte(`{}`)
						case "changed generation":
							response.Generation = "daemon-2"
						case "RPC failure":
							err = errors.New("connection unavailable")
						}
					}
					return response, err
				}
			}
			if err := control.prepare(`C:\Focusd`); err == nil {
				t.Fatal("unknown shutdown state accepted")
			}
			for _, command := range *commands {
				if command == "stop" {
					t.Fatal("stopped daemon despite failed client verification")
				}
			}
		})
	}
}

func TestInstallerShutdownDaemonExitTimeout(t *testing.T) {
	control, _, _, daemons := fakeShutdown(t)
	call := control.call
	control.call = func(request coreapi.Request) (coreapi.Response, error) {
		response, err := call(request)
		*daemons = 1
		return response, err
	}
	if err := control.prepare(`C:\Focusd`); err == nil || !strings.Contains(err.Error(), "installation cancelled") {
		t.Fatalf("daemon timeout error %v", err)
	}
}

func TestInstallerShutdownRejectsRelativeDirectory(t *testing.T) {
	control, _, _, _ := fakeShutdown(t)
	if err := control.prepare(`Focusd`); err == nil {
		t.Fatal("relative installation directory accepted")
	}
}
