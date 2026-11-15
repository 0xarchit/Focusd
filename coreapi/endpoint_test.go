package coreapi

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testEndpoint(t *testing.T) Endpoint {
	t.Helper()
	return Endpoint{Version: ProtocolVersion, Address: "127.0.0.1:12345", Generation: "daemon-generation", PID: 123, ExePath: filepath.Join(t.TempDir(), "focusd_daemon.exe")}
}

func TestEndpointRejectsRemoteAndIncompleteDiscovery(t *testing.T) {
	for _, address := range []string{"192.0.2.1:12345", "localhost:12345", "127.0.0.1:0", "127.0.0.1:65536", "invalid", "[::1]:-1"} {
		endpoint := testEndpoint(t)
		endpoint.Address = address
		if err := validateEndpoint(endpoint); err == nil {
			t.Errorf("endpoint %q accepted", address)
		}
	}
	endpoint := testEndpoint(t)
	endpoint.Generation = ""
	if err := validateEndpoint(endpoint); err == nil {
		t.Fatal("missing daemon identity accepted")
	}
}

func TestEndpointPublishAndRemoveAreGenerationScoped(t *testing.T) {
	setupTempAppData(t)
	endpoint := testEndpoint(t)
	if err := PublishEndpoint(endpoint); err != nil {
		t.Fatal(err)
	}
	got, err := ReadEndpoint()
	if err != nil || got != endpoint {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if err := RemoveEndpoint("older-generation"); err == nil {
		t.Fatal("old daemon removed a newer endpoint")
	}
	if _, err := ReadEndpoint(); err != nil {
		t.Fatalf("endpoint disappeared: %v", err)
	}
	if err := RemoveEndpoint(endpoint.Generation); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadEndpoint(); !os.IsNotExist(err) {
		t.Fatalf("endpoint remains: %v", err)
	}
}

func TestReadEndpointCapsDiscoverySize(t *testing.T) {
	setupTempAppData(t)
	path, err := endpointPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat(" ", MaxIPCBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadEndpoint(); err == nil {
		t.Fatal("oversized endpoint accepted")
	}
}

func TestCallUsesPublishedEndpointAndCorrelatesResponse(t *testing.T) {
	setupTempAppData(t)
	token, err := EnsureIPCToken()
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	endpoint := testEndpoint(t)
	endpoint.Address = listener.Addr().String()
	if err := PublishEndpoint(endpoint); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		request, err := ReadRequest(conn)
		if err != nil {
			done <- err
			return
		}
		if request.Token != token || request.Generation != endpoint.Generation || request.Command != "status" || request.RequestID != "request-1" {
			done <- errors.New("request identity mismatch")
			return
		}
		done <- WriteResponse(conn, Response{OK: true, Generation: endpoint.Generation, RequestID: request.RequestID, State: "awaiting_choice"})
	}()
	response, err := Call(Request{Command: "status", RequestID: "request-1"})
	if err != nil || response.State != "awaiting_choice" {
		t.Fatalf("response=%+v err=%v", response, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestCallRejectsWrongGenerationAndServerReply(t *testing.T) {
	setupTempAppData(t)
	if _, err := EnsureIPCToken(); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	endpoint := testEndpoint(t)
	endpoint.Address = listener.Addr().String()
	if err := PublishEndpoint(endpoint); err != nil {
		t.Fatal(err)
	}
	if _, err := Call(Request{Command: "stop", Generation: "old"}); err == nil {
		t.Fatal("stale generation submitted")
	}
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		request, err := ReadRequest(conn)
		if err != nil {
			done <- err
			return
		}
		done <- WriteResponse(conn, Response{OK: true, Generation: "other", RequestID: request.RequestID})
	}()
	if _, err := Call(Request{Command: "status"}); err == nil {
		t.Fatal("uncorrelated response accepted")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestReadRequestRejectsOversizeAndUnknownFields(t *testing.T) {
	request := Request{Version: ProtocolVersion, Command: "status", Token: "token", Generation: "generation", RequestID: "request"}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRequest(strings.NewReader(string(data) + "\n")); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{strings.Repeat("x", MaxIPCBytes) + "\n", string(data[:len(data)-1]) + `,"extra":1}` + "\n", string(data) + ` {}` + "\n", string(data)} {
		if _, err := ReadRequest(bufio.NewReader(strings.NewReader(raw))); err == nil {
			t.Errorf("invalid envelope accepted: %.80q", raw)
		}
	}
}

func TestCallPreservesRejectedOperationResponse(t *testing.T) {
	setupTempAppData(t)
	if _, err := EnsureIPCToken(); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	endpoint := testEndpoint(t)
	endpoint.Address = listener.Addr().String()
	if err := PublishEndpoint(endpoint); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		request, err := ReadRequest(conn)
		if err != nil {
			done <- err
			return
		}
		done <- WriteResponse(conn, Response{Generation: endpoint.Generation, RequestID: request.RequestID, State: "conflict", Error: "incident changed", OperationID: "operation-1"})
	}()
	response, err := Call(Request{Command: "clock-resolve", IncidentID: "incident-1", Revision: 1})
	var rpcError *RPCError
	if !errors.As(err, &rpcError) || response.State != "conflict" || rpcError.Response.OperationID != "operation-1" {
		t.Fatalf("response=%+v err=%v", response, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestCallWithNoEndpointNeverAttemptsLegacyFixedPort(t *testing.T) {
	setupTempAppData(t)
	if _, err := EnsureIPCToken(); err != nil {
		t.Fatal(err)
	}
	if _, err := Call(Request{Command: "stop"}); !os.IsNotExist(err) {
		t.Fatalf("err=%v; want missing discovery without a connection attempt", err)
	}
	if SendIPCCmd("stop") {
		t.Fatal("missing endpoint was reported as a successful legacy stop")
	}
}
