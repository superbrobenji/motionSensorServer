package mesh

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

// newTestMeshServer builds a minimal MeshServer that doesn't open a serial
// port — suitable for testing the API layer in isolation.
func newTestMeshServer(t *testing.T) *MeshServer {
	t.Helper()
	cfg := MeshServerConfig{
		SerialPort:       "",
		BaudRate:         115200,
		HealthTimeout:    30 * time.Second,
		EventStore:       NewMockEventStore(),
		AuthRegistryPath: "",
		NodeRegistryPath: "",
	}
	return NewMeshServer(cfg)
}

// freePort asks the OS for an available TCP port and returns it.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// TestStartAPIServer_ReturnsNonNilShutdown verifies that StartAPIServer
// returns a non-nil shutdown function without error.
func TestStartAPIServer_ReturnsNonNilShutdown(t *testing.T) {
	ms := newTestMeshServer(t)
	port := freePort(t)

	shutdown, err := StartAPIServer(ms, port, "", nil)
	if err != nil {
		t.Fatalf("StartAPIServer returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("StartAPIServer returned nil shutdown function")
	}

	// Clean up
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown returned error: %v", err)
	}
}

// TestStartAPIServer_AcceptsHTTPConnection verifies that after StartAPIServer
// the server is reachable and GET /status returns a successful JSON response.
func TestStartAPIServer_AcceptsHTTPConnection(t *testing.T) {
	ms := newTestMeshServer(t)
	port := freePort(t)

	shutdown, err := StartAPIServer(ms, port, "", nil)
	if err != nil {
		t.Fatalf("StartAPIServer returned error: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdown(ctx) //nolint:errcheck
	})

	url := fmt.Sprintf("http://127.0.0.1:%d/status", port)
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		t.Fatalf("GET /status failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if !body.Success {
		t.Errorf("expected success=true, got false (error: %s)", body.Error)
	}
}

// TestStartAPIServer_ShutdownCompletesWithoutError verifies that calling the
// returned shutdown function completes without error within 10 seconds.
func TestStartAPIServer_ShutdownCompletesWithoutError(t *testing.T) {
	ms := newTestMeshServer(t)
	port := freePort(t)

	shutdown, err := StartAPIServer(ms, port, "", nil)
	if err != nil {
		t.Fatalf("StartAPIServer returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown returned unexpected error: %v", err)
	}
}

// TestStartAPIServer_RejectsConnectionsAfterShutdown verifies that once
// shutdown completes the server no longer accepts new TCP connections.
func TestStartAPIServer_RejectsConnectionsAfterShutdown(t *testing.T) {
	ms := newTestMeshServer(t)
	port := freePort(t)

	shutdown, err := StartAPIServer(ms, port, "", nil)
	if err != nil {
		t.Fatalf("StartAPIServer returned error: %v", err)
	}

	// Confirm the port is open before shutdown
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	pre, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("expected port %d to be open before shutdown: %v", port, err)
	}
	pre.Close()

	// Shut the server down
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown returned error: %v", err)
	}

	// Give the OS a moment to release the socket
	time.Sleep(50 * time.Millisecond)

	// Now the port should be closed
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Errorf("expected port %d to be closed after shutdown, but a connection succeeded", port)
	}
}
