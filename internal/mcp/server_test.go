package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/2lines/trajectory-memory/internal/store"
	"github.com/2lines/trajectory-memory/internal/types"
)

func setupTestServer(t *testing.T) (*Server, *store.BoltStore, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "mcp-test-*")
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

	server := NewServer(s, socketPath, "test")

	cleanup := func() {
		s.Close()
		os.RemoveAll(tmpDir)
	}

	return server, s, cleanup
}

func sendRequest(server *Server, method string, params interface{}) Response {
	var input bytes.Buffer
	var output bytes.Buffer

	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  method,
	}
	if params != nil {
		paramsJSON, _ := json.Marshal(params)
		req.Params = paramsJSON
	}

	reqJSON, _ := json.Marshal(req)
	input.Write(append(reqJSON, '\n'))

	server.SetIO(&input, &output)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	server.Run(ctx)

	var resp Response
	json.Unmarshal(output.Bytes(), &resp)
	return resp
}

func TestInitialize(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
	}

	resp := sendRequest(server, "initialize", params)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result InitializeResult
	resultJSON, _ := json.Marshal(resp.Result)
	json.Unmarshal(resultJSON, &result)

	if result.ServerInfo.Name != "trajectory-memory" {
		t.Errorf("expected server name 'trajectory-memory', got %s", result.ServerInfo.Name)
	}
	if result.ProtocolVersion == "" {
		t.Error("protocol version should not be empty")
	}
}

func TestToolsList(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	resp := sendRequest(server, "tools/list", nil)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result ToolsListResult
	resultJSON, _ := json.Marshal(resp.Result)
	json.Unmarshal(resultJSON, &result)

	expectedTools := []string{
		"trajectory_start",
		"trajectory_stop",
		"trajectory_status",
		"trajectory_search",
		"trajectory_list",
		"trajectory_score",
		"trajectory_summarize",
		"trajectory_optimize_propose",
		"trajectory_optimize_save",
		"trajectory_optimize_apply",
		"trajectory_optimize_rollback",
		"trajectory_optimize_history",
		"trajectory_curate_examples",
		"trajectory_curate_apply",
		"trajectory_trigger_status",
		"trajectory_trigger_configure",
	}

	if len(result.Tools) != len(expectedTools) {
		t.Errorf("expected %d tools, got %d", len(expectedTools), len(result.Tools))
	}

	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestTrajectoryStart(t *testing.T) {
	server, s, cleanup := setupTestServer(t)
	defer cleanup()

	params := ToolCallParams{
		Name:      "trajectory_start",
		Arguments: json.RawMessage(`{"task_prompt": "Test task", "tags": ["test"]}`),
	}

	resp := sendRequest(server, "tools/call", params)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result ToolCallResult
	resultJSON, _ := json.Marshal(resp.Result)
	json.Unmarshal(resultJSON, &result)

	if result.IsError {
		t.Errorf("unexpected tool error: %v", result.Content)
	}

	// Verify session was created
	session, err := s.GetActiveSession()
	if err != nil {
		t.Fatalf("no active session: %v", err)
	}

	if session.TaskPrompt != "Test task" {
		t.Errorf("expected task prompt 'Test task', got %s", session.TaskPrompt)
	}
	if len(session.Tags) != 1 || session.Tags[0] != "test" {
		t.Errorf("expected tags ['test'], got %v", session.Tags)
	}
}

func TestTrajectoryStartDuplicate(t *testing.T) {
	server, s, cleanup := setupTestServer(t)
	defer cleanup()

	// Create an active session
	session := &types.Session{
		ID:         store.NewULID(),
		TaskPrompt: "Existing task",
		Status:     types.StatusRecording,
		StartedAt:  time.Now(),
	}
	s.CreateSession(session)
	s.SetActiveSession(session.ID)

	params := ToolCallParams{
		Name:      "trajectory_start",
		Arguments: json.RawMessage(`{"task_prompt": "New task"}`),
	}

	resp := sendRequest(server, "tools/call", params)

	var result ToolCallResult
	resultJSON, _ := json.Marshal(resp.Result)
	json.Unmarshal(resultJSON, &result)

	if !result.IsError {
		t.Error("expected error when starting while already recording")
	}

	if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, "already recording") {
		t.Errorf("expected 'already recording' error, got %v", result.Content)
	}
}

func TestTrajectoryStop(t *testing.T) {
	server, s, cleanup := setupTestServer(t)
	defer cleanup()

	// Create an active session with steps
	session := &types.Session{
		ID:         store.NewULID(),
		TaskPrompt: "Test task",
		Status:     types.StatusRecording,
		StartedAt:  time.Now(),
		Steps: []types.TrajectoryStep{
			{ToolName: "Read", InputSummary: "file.go"},
			{ToolName: "Write", InputSummary: "output.go"},
		},
	}
	s.CreateSession(session)
	s.SetActiveSession(session.ID)

	params := ToolCallParams{
		Name:      "trajectory_stop",
		Arguments: json.RawMessage(`{"score": 0.8, "notes": "Good job"}`),
	}

	resp := sendRequest(server, "tools/call", params)

	var result ToolCallResult
	resultJSON, _ := json.Marshal(resp.Result)
	json.Unmarshal(resultJSON, &result)

	if result.IsError {
		t.Errorf("unexpected tool error: %v", result.Content)
	}

	// Verify session was updated
	updated, err := s.GetSession(session.ID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if updated.Status != types.StatusScored {
		t.Errorf("expected status 'scored', got %s", updated.Status)
	}
	if updated.Outcome == nil {
		t.Fatal("outcome should be set")
	}
	if updated.Outcome.Score != 0.8 {
		t.Errorf("expected score 0.8, got %f", updated.Outcome.Score)
	}

	// Verify no active session
	_, err = s.GetActiveSession()
	if err == nil {
		t.Error("expected no active session after stop")
	}

	// Verify trajectory output contains summarization prompt
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	if !strings.Contains(result.Content[0].Text, "trajectory_summarize") {
		t.Error("expected summarization prompt in output")
	}
}

func TestTrajectoryStatus(t *testing.T) {
	server, s, cleanup := setupTestServer(t)
	defer cleanup()

	// Test with no active session
	params := ToolCallParams{
		Name: "trajectory_status",
	}

	resp := sendRequest(server, "tools/call", params)

	var result ToolCallResult
	resultJSON, _ := json.Marshal(resp.Result)
	json.Unmarshal(resultJSON, &result)

	var status TrajectoryStatusOutput
	json.Unmarshal([]byte(result.Content[0].Text), &status)

	if status.Active {
		t.Error("expected active=false with no session")
	}

	// Create active session
	session := &types.Session{
		ID:         store.NewULID(),
		TaskPrompt: "Test",
		Status:     types.StatusRecording,
		StartedAt:  time.Now(),
		Steps: []types.TrajectoryStep{
			{ToolName: "Read"},
			{ToolName: "Write"},
		},
	}
	s.CreateSession(session)
	s.SetActiveSession(session.ID)

	resp = sendRequest(server, "tools/call", params)
	resultJSON, _ = json.Marshal(resp.Result)
	json.Unmarshal(resultJSON, &result)
	json.Unmarshal([]byte(result.Content[0].Text), &status)

	if !status.Active {
		t.Error("expected active=true with session")
	}
	if status.SessionID != session.ID {
		t.Errorf("expected session ID %s, got %s", session.ID, status.SessionID)
	}
	if status.StepCount != 2 {
		t.Errorf("expected 2 steps, got %d", status.StepCount)
	}
}

func TestTrajectorySearch(t *testing.T) {
	server, s, cleanup := setupTestServer(t)
	defer cleanup()

	// Create sessions
	sessions := []*types.Session{
		{
			ID:         store.NewULID(),
			TaskPrompt: "Implement fibonacci",
			Summary:    "Created recursive fibonacci function",
			Status:     types.StatusScored,
			StartedAt:  time.Now(),
			Outcome:    &types.Outcome{Score: 0.9},
		},
		{
			ID:         store.NewULID(),
			TaskPrompt: "Write API tests",
			Summary:    "Added unit tests for API",
			Status:     types.StatusScored,
			StartedAt:  time.Now(),
			Outcome:    &types.Outcome{Score: 0.5},
		},
	}
	for _, sess := range sessions {
		s.CreateSession(sess)
	}

	// Search for fibonacci
	params := ToolCallParams{
		Name:      "trajectory_search",
		Arguments: json.RawMessage(`{"query": "fibonacci", "limit": 5}`),
	}

	resp := sendRequest(server, "tools/call", params)

	var result ToolCallResult
	resultJSON, _ := json.Marshal(resp.Result)
	json.Unmarshal(resultJSON, &result)

	var searchResults []TrajectorySearchResult
	json.Unmarshal([]byte(result.Content[0].Text), &searchResults)

	if len(searchResults) != 1 {
		t.Errorf("expected 1 result, got %d", len(searchResults))
	}

	// Search with min_score filter
	params.Arguments = json.RawMessage(`{"query": "test", "min_score": 0.6}`)
	resp = sendRequest(server, "tools/call", params)
	resultJSON, _ = json.Marshal(resp.Result)
	json.Unmarshal(resultJSON, &result)
	json.Unmarshal([]byte(result.Content[0].Text), &searchResults)

	// "test" matches "Write API tests" but score is 0.5 < 0.6
	if len(searchResults) != 0 {
		t.Errorf("expected 0 results with min_score filter, got %d", len(searchResults))
	}
}

func TestTrajectoryScore(t *testing.T) {
	server, s, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a session
	session := &types.Session{
		ID:         store.NewULID(),
		TaskPrompt: "Test task",
		Status:     types.StatusCompleted,
		StartedAt:  time.Now(),
	}
	s.CreateSession(session)

	params := ToolCallParams{
		Name:      "trajectory_score",
		Arguments: json.RawMessage(`{"session_id": "` + session.ID + `", "score": 0.75, "notes": "Good work"}`),
	}

	resp := sendRequest(server, "tools/call", params)

	var result ToolCallResult
	resultJSON, _ := json.Marshal(resp.Result)
	json.Unmarshal(resultJSON, &result)

	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}

	// Verify outcome was set
	updated, _ := s.GetSession(session.ID)
	if updated.Outcome == nil {
		t.Fatal("outcome should be set")
	}
	if updated.Outcome.Score != 0.75 {
		t.Errorf("expected score 0.75, got %f", updated.Outcome.Score)
	}
	if updated.Status != types.StatusScored {
		t.Errorf("expected status 'scored', got %s", updated.Status)
	}
}

func TestTrajectoryScoreInvalidRange(t *testing.T) {
	server, s, cleanup := setupTestServer(t)
	defer cleanup()

	session := &types.Session{
		ID:         store.NewULID(),
		TaskPrompt: "Test",
		Status:     types.StatusCompleted,
		StartedAt:  time.Now(),
	}
	s.CreateSession(session)

	// Score > 1
	params := ToolCallParams{
		Name:      "trajectory_score",
		Arguments: json.RawMessage(`{"session_id": "` + session.ID + `", "score": 1.5}`),
	}

	resp := sendRequest(server, "tools/call", params)

	var result ToolCallResult
	resultJSON, _ := json.Marshal(resp.Result)
	json.Unmarshal(resultJSON, &result)

	if !result.IsError {
		t.Error("expected error for score > 1")
	}
}

func TestTrajectorySummarize(t *testing.T) {
	server, s, cleanup := setupTestServer(t)
	defer cleanup()

	session := &types.Session{
		ID:         store.NewULID(),
		TaskPrompt: "Test task",
		Status:     types.StatusCompleted,
		StartedAt:  time.Now(),
	}
	s.CreateSession(session)

	params := ToolCallParams{
		Name:      "trajectory_summarize",
		Arguments: json.RawMessage(`{"session_id": "` + session.ID + `", "summary": "This session implemented X using Y approach"}`),
	}

	resp := sendRequest(server, "tools/call", params)

	var result ToolCallResult
	resultJSON, _ := json.Marshal(resp.Result)
	json.Unmarshal(resultJSON, &result)

	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}

	// Verify summary was stored
	updated, _ := s.GetSession(session.ID)
	if updated.Summary != "This session implemented X using Y approach" {
		t.Errorf("summary not stored correctly: %s", updated.Summary)
	}
}

func TestMethodNotFound(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	resp := sendRequest(server, "unknown/method", nil)

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != MethodNotFound {
		t.Errorf("expected MethodNotFound error code, got %d", resp.Error.Code)
	}
}

// FormatTrajectory tests are in the summarize package
