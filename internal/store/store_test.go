package store

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/2lines/trajectory-memory/internal/types"
)

func setupTestStore(t *testing.T) (*BoltStore, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "trajectory-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := NewBoltStore(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func createTestSession(id string) *types.Session {
	now := time.Now()
	return &types.Session{
		ID:            id,
		TaskPrompt:    "Write a function to calculate fibonacci numbers",
		WorkingDir:    "/home/user/project",
		ClaudeMDHash:  "abc123",
		LoadedContext: []string{"CLAUDE.md", "README.md"},
		Steps:         []types.TrajectoryStep{},
		Summary:       "",
		Outcome:       nil,
		Tags:          []string{"coding", "math"},
		Strategy:      "default",
		StartedAt:     now,
		CompletedAt:   nil,
		Status:        types.StatusRecording,
	}
}

func TestCreateSession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := createTestSession(NewULID())

	// Test successful creation
	err := store.CreateSession(session)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Verify session was stored
	retrieved, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, session.ID)
	}
	if retrieved.TaskPrompt != session.TaskPrompt {
		t.Errorf("TaskPrompt mismatch: got %s, want %s", retrieved.TaskPrompt, session.TaskPrompt)
	}
	if retrieved.Status != types.StatusRecording {
		t.Errorf("Status mismatch: got %s, want %s", retrieved.Status, types.StatusRecording)
	}

	// Test duplicate creation
	err = store.CreateSession(session)
	if err == nil {
		t.Error("Expected error on duplicate creation, got nil")
	}
}

func TestGetSession_NotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	_, err := store.GetSession("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("Expected ErrSessionNotFound, got %v", err)
	}
}

func TestUpdateSession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := createTestSession(NewULID())
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Update the session
	session.Summary = "Successfully implemented fibonacci function"
	session.Status = types.StatusCompleted
	now := time.Now()
	session.CompletedAt = &now

	err := store.UpdateSession(session)
	if err != nil {
		t.Fatalf("UpdateSession failed: %v", err)
	}

	// Verify updates
	retrieved, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if retrieved.Summary != session.Summary {
		t.Errorf("Summary mismatch: got %s, want %s", retrieved.Summary, session.Summary)
	}
	if retrieved.Status != types.StatusCompleted {
		t.Errorf("Status mismatch: got %s, want %s", retrieved.Status, types.StatusCompleted)
	}
}

func TestUpdateSession_NotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := createTestSession("nonexistent")
	err := store.UpdateSession(session)
	if err != ErrSessionNotFound {
		t.Errorf("Expected ErrSessionNotFound, got %v", err)
	}
}

func TestAppendStep(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := createTestSession(NewULID())
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Append steps
	steps := []types.TrajectoryStep{
		{
			Timestamp:     time.Now(),
			ToolName:      "Read",
			InputSummary:  "Reading CLAUDE.md",
			OutputSummary: "File contents...",
			DurationMs:    50,
		},
		{
			Timestamp:     time.Now(),
			ToolName:      "Write",
			InputSummary:  "Writing fibonacci.go",
			OutputSummary: "File written successfully",
			DurationMs:    100,
		},
	}

	for _, step := range steps {
		if err := store.AppendStep(session.ID, step); err != nil {
			t.Fatalf("AppendStep failed: %v", err)
		}
	}

	// Verify steps were added
	retrieved, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if len(retrieved.Steps) != 2 {
		t.Errorf("Step count mismatch: got %d, want 2", len(retrieved.Steps))
	}

	if retrieved.Steps[0].ToolName != "Read" {
		t.Errorf("First step ToolName mismatch: got %s, want Read", retrieved.Steps[0].ToolName)
	}
}

func TestAppendStep_Truncation(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := createTestSession(NewULID())
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Create a step with very long input/output
	longText := strings.Repeat("x", 1000)
	step := types.TrajectoryStep{
		Timestamp:     time.Now(),
		ToolName:      "Read",
		InputSummary:  longText,
		OutputSummary: longText,
		DurationMs:    50,
	}

	if err := store.AppendStep(session.ID, step); err != nil {
		t.Fatalf("AppendStep failed: %v", err)
	}

	// Verify truncation
	retrieved, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if len(retrieved.Steps[0].InputSummary) != types.MaxInputSummaryLen {
		t.Errorf("InputSummary not truncated: got %d, want %d",
			len(retrieved.Steps[0].InputSummary), types.MaxInputSummaryLen)
	}
	if len(retrieved.Steps[0].OutputSummary) != types.MaxOutputSummaryLen {
		t.Errorf("OutputSummary not truncated: got %d, want %d",
			len(retrieved.Steps[0].OutputSummary), types.MaxOutputSummaryLen)
	}
}

func TestAppendStep_NotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	step := types.TrajectoryStep{
		Timestamp:     time.Now(),
		ToolName:      "Read",
		InputSummary:  "test",
		OutputSummary: "test",
	}

	err := store.AppendStep("nonexistent", step)
	if err != ErrSessionNotFound {
		t.Errorf("Expected ErrSessionNotFound, got %v", err)
	}
}

func TestListSessions(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create multiple sessions
	for i := 0; i < 5; i++ {
		session := createTestSession(NewULID())
		session.TaskPrompt = "Task " + string(rune('A'+i))
		if err := store.CreateSession(session); err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		time.Sleep(time.Millisecond) // Ensure different ULIDs
	}

	// Test listing with limit
	results, err := store.ListSessions(3, 0)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Most recent should be first (Task E)
	if !strings.HasPrefix(results[0].TaskPrompt, "Task E") {
		t.Errorf("Expected most recent first, got %s", results[0].TaskPrompt)
	}

	// Test offset
	results, err = store.ListSessions(3, 2)
	if err != nil {
		t.Fatalf("ListSessions with offset failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results with offset, got %d", len(results))
	}
}

func TestSearchSessions(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create sessions with different content
	sessions := []*types.Session{
		{
			ID:         NewULID(),
			TaskPrompt: "Implement fibonacci algorithm",
			Summary:    "Implemented recursive fibonacci",
			Tags:       []string{"math", "algorithm"},
			Status:     types.StatusCompleted,
			StartedAt:  time.Now(),
		},
		{
			ID:         NewULID(),
			TaskPrompt: "Write API endpoint",
			Summary:    "Created REST endpoint for users",
			Tags:       []string{"api", "backend"},
			Status:     types.StatusCompleted,
			StartedAt:  time.Now(),
		},
		{
			ID:         NewULID(),
			TaskPrompt: "Fix database connection bug",
			Summary:    "Fixed connection pooling issue",
			Tags:       []string{"bugfix", "database"},
			Status:     types.StatusCompleted,
			StartedAt:  time.Now(),
		},
	}

	for _, s := range sessions {
		if err := store.CreateSession(s); err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
	}

	tests := []struct {
		query    string
		expected int
	}{
		{"fibonacci", 1},
		{"API", 1},
		{"database", 1},
		{"math", 1},           // tag search
		{"endpoint", 1},       // summary search
		{"nonexistent", 0},
		{"a", 3},              // all contain 'a' somewhere
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			results, err := store.SearchSessions(tc.query, 10)
			if err != nil {
				t.Fatalf("SearchSessions failed: %v", err)
			}
			if len(results) != tc.expected {
				t.Errorf("Query %q: expected %d results, got %d", tc.query, tc.expected, len(results))
			}
		})
	}
}

func TestSetOutcome(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := createTestSession(NewULID())
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	outcome := types.Outcome{
		Score:    0.85,
		Notes:    "Good implementation, could improve error handling",
		ScoredAt: time.Now(),
	}

	err := store.SetOutcome(session.ID, outcome)
	if err != nil {
		t.Fatalf("SetOutcome failed: %v", err)
	}

	// Verify outcome was set
	retrieved, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if retrieved.Outcome == nil {
		t.Fatal("Outcome should not be nil")
	}
	if retrieved.Outcome.Score != 0.85 {
		t.Errorf("Score mismatch: got %f, want 0.85", retrieved.Outcome.Score)
	}
	if retrieved.Status != types.StatusScored {
		t.Errorf("Status should be 'scored', got %s", retrieved.Status)
	}
}

func TestSetOutcome_NotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	outcome := types.Outcome{Score: 0.5}
	err := store.SetOutcome("nonexistent", outcome)
	if err != ErrSessionNotFound {
		t.Errorf("Expected ErrSessionNotFound, got %v", err)
	}
}

func TestActiveSession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Initially no active session
	_, err := store.GetActiveSession()
	if err != ErrNoActiveSession {
		t.Errorf("Expected ErrNoActiveSession, got %v", err)
	}

	// Create and set active session
	session := createTestSession(NewULID())
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if err := store.SetActiveSession(session.ID); err != nil {
		t.Fatalf("SetActiveSession failed: %v", err)
	}

	// Verify active session
	active, err := store.GetActiveSession()
	if err != nil {
		t.Fatalf("GetActiveSession failed: %v", err)
	}
	if active.ID != session.ID {
		t.Errorf("Active session ID mismatch: got %s, want %s", active.ID, session.ID)
	}

	// Try to set another active session (should fail)
	session2 := createTestSession(NewULID())
	if err := store.CreateSession(session2); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	err = store.SetActiveSession(session2.ID)
	if err != ErrSessionAlreadyActive {
		t.Errorf("Expected ErrSessionAlreadyActive, got %v", err)
	}

	// Clear active and set new one
	if err := store.ClearActiveSession(); err != nil {
		t.Fatalf("ClearActiveSession failed: %v", err)
	}

	if err := store.SetActiveSession(session2.ID); err != nil {
		t.Fatalf("SetActiveSession failed after clear: %v", err)
	}

	active, _ = store.GetActiveSession()
	if active.ID != session2.ID {
		t.Errorf("Active session should be session2")
	}
}

func TestDeleteSession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	session := createTestSession(NewULID())
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Set as active
	if err := store.SetActiveSession(session.ID); err != nil {
		t.Fatalf("SetActiveSession failed: %v", err)
	}

	// Delete
	if err := store.DeleteSession(session.ID); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Verify deletion
	_, err := store.GetSession(session.ID)
	if err != ErrSessionNotFound {
		t.Errorf("Expected ErrSessionNotFound after delete, got %v", err)
	}

	// Verify active was cleared
	_, err = store.GetActiveSession()
	if err != ErrNoActiveSession {
		t.Errorf("Expected ErrNoActiveSession after delete, got %v", err)
	}
}

func TestDeleteSession_NotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	err := store.DeleteSession("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("Expected ErrSessionNotFound, got %v", err)
	}
}

func TestExportImport(t *testing.T) {
	store1, cleanup1 := setupTestStore(t)
	defer cleanup1()

	// Create sessions in store1
	sessions := []*types.Session{
		{
			ID:         NewULID(),
			TaskPrompt: "Task 1",
			Status:     types.StatusCompleted,
			StartedAt:  time.Now(),
		},
		{
			ID:         NewULID(),
			TaskPrompt: "Task 2",
			Status:     types.StatusScored,
			Outcome:    &types.Outcome{Score: 0.9, ScoredAt: time.Now()},
			StartedAt:  time.Now(),
		},
	}

	for _, s := range sessions {
		if err := store1.CreateSession(s); err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
	}

	// Export
	var buf bytes.Buffer
	if err := store1.ExportAll(&buf); err != nil {
		t.Fatalf("ExportAll failed: %v", err)
	}

	exportedData := buf.String()
	if !strings.Contains(exportedData, "Task 1") || !strings.Contains(exportedData, "Task 2") {
		t.Error("Export should contain both tasks")
	}

	// Import into new store
	store2, cleanup2 := setupTestStore(t)
	defer cleanup2()

	if err := store2.ImportAll(strings.NewReader(exportedData)); err != nil {
		t.Fatalf("ImportAll failed: %v", err)
	}

	// Verify import
	for _, s := range sessions {
		retrieved, err := store2.GetSession(s.ID)
		if err != nil {
			t.Errorf("Failed to get imported session %s: %v", s.ID, err)
			continue
		}
		if retrieved.TaskPrompt != s.TaskPrompt {
			t.Errorf("TaskPrompt mismatch: got %s, want %s", retrieved.TaskPrompt, s.TaskPrompt)
		}
	}
}

func TestSessionMetadata(t *testing.T) {
	session := &types.Session{
		ID:         "test-id",
		TaskPrompt: strings.Repeat("x", 300), // Longer than MaxTaskPromptMetadataLen
		Steps: []types.TrajectoryStep{
			{ToolName: "Read"},
			{ToolName: "Write"},
		},
		Tags:      []string{"tag1", "tag2"},
		Status:    types.StatusScored,
		StartedAt: time.Now(),
		Outcome:   &types.Outcome{Score: 0.75},
	}

	meta := session.ToMetadata()

	if meta.ID != session.ID {
		t.Errorf("ID mismatch")
	}
	if len(meta.TaskPrompt) != types.MaxTaskPromptMetadataLen {
		t.Errorf("TaskPrompt should be truncated to %d, got %d",
			types.MaxTaskPromptMetadataLen, len(meta.TaskPrompt))
	}
	if meta.StepCount != 2 {
		t.Errorf("StepCount should be 2, got %d", meta.StepCount)
	}
	if meta.Score == nil || *meta.Score != 0.75 {
		t.Errorf("Score mismatch")
	}
}

func TestNewULID(t *testing.T) {
	// Test uniqueness
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := NewULID()
		if ids[id] {
			t.Errorf("Duplicate ULID generated: %s", id)
		}
		ids[id] = true

		// Test length (26 characters)
		if len(id) != 26 {
			t.Errorf("ULID should be 26 chars, got %d: %s", len(id), id)
		}
	}

	// Test sortability (later ULIDs should be >= earlier ones)
	id1 := NewULID()
	time.Sleep(time.Millisecond)
	id2 := NewULID()
	if id2 < id1 {
		t.Errorf("Later ULID should be >= earlier ULID: %s < %s", id2, id1)
	}
}
