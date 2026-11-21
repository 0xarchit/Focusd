package cli

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"focusd/coreapi"
	"focusd/system"
)

type updateTransport func(*http.Request) (*http.Response, error)

func (transport updateTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

type fakeUpdateStage struct {
	events    *[]string
	launchErr error
}

func (stage *fakeUpdateStage) Launch() error {
	*stage.events = append(*stage.events, "launch")
	return stage.launchErr
}

func (stage *fakeUpdateStage) Close() error {
	*stage.events = append(*stage.events, "close")
	return nil
}

func stubUpdateDownload(t *testing.T) {
	t.Helper()
	originalMeta, originalDownload := metaClient, downloadClient
	t.Cleanup(func() { metaClient, downloadClient = originalMeta, originalDownload })
	metaClient = &http.Client{Transport: updateTransport(func(*http.Request) (*http.Response, error) {
		body := strings.Repeat("a", 64) + "  focusd_setup.exe\n"
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	downloadClient = &http.Client{Transport: updateTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("installer"))}, nil
	})}
}

func TestUpdateVerifiesBeforeStoppingAnyProcess(t *testing.T) {
	stubUpdateDownload(t)
	originalStage, originalPrepare := stageInstaller, prepareForUpdate
	t.Cleanup(func() { stageInstaller, prepareForUpdate = originalStage, originalPrepare })
	var events []string
	stageInstaller = func(reader io.Reader, expected string, limit int64) (updateStage, error) {
		if expected != strings.Repeat("a", 64) || limit != maxInstallerBytes {
			t.Fatal("download verification contract changed")
		}
		events = append(events, "verify")
		return &fakeUpdateStage{events: &events}, nil
	}
	prepareForUpdate = func() (func() error, error) {
		events = append(events, "prepare")
		return nil, nil
	}
	if err := performUpdate("3.0.0"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"verify", "prepare", "launch", "close"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events=%v, want=%v", events, want)
	}
}

func TestUpdateVerificationFailureDoesNotStopOrLaunch(t *testing.T) {
	stubUpdateDownload(t)
	originalStage, originalPrepare := stageInstaller, prepareForUpdate
	t.Cleanup(func() { stageInstaller, prepareForUpdate = originalStage, originalPrepare })
	failure := errors.New("checksum failed")
	stageInstaller = func(io.Reader, string, int64) (updateStage, error) { return nil, failure }
	prepareForUpdate = func() (func() error, error) {
		t.Fatal("verification failure must not prepare processes")
		return nil, nil
	}
	if err := performUpdate("3.0.0"); !errors.Is(err, failure) {
		t.Fatalf("error=%v", err)
	}
}

func TestUpdateLaunchFailureRestartsVerifiedDaemon(t *testing.T) {
	stubUpdateDownload(t)
	originalStage, originalPrepare := stageInstaller, prepareForUpdate
	t.Cleanup(func() { stageInstaller, prepareForUpdate = originalStage, originalPrepare })
	var events []string
	failure := errors.New("launch failed")
	stageInstaller = func(io.Reader, string, int64) (updateStage, error) {
		return &fakeUpdateStage{events: &events, launchErr: failure}, nil
	}
	prepareForUpdate = func() (func() error, error) {
		return func() error { events = append(events, "restart"); return nil }, nil
	}
	if err := performUpdate("3.0.0"); !errors.Is(err, failure) {
		t.Fatalf("error=%v", err)
	}
	if want := []string{"launch", "restart", "close"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events=%v, want=%v", events, want)
	}
}

func TestUpdateUncertainStartupDoesNotRestartAgainstRunningInstaller(t *testing.T) {
	stubUpdateDownload(t)
	originalStage, originalPrepare := stageInstaller, prepareForUpdate
	t.Cleanup(func() { stageInstaller, prepareForUpdate = originalStage, originalPrepare })
	var events []string
	stageInstaller = func(io.Reader, string, int64) (updateStage, error) {
		return &fakeUpdateStage{events: &events, launchErr: system.ErrInstallerStartupUncertain}, nil
	}
	prepareForUpdate = func() (func() error, error) {
		return func() error { t.Fatal("uncertain active installer must not restart daemon"); return nil }, nil
	}
	if err := performUpdate("3.0.0"); !errors.Is(err, system.ErrInstallerStartupUncertain) {
		t.Fatalf("error=%v", err)
	}
}

func TestUpdatePreparationRequiresConfirmedTerminalExit(t *testing.T) {
	originalRPC, originalEndpoint, originalCount := updateRPC, updateEndpoint, updateCount
	t.Cleanup(func() { updateRPC, updateEndpoint, updateCount = originalRPC, originalEndpoint, originalCount })
	updateEndpoint = func() (coreapi.Endpoint, error) {
		return coreapi.Endpoint{Generation: "generation", ExePath: `C:\Focusd\focusd_daemon.exe`}, nil
	}
	var commands []string
	updateRPC = func(request coreapi.Request) (coreapi.Response, error) {
		if request.Generation != "generation" {
			t.Fatal("all update control must bind to original daemon generation")
		}
		commands = append(commands, request.Command)
		data := json.RawMessage(`{"active_clients":0}`)
		return coreapi.Response{OK: true, Data: data}, nil
	}
	updateCount = func(string, string) (int, error) { return 0, nil }
	if _, err := prepareUpdateProcesses(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"shutdown-clients", "status", "stop"}; !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands=%v, want=%v", commands, want)
	}
}

func TestUpdatePreparationDoesNotStopWhenClientStatusIsUnknown(t *testing.T) {
	originalRPC, originalEndpoint := updateRPC, updateEndpoint
	t.Cleanup(func() { updateRPC, updateEndpoint = originalRPC, originalEndpoint })
	updateEndpoint = func() (coreapi.Endpoint, error) { return coreapi.Endpoint{Generation: "generation"}, nil }
	updateRPC = func(request coreapi.Request) (coreapi.Response, error) {
		if request.Command == "stop" {
			t.Fatal("unknown terminal state must not stop daemon")
		}
		return coreapi.Response{OK: true, Data: json.RawMessage(`{}`)}, nil
	}
	if _, err := prepareUpdateProcesses(); err == nil {
		t.Fatal("missing active-client status was accepted")
	}
}

func TestUpdateWithoutEndpointDoesNotTreatCountFailureAsStopped(t *testing.T) {
	originalEndpoint, originalCount := updateEndpoint, updateCount
	t.Cleanup(func() { updateEndpoint, updateCount = originalEndpoint, originalCount })
	updateEndpoint = func() (coreapi.Endpoint, error) { return coreapi.Endpoint{}, os.ErrNotExist }
	updateCount = func(string, string) (int, error) { return 0, errors.New("cannot enumerate") }
	if _, err := prepareUpdateProcesses(); err == nil {
		t.Fatal("count failure was treated as no daemon")
	}
}
