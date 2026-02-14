package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/2lines/trajectory-memory/internal/store"
	"github.com/2lines/trajectory-memory/internal/types"
)

func setupTestServer(t *testing.T) (*Server, *store.BoltStore, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "ingestion-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	socketPath := filepath.Join(tmpDir, "test.sock")

	s, err := store.NewBoltStore(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create store: %v", err)
	}

	server := NewServer(s, socketPath)

	cleanup := func() {
		server.Stop()
		s.Close()
		os.RemoveAll(tmpDir)
	}

	return server, s, socketPath, cleanup
}

func createUnixClient(socketPath string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 5 * time.Second,
	}
}

func TestServerStartStop(t *testing.T) {
	server, _, _, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server
	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	if !server.IsRunning() {
		t.Error("server should be running")
	}

	// Stop server
	if err := server.Stop(); err != nil {
		t.Fatalf("failed to stop server: %v", err)
	}

	if server.IsRunning() {
		t.Error("server should not be running after stop")
	}
}

func TestServerDoubleStart(t *testing.T) {
	server, _, _, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("first start failed: %v", err)
	}

	err := server.Start(ctx)
	if err == nil {
		t.Error("expected error on double start")
	}
}

func TestHealthEndpoint(t *testing.T) {
	server, _, socketPath, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	client := createUnixClient(socketPath)

	resp, err := client.Get("http://localhost/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("expected body 'ok', got %s", body)
	}
}

func TestStepEndpoint_NoActiveSession(t *testing.T) {
	server, _, socketPath, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	client := createUnixClient(socketPath)

	payload := HookPayload{
		ToolName:   "Read",
		ToolInput:  json.RawMessage(`{"file_path": "/test/file.go"}`),
		ToolOutput: json.RawMessage(`"file contents"`),
	}

	body, _ := json.Marshal(payload)
	resp, err := client.Post("http://localhost/step", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("step request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404 without active session, got %d", resp.StatusCode)
	}
}

func TestStepEndpoint_Success(t *testing.T) {
	server, s, socketPath, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create and activate a session
	session := &types.Session{
		ID:         store.NewULID(),
		TaskPrompt: "Test task",
		Status:     types.StatusRecording,
		StartedAt:  time.Now(),
	}
	if err := s.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if err := s.SetActiveSession(session.ID); err != nil {
		t.Fatalf("failed to set active session: %v", err)
	}

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	client := createUnixClient(socketPath)

	payload := HookPayload{
		ToolName:   "Read",
		ToolInput:  json.RawMessage(`{"file_path": "/test/file.go"}`),
		ToolOutput: json.RawMessage(`"file contents here"`),
		DurationMs: 50,
	}

	body, _ := json.Marshal(payload)
	resp, err := client.Post("http://localhost/step", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("step request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Verify step was recorded
	updated, err := s.GetSession(session.ID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if len(updated.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(updated.Steps))
	}

	step := updated.Steps[0]
	if step.ToolName != "Read" {
		t.Errorf("expected tool name 'Read', got %s", step.ToolName)
	}
	if step.DurationMs != 50 {
		t.Errorf("expected duration 50ms, got %d", step.DurationMs)
	}
}

func TestStepEndpoint_MarkdownLoadedContext(t *testing.T) {
	server, s, socketPath, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create and activate a session
	session := &types.Session{
		ID:            store.NewULID(),
		TaskPrompt:    "Test task",
		Status:        types.StatusRecording,
		StartedAt:     time.Now(),
		LoadedContext: []string{},
	}
	if err := s.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if err := s.SetActiveSession(session.ID); err != nil {
		t.Fatalf("failed to set active session: %v", err)
	}

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	client := createUnixClient(socketPath)

	// Read a markdown file
	payload := HookPayload{
		ToolName:   "Read",
		ToolInput:  json.RawMessage(`{"file_path": "/project/CLAUDE.md"}`),
		ToolOutput: json.RawMessage(`"# Instructions"`),
	}

	body, _ := json.Marshal(payload)
	resp, err := client.Post("http://localhost/step", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("step request failed: %v", err)
	}
	resp.Body.Close()

	// Verify LoadedContext was updated
	updated, err := s.GetSession(session.ID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if len(updated.LoadedContext) != 1 {
		t.Fatalf("expected 1 loaded context, got %d", len(updated.LoadedContext))
	}
	if updated.LoadedContext[0] != "/project/CLAUDE.md" {
		t.Errorf("expected '/project/CLAUDE.md' in loaded context, got %s", updated.LoadedContext[0])
	}

	// Send same file again - should not duplicate
	resp, err = client.Post("http://localhost/step", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("step request failed: %v", err)
	}
	resp.Body.Close()

	updated, _ = s.GetSession(session.ID)
	if len(updated.LoadedContext) != 1 {
		t.Errorf("loaded context should not have duplicates, got %d entries", len(updated.LoadedContext))
	}
}

func TestStepEndpoint_BadPayload(t *testing.T) {
	server, _, socketPath, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	client := createUnixClient(socketPath)

	// Invalid JSON
	resp, err := client.Post("http://localhost/step", "application/json", bytes.NewReader([]byte("invalid")))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d", resp.StatusCode)
	}
}

func TestStepEndpoint_MissingToolName(t *testing.T) {
	server, s, socketPath, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create and activate a session
	session := &types.Session{
		ID:         store.NewULID(),
		TaskPrompt: "Test task",
		Status:     types.StatusRecording,
		StartedAt:  time.Now(),
	}
	if err := s.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if err := s.SetActiveSession(session.ID); err != nil {
		t.Fatalf("failed to set active session: %v", err)
	}

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	client := createUnixClient(socketPath)

	// Missing tool_name
	payload := HookPayload{
		ToolInput: json.RawMessage(`{}`),
	}

	body, _ := json.Marshal(payload)
	resp, err := client.Post("http://localhost/step", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing tool_name, got %d", resp.StatusCode)
	}
}

func TestStepEndpoint_MethodNotAllowed(t *testing.T) {
	server, _, socketPath, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	client := createUnixClient(socketPath)

	resp, err := client.Get("http://localhost/step")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", resp.StatusCode)
	}
}

func TestSummarizeJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    json.RawMessage
		expected string
	}{
		{
			name:     "file_path extraction",
			input:    json.RawMessage(`{"file_path": "/test/file.go", "other": "data"}`),
			expected: "/test/file.go",
		},
		{
			name:     "command extraction",
			input:    json.RawMessage(`{"command": "go test ./..."}`),
			expected: "go test ./...",
		},
		{
			name:     "raw string",
			input:    json.RawMessage(`"just a string"`),
			expected: "just a string",
		},
		{
			name:     "empty",
			input:    json.RawMessage(``),
			expected: "",
		},
		{
			name:     "fallback to raw",
			input:    json.RawMessage(`{"unknown": "field"}`),
			expected: `{"unknown": "field"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := summarizeJSON(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestContextCancellation(t *testing.T) {
	server, _, socketPath, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	if !server.IsRunning() {
		t.Error("server should be running")
	}

	// Cancel context - should trigger shutdown
	cancel()

	// Wait a bit for shutdown
	time.Sleep(100 * time.Millisecond)

	// Try to connect - should fail
	client := createUnixClient(socketPath)
	_, err := client.Get("http://localhost/health")
	if err == nil {
		t.Error("expected connection to fail after context cancellation")
	}
}

func TestConcurrentSteps(t *testing.T) {
	server, s, socketPath, cleanup := setupTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create and activate a session
	session := &types.Session{
		ID:         store.NewULID(),
		TaskPrompt: "Test task",
		Status:     types.StatusRecording,
		StartedAt:  time.Now(),
	}
	if err := s.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if err := s.SetActiveSession(session.ID); err != nil {
		t.Fatalf("failed to set active session: %v", err)
	}

	if err := server.Start(ctx); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Send multiple concurrent requests
	const numRequests = 10
	done := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(n int) {
			client := createUnixClient(socketPath)
			payload := HookPayload{
				ToolName:   "Read",
				ToolInput:  json.RawMessage(`{"file_path": "/test/file.go"}`),
				ToolOutput: json.RawMessage(`"contents"`),
			}

			body, _ := json.Marshal(payload)
			resp, err := client.Post("http://localhost/step", "application/json", bytes.NewReader(body))
			if err != nil {
				done <- err
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				done <- err
				return
			}
			done <- nil
		}(i)
	}

	// Wait for all requests
	for i := 0; i < numRequests; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent request failed: %v", err)
		}
	}

	// Verify all steps were recorded
	updated, err := s.GetSession(session.ID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if len(updated.Steps) != numRequests {
		t.Errorf("expected %d steps, got %d", numRequests, len(updated.Steps))
	}
}
