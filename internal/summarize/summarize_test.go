package summarize

import (
	"strings"
	"testing"
	"time"

	"github.com/2lines/trajectory-memory/internal/types"
)

func createTestSession() *types.Session {
	now := time.Now()
	completed := now.Add(5 * time.Minute)

	return &types.Session{
		ID:            "01HTEST123456789012345",
		TaskPrompt:    "Implement a fibonacci function",
		WorkingDir:    "/home/user/project",
		ClaudeMDHash:  "abcdef1234567890abcdef1234567890",
		LoadedContext: []string{"CLAUDE.md", "README.md"},
		Tags:          []string{"algorithm", "math"},
		Status:        types.StatusCompleted,
		StartedAt:     now,
		CompletedAt:   &completed,
		Steps: []types.TrajectoryStep{
			{ToolName: "Read", InputSummary: "CLAUDE.md", DurationMs: 50},
			{ToolName: "Read", InputSummary: "src/main.go", DurationMs: 30},
			{ToolName: "Write", InputSummary: "src/fibonacci.go", OutputSummary: "Created file with 50 lines", DurationMs: 100},
			{ToolName: "Bash", InputSummary: "go test ./...", OutputSummary: "PASS", DurationMs: 500},
		},
	}
}

func TestFormatTrajectoryForSummarization(t *testing.T) {
	session := createTestSession()
	output := FormatTrajectoryForSummarization(session)

	// Check for required elements
	required := []string{
		"## Session Trajectory",
		"01HTEST123456789012345",
		"fibonacci",
		"Working Directory",
		"Duration",
		"CLAUDE.md",
		"[Read]",
		"[Write]",
		"[Bash]",
		"trajectory_summarize",
	}

	for _, r := range required {
		if !strings.Contains(output, r) {
			t.Errorf("output should contain %q", r)
		}
	}
}

func TestFormatTrajectoryWithOptions_NoPrompt(t *testing.T) {
	session := createTestSession()
	opts := FormatOptions{
		IncludeSummarizationPrompt: false,
		Verbose:                    true,
	}

	output := FormatTrajectoryWithOptions(session, opts)

	if strings.Contains(output, "trajectory_summarize") {
		t.Error("should not contain summarization prompt when disabled")
	}
}

func TestFormatTrajectoryWithOptions_NotVerbose(t *testing.T) {
	session := createTestSession()
	opts := FormatOptions{
		IncludeSummarizationPrompt: true,
		Verbose:                    false,
	}

	output := FormatTrajectoryWithOptions(session, opts)

	if strings.Contains(output, "Working Directory") {
		t.Error("should not contain working directory when not verbose")
	}
	if strings.Contains(output, "Loaded Context") {
		t.Error("should not contain loaded context when not verbose")
	}
}

func TestFormatTrajectoryWithOptions_MaxSteps(t *testing.T) {
	session := createTestSession()
	opts := FormatOptions{
		IncludeSummarizationPrompt: false,
		MaxSteps:                   2,
		Verbose:                    false,
	}

	output := FormatTrajectoryWithOptions(session, opts)

	// Should mention showing first 2 of 4
	if !strings.Contains(output, "Showing first 2 of 4") {
		t.Error("should indicate truncated steps")
	}

	// Should only show Read steps (first 2)
	if strings.Contains(output, "[Write]") {
		t.Error("should not show Write step when truncated to 2")
	}
}

func TestFormatTrajectoryLongSession(t *testing.T) {
	session := createTestSession()

	// Add many steps
	session.Steps = make([]types.TrajectoryStep, 60)
	for i := range session.Steps {
		session.Steps[i] = types.TrajectoryStep{
			ToolName:     "Read",
			InputSummary: "file.go",
		}
	}
	// Add a Write step in the middle
	session.Steps[30].ToolName = "Write"
	session.Steps[30].InputSummary = "important.go"

	output := FormatTrajectoryForSummarization(session)

	// Should mention elided steps
	if !strings.Contains(output, "elided") {
		t.Error("should mention elided steps for long sessions")
	}

	// Should include the Write step from the middle
	if !strings.Contains(output, "important.go") {
		t.Error("should include important Write steps from middle")
	}
}

func TestFormatTrajectoryWithOutcome(t *testing.T) {
	session := createTestSession()
	session.Status = types.StatusScored
	session.Outcome = &types.Outcome{
		Score:    0.85,
		Notes:    "Good implementation",
		ScoredAt: time.Now(),
	}

	output := FormatTrajectoryForSummarization(session)

	if !strings.Contains(output, "0.85") {
		t.Error("should contain score")
	}
	if !strings.Contains(output, "Good implementation") {
		t.Error("should contain outcome notes")
	}
}

func TestFormatCompactTrajectory(t *testing.T) {
	session := createTestSession()
	session.Summary = "Implemented recursive fibonacci with memoization"
	session.Outcome = &types.Outcome{Score: 0.9}
	session.Status = types.StatusScored

	output := FormatCompactTrajectory(session)

	// Check key elements
	if !strings.Contains(output, "fibonacci") {
		t.Error("should contain task")
	}
	if !strings.Contains(output, "Steps: 4") {
		t.Error("should contain step count")
	}
	if !strings.Contains(output, "Score: 0.90") {
		t.Error("should contain score")
	}
	if !strings.Contains(output, "Read:2") {
		t.Error("should contain tool counts")
	}
	if !strings.Contains(output, "recursive fibonacci") {
		t.Error("should contain summary")
	}
}

func TestSelectRelevantSteps(t *testing.T) {
	// Create 60 steps
	steps := make([]types.TrajectoryStep, 60)
	for i := range steps {
		steps[i] = types.TrajectoryStep{
			ToolName:     "Read",
			InputSummary: "file.go",
		}
	}

	// Add important steps in the middle
	steps[20].ToolName = "Write"
	steps[20].InputSummary = "write1.go"
	steps[30].ToolName = "Edit"
	steps[30].InputSummary = "edit1.go"
	steps[40].ToolName = "Bash"
	steps[40].InputSummary = "go test ./..."

	selected := selectRelevantSteps(steps)

	// Should have first 5 + last 5 + important middle steps
	if len(selected) < 10 {
		t.Errorf("should have at least 10 steps, got %d", len(selected))
	}

	// Check that important steps are included
	hasWrite := false
	hasEdit := false
	hasBash := false
	for _, s := range selected {
		if s.InputSummary == "write1.go" {
			hasWrite = true
		}
		if s.InputSummary == "edit1.go" {
			hasEdit = true
		}
		if s.InputSummary == "go test ./..." {
			hasBash = true
		}
	}

	if !hasWrite {
		t.Error("should include Write step")
	}
	if !hasEdit {
		t.Error("should include Edit step")
	}
	if !hasBash {
		t.Error("should include significant Bash step")
	}
}

func TestSelectRelevantStepsShortSession(t *testing.T) {
	steps := []types.TrajectoryStep{
		{ToolName: "Read", InputSummary: "file1.go"},
		{ToolName: "Read", InputSummary: "file2.go"},
		{ToolName: "Write", InputSummary: "output.go"},
	}

	selected := selectRelevantSteps(steps)

	if len(selected) != 3 {
		t.Errorf("should return all steps for short sessions, got %d", len(selected))
	}
}

func TestIsSignificantBashCommand(t *testing.T) {
	tests := []struct {
		cmd      string
		expected bool
	}{
		{"go test ./...", true},
		{"git commit -m 'fix'", true},
		{"npm install", true},
		{"docker build .", true},
		{"ls -la", false},
		{"cat file.txt", false},
		{"echo hello", false},
		{"GO TEST ./...", true}, // case insensitive
	}

	for _, tc := range tests {
		result := isSignificantBashCommand(tc.cmd)
		if result != tc.expected {
			t.Errorf("isSignificantBashCommand(%q) = %v, want %v", tc.cmd, result, tc.expected)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "he..."},
		{"hello", 5, "hello"},
		{"hello", 3, "hel"},
		{"ab", 10, "ab"},
		{"", 5, ""},
	}

	for _, tc := range tests {
		result := truncate(tc.input, tc.maxLen)
		if result != tc.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.input, tc.maxLen, result, tc.expected)
		}
	}
}

func TestTruncateStepSummary(t *testing.T) {
	// Test newline removal
	input := "line1\nline2\nline3"
	result := truncateStepSummary(input)
	if strings.Contains(result, "\n") {
		t.Error("should remove newlines")
	}

	// Test space collapsing
	input = "word1   word2    word3"
	result = truncateStepSummary(input)
	if strings.Contains(result, "  ") {
		t.Error("should collapse multiple spaces")
	}

	// Test truncation
	input = strings.Repeat("x", 200)
	result = truncateStepSummary(input)
	if len(result) > 100 {
		t.Error("should truncate to 100 chars")
	}
}

func TestFormatDuration(t *testing.T) {
	now := time.Now()

	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{2 * time.Hour, "2h 0m"},
		{65 * time.Minute, "1h 5m"},
	}

	for _, tc := range tests {
		end := now.Add(tc.duration)
		result := formatDuration(now, &end)
		if result != tc.expected {
			t.Errorf("formatDuration for %v = %q, want %q", tc.duration, result, tc.expected)
		}
	}
}
