package mcp

// Tool input/output structures for trajectory memory

// TrajectoryStartInput is the input for trajectory_start.
type TrajectoryStartInput struct {
	TaskPrompt string   `json:"task_prompt"`
	Tags       []string `json:"tags,omitempty"`
}

// TrajectoryStartOutput is the output for trajectory_start.
type TrajectoryStartOutput struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

// TrajectoryStopInput is the input for trajectory_stop.
type TrajectoryStopInput struct {
	Score         *float64 `json:"score,omitempty"`
	Notes         string   `json:"notes,omitempty"`
	AutoSummarize *bool    `json:"auto_summarize,omitempty"`
}

// TrajectoryStatusOutput is the output for trajectory_status.
type TrajectoryStatusOutput struct {
	Active          bool   `json:"active"`
	SessionID       string `json:"session_id,omitempty"`
	StepCount       int    `json:"step_count"`
	DurationSeconds int    `json:"duration_seconds"`
}

// TrajectorySearchInput is the input for trajectory_search.
type TrajectorySearchInput struct {
	Query    string   `json:"query"`
	Limit    int      `json:"limit,omitempty"`
	MinScore *float64 `json:"min_score,omitempty"`
}

// TrajectorySearchResult is a single search result.
type TrajectorySearchResult struct {
	SessionID  string   `json:"session_id"`
	TaskPrompt string   `json:"task_prompt"`
	Summary    string   `json:"summary"`
	Score      *float64 `json:"score,omitempty"`
	StepCount  int      `json:"step_count"`
	Tags       []string `json:"tags"`
	StartedAt  string   `json:"started_at"`
}

// TrajectoryListInput is the input for trajectory_list.
type TrajectoryListInput struct {
	Limit int `json:"limit,omitempty"`
}

// TrajectoryScoreInput is the input for trajectory_score.
type TrajectoryScoreInput struct {
	SessionID string  `json:"session_id"`
	Score     float64 `json:"score"`
	Notes     string  `json:"notes,omitempty"`
}

// TrajectorySummarizeInput is the input for trajectory_summarize.
type TrajectorySummarizeInput struct {
	SessionID string `json:"session_id"`
	Summary   string `json:"summary"`
}

// TrajectoryOptimizeProposeInput is the input for trajectory_optimize_propose.
type TrajectoryOptimizeProposeInput struct {
	FilePath string `json:"file_path"`
	Tag      string `json:"tag,omitempty"`
}

// TrajectoryOptimizeSaveInput is the input for trajectory_optimize_save.
type TrajectoryOptimizeSaveInput struct {
	RecordID        string `json:"record_id"`
	FilePath        string `json:"file_path"`
	Tag             string `json:"tag"`
	PreviousContent string `json:"previous_content"`
	Content         string `json:"content"`
}

// TrajectoryOptimizeApplyInput is the input for trajectory_optimize_apply.
type TrajectoryOptimizeApplyInput struct {
	RecordID string `json:"record_id"`
}

// TrajectoryOptimizeRollbackInput is the input for trajectory_optimize_rollback.
type TrajectoryOptimizeRollbackInput struct {
	RecordID string `json:"record_id"`
}

// TrajectoryOptimizeHistoryInput is the input for trajectory_optimize_history.
type TrajectoryOptimizeHistoryInput struct {
	FilePath string `json:"file_path,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// TrajectoryCurateExamplesInput is the input for trajectory_curate_examples.
type TrajectoryCurateExamplesInput struct {
	Tag             string `json:"tag"`
	MaxExamples     int    `json:"max_examples,omitempty"`
	IncludeNegative bool   `json:"include_negative,omitempty"`
}

// TrajectoryCurateApplyInput is the input for trajectory_curate_apply.
type TrajectoryCurateApplyInput struct {
	FilePath string `json:"file_path"`
	Tag      string `json:"tag"`
	Content  string `json:"content"`
}

// TrajectoryTriggerStatusOutput is the output for trajectory_trigger_status.
type TrajectoryTriggerStatusOutput struct {
	Config              TriggerConfigOutput    `json:"config"`
	PendingOptimizations []PendingOptOutput    `json:"pending_optimizations"`
}

// TriggerConfigOutput represents trigger config for output.
type TriggerConfigOutput struct {
	SessionThreshold int      `json:"session_threshold"`
	MinScoreGap      float64  `json:"min_score_gap"`
	Enabled          bool     `json:"enabled"`
	WatchFiles       []string `json:"watch_files"`
}

// PendingOptOutput represents a pending optimization for output.
type PendingOptOutput struct {
	FilePath       string  `json:"file_path"`
	Tag            string  `json:"tag"`
	SessionsSince  int     `json:"sessions_since"`
	RecentAvgScore float64 `json:"recent_avg_score"`
	BaselineAvg    float64 `json:"baseline_avg"`
	Reason         string  `json:"reason"`
}

// TrajectoryTriggerConfigureInput is the input for trajectory_trigger_configure.
type TrajectoryTriggerConfigureInput struct {
	Enabled          *bool    `json:"enabled,omitempty"`
	SessionThreshold *int     `json:"session_threshold,omitempty"`
	MinScoreGap      *float64 `json:"min_score_gap,omitempty"`
	WatchFiles       []string `json:"watch_files,omitempty"`
}

// GetToolDefinitions returns all trajectory memory tool definitions.
func GetToolDefinitions() []Tool {
	minScore := 0.0
	maxScore := 1.0

	return []Tool{
		{
			Name:        "trajectory_start",
			Description: "Start recording a new trajectory session. Captures tool invocations for later analysis and learning.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"task_prompt": {
						Type:        "string",
						Description: "Description of the task being performed",
					},
					"tags": {
						Type:        "array",
						Description: "Optional tags for categorizing the session",
						Items:       &Property{Type: "string"},
					},
				},
				Required: []string{"task_prompt"},
			},
		},
		{
			Name:        "trajectory_stop",
			Description: "Stop the current recording session. Returns the trajectory formatted for summarization.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"score": {
						Type:        "number",
						Description: "Optional immediate score (0.0 to 1.0) for the session outcome",
						Minimum:     &minScore,
						Maximum:     &maxScore,
					},
					"notes": {
						Type:        "string",
						Description: "Optional notes about the session outcome",
					},
					"auto_summarize": {
						Type:        "boolean",
						Description: "Include summarization prompt in output (default: true)",
						Default:     true,
					},
				},
			},
		},
		{
			Name:        "trajectory_status",
			Description: "Check the current recording status.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "trajectory_search",
			Description: "Search past trajectory sessions by keyword. Returns matching sessions with summaries and scores.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "Search query to match against task prompts, summaries, and tags",
					},
					"limit": {
						Type:        "number",
						Description: "Maximum number of results (default: 5)",
						Default:     float64(5),
					},
					"min_score": {
						Type:        "number",
						Description: "Minimum score filter (0.0 to 1.0)",
						Minimum:     &minScore,
						Maximum:     &maxScore,
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "trajectory_list",
			Description: "List recent trajectory sessions.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"limit": {
						Type:        "number",
						Description: "Maximum number of sessions to return (default: 10)",
						Default:     float64(10),
					},
				},
			},
		},
		{
			Name:        "trajectory_score",
			Description: "Score or re-score a past trajectory session.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {
						Type:        "string",
						Description: "The session ID to score",
					},
					"score": {
						Type:        "number",
						Description: "Score value (0.0 to 1.0)",
						Minimum:     &minScore,
						Maximum:     &maxScore,
					},
					"notes": {
						Type:        "string",
						Description: "Optional notes about the scoring",
					},
				},
				Required: []string{"session_id", "score"},
			},
		},
		{
			Name:        "trajectory_summarize",
			Description: "Store a summary for a trajectory session. Call this after trajectory_stop with the generated summary.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {
						Type:        "string",
						Description: "The session ID to summarize",
					},
					"summary": {
						Type:        "string",
						Description: "The summary text to store",
					},
				},
				Required: []string{"session_id", "summary"},
			},
		},
		// Optimization tools
		{
			Name:        "trajectory_optimize_propose",
			Description: "Analyze trajectories and propose optimized content for a CLAUDE.md section marked with trajectory-optimize markers. Returns a meta-prompt with analysis data for generating optimized instructions.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file_path": {
						Type:        "string",
						Description: "Path to the markdown file containing optimization markers",
					},
					"tag": {
						Type:        "string",
						Description: "Specific tag to optimize. If omitted, analyzes all targets in the file",
					},
				},
				Required: []string{"file_path"},
			},
		},
		{
			Name:        "trajectory_optimize_save",
			Description: "Save a proposed optimization with the generated content. Call this after generating optimized instructions from trajectory_optimize_propose.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"record_id": {
						Type:        "string",
						Description: "The optimization record ID from the propose step",
					},
					"file_path": {
						Type:        "string",
						Description: "Path to the target file",
					},
					"tag": {
						Type:        "string",
						Description: "The optimization target tag",
					},
					"previous_content": {
						Type:        "string",
						Description: "The original content being replaced",
					},
					"content": {
						Type:        "string",
						Description: "The new optimized content to save",
					},
				},
				Required: []string{"record_id", "file_path", "tag", "previous_content", "content"},
			},
		},
		{
			Name:        "trajectory_optimize_apply",
			Description: "Apply a proposed optimization to the target file.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"record_id": {
						Type:        "string",
						Description: "The optimization record ID to apply",
					},
				},
				Required: []string{"record_id"},
			},
		},
		{
			Name:        "trajectory_optimize_rollback",
			Description: "Revert a previously applied optimization to restore the original content.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"record_id": {
						Type:        "string",
						Description: "The optimization record ID to rollback",
					},
				},
				Required: []string{"record_id"},
			},
		},
		{
			Name:        "trajectory_optimize_history",
			Description: "List optimization history, optionally filtered by file and/or tag.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file_path": {
						Type:        "string",
						Description: "Filter by file path",
					},
					"tag": {
						Type:        "string",
						Description: "Filter by tag",
					},
					"limit": {
						Type:        "number",
						Description: "Maximum number of records (default: 10)",
						Default:     float64(10),
					},
				},
			},
		},
		{
			Name:        "trajectory_curate_examples",
			Description: "Curate the best trajectory examples for a tag, formatted for CLAUDE.md injection.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"tag": {
						Type:        "string",
						Description: "The tag to curate examples for",
					},
					"max_examples": {
						Type:        "number",
						Description: "Maximum positive examples (default: 3)",
						Default:     float64(3),
					},
					"include_negative": {
						Type:        "boolean",
						Description: "Include a negative example (default: true)",
						Default:     true,
					},
				},
				Required: []string{"tag"},
			},
		},
		{
			Name:        "trajectory_curate_apply",
			Description: "Apply curated examples to a file's trajectory-examples markers.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file_path": {
						Type:        "string",
						Description: "Path to the target file",
					},
					"tag": {
						Type:        "string",
						Description: "The tag to match",
					},
					"content": {
						Type:        "string",
						Description: "The curated examples content to apply",
					},
				},
				Required: []string{"file_path", "tag", "content"},
			},
		},
		{
			Name:        "trajectory_trigger_status",
			Description: "Check trigger configuration and any pending optimizations.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "trajectory_trigger_configure",
			Description: "Configure auto-optimization triggers.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"enabled": {
						Type:        "boolean",
						Description: "Enable or disable auto-optimization triggers",
					},
					"session_threshold": {
						Type:        "number",
						Description: "Number of new sessions before triggering optimization",
					},
					"min_score_gap": {
						Type:        "number",
						Description: "Minimum score improvement potential before triggering",
					},
					"watch_files": {
						Type:        "array",
						Description: "Files to monitor for optimization targets",
						Items:       &Property{Type: "string"},
					},
				},
			},
		},
	}
}
