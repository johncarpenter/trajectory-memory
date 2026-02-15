// Package types defines the core data structures for trajectory memory.
package types

import (
	"time"
)

// Session represents a single trajectory recording session.
type Session struct {
	ID            string           `json:"id"`              // ULID
	TaskPrompt    string           `json:"task_prompt"`     // what the user asked
	WorkingDir    string           `json:"working_dir"`     // pwd at session start
	ClaudeMDHash  string           `json:"claude_md_hash"`  // hash of active CLAUDE.md
	LoadedContext []string         `json:"loaded_context"`  // .md files read during session
	Steps         []TrajectoryStep `json:"steps"`           // ordered tool invocations
	Summary       string           `json:"summary"`         // post-hoc model summarization
	Outcome       *Outcome         `json:"outcome"`         // score + notes (nil if unscored)
	Tags          []string         `json:"tags"`            // user or auto-assigned tags
	Strategy      string           `json:"strategy"`        // strategy profile used (for bandit, future)
	StartedAt     time.Time        `json:"started_at"`
	CompletedAt   *time.Time       `json:"completed_at"`
	Status        SessionStatus    `json:"status"` // "recording", "completed", "scored"
}

// SessionStatus represents the state of a session.
type SessionStatus string

const (
	StatusRecording SessionStatus = "recording"
	StatusCompleted SessionStatus = "completed"
	StatusScored    SessionStatus = "scored"
)

// TrajectoryStep represents a single tool invocation within a session.
type TrajectoryStep struct {
	Timestamp     time.Time `json:"timestamp"`
	ToolName      string    `json:"tool_name"`      // Read, Write, Bash, TodoWrite, etc.
	InputSummary  string    `json:"input_summary"`  // truncated to 500 chars
	OutputSummary string    `json:"output_summary"` // truncated to 500 chars
	DurationMs    int64     `json:"duration_ms"`    // if measurable
}

// Outcome represents the scoring result for a session.
type Outcome struct {
	Score    float64   `json:"score"`     // 0.0 to 1.0
	Notes    string    `json:"notes"`     // free-text user notes
	ScoredAt time.Time `json:"scored_at"`
}

// SessionMetadata is a lightweight representation for listing sessions.
type SessionMetadata struct {
	ID         string    `json:"id"`
	TaskPrompt string    `json:"task_prompt"` // first 200 chars
	Score      *float64  `json:"score"`
	StepCount  int       `json:"step_count"`
	Tags       []string  `json:"tags"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	Summary    string    `json:"summary,omitempty"`
}

// MaxInputSummaryLen is the maximum length for input summaries.
const MaxInputSummaryLen = 500

// MaxOutputSummaryLen is the maximum length for output summaries.
const MaxOutputSummaryLen = 500

// MaxTaskPromptMetadataLen is the maximum task prompt length in metadata.
const MaxTaskPromptMetadataLen = 200

// TruncateString truncates a string to the specified length.
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// ToMetadata converts a Session to SessionMetadata.
func (s *Session) ToMetadata() SessionMetadata {
	meta := SessionMetadata{
		ID:         s.ID,
		TaskPrompt: TruncateString(s.TaskPrompt, MaxTaskPromptMetadataLen),
		StepCount:  len(s.Steps),
		Tags:       s.Tags,
		Status:     string(s.Status),
		StartedAt:  s.StartedAt,
		Summary:    s.Summary,
	}
	if s.Outcome != nil {
		meta.Score = &s.Outcome.Score
	}
	return meta
}

// OptimizationTarget represents a section in a markdown file that can be optimized.
type OptimizationTarget struct {
	FilePath    string `json:"file_path"`
	Tag         string `json:"tag"`
	MinSessions int    `json:"min_sessions"`
	StartLine   int    `json:"start_line"` // line number of start marker (1-indexed)
	EndLine     int    `json:"end_line"`   // line number of end marker (1-indexed)
	Content     string `json:"content"`    // current content between markers
}

// OptimizationRecord tracks an optimization proposal and its lifecycle.
type OptimizationRecord struct {
	ID              string     `json:"id"`
	TargetFile      string     `json:"target_file"`
	Tag             string     `json:"tag"`
	SessionsUsed    int        `json:"sessions_used"`
	AvgScoreHigh    float64    `json:"avg_score_high"`
	AvgScoreLow     float64    `json:"avg_score_low"`
	PreviousContent string     `json:"previous_content"`
	NewContent      string     `json:"new_content"`
	Diff            string     `json:"diff"`
	Status          string     `json:"status"` // "proposed", "accepted", "rejected", "rolled_back"
	CreatedAt       time.Time  `json:"created_at"`
	AppliedAt       *time.Time `json:"applied_at"`
	RolledBackAt    *time.Time `json:"rolled_back_at"`
}

// OptimizationStatus constants
const (
	OptStatusProposed   = "proposed"
	OptStatusAccepted   = "accepted"
	OptStatusRejected   = "rejected"
	OptStatusRolledBack = "rolled_back"
)

// TrajectoryAnalysis contains the analysis results for a set of trajectories.
type TrajectoryAnalysis struct {
	Tag                  string           `json:"tag"`
	TotalSessions        int              `json:"total_sessions"`
	HighScoreSessions    int              `json:"high_score_sessions"`
	LowScoreSessions     int              `json:"low_score_sessions"`
	AvgScoreHigh         float64          `json:"avg_score_high"`
	AvgScoreLow          float64          `json:"avg_score_low"`
	HighScorePatterns    []string         `json:"high_score_patterns"`
	LowScoreAntiPatterns []string         `json:"low_score_anti_patterns"`
	RecommendedPractices []string         `json:"recommended_practices"`
	CuratedExamples      []CuratedExample `json:"curated_examples"`
}

// CuratedExample represents a selected trajectory for use as a few-shot example.
type CuratedExample struct {
	SessionID   string  `json:"session_id"`
	TaskPrompt  string  `json:"task_prompt"`
	Summary     string  `json:"summary"`
	Score       float64 `json:"score"`
	Notes       string  `json:"notes"`       // user notes from outcome
	WhySelected string  `json:"why_selected"` // rationale for selection
}

// ExamplesTarget represents a section for curated examples in markdown.
type ExamplesTarget struct {
	FilePath        string `json:"file_path"`
	Tag             string `json:"tag"`
	MaxExamples     int    `json:"max_examples"`
	IncludeNegative bool   `json:"include_negative"`
	StartLine       int    `json:"start_line"`
	EndLine         int    `json:"end_line"`
	Content         string `json:"content"`
}

// StrategiesTarget represents a section containing strategy definitions in markdown.
type StrategiesTarget struct {
	FilePath  string `json:"file_path"`
	Tag       string `json:"tag"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content"`
}

// Strategy represents a named approach for a task type.
type Strategy struct {
	Name           string  `json:"name"`
	Description    string  `json:"description,omitempty"`
	ApproachPrompt string  `json:"approach_prompt"`
	AvgScore       float64 `json:"avg_score,omitempty"`
	SessionCount   int     `json:"session_count,omitempty"`
}

// StrategyUsage records which strategy was used for a session.
type StrategyUsage struct {
	Tag          string    `json:"tag"`
	StrategyName string    `json:"strategy_name"`
	SessionID    string    `json:"session_id"`
	Score        float64   `json:"score,omitempty"`
	UsedAt       time.Time `json:"used_at"`
}

// StrategiesAnalysis contains analysis of strategy performance.
type StrategiesAnalysis struct {
	Tag               string     `json:"tag"`
	Strategies        []Strategy `json:"strategies"`
	TotalSessions     int        `json:"total_sessions"`
	BestStrategy      string     `json:"best_strategy,omitempty"`
	RecommendedNext   string     `json:"recommended_next,omitempty"`
	RotationSuggested bool       `json:"rotation_suggested"`
}

// StrategySelectionMode defines how to select a strategy.
type StrategySelectionMode string

const (
	// StrategyModeExplicit means user specifies which strategy to use.
	StrategyModeExplicit StrategySelectionMode = "explicit"
	// StrategyModeRecommend means AI recommends based on past performance.
	StrategyModeRecommend StrategySelectionMode = "recommend"
	// StrategyModeRotate means cycle through strategies for exploration.
	StrategyModeRotate StrategySelectionMode = "rotate"
)
