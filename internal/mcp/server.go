package mcp

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/johncarpenter/trajectory-memory/internal/ingestion"
	"github.com/johncarpenter/trajectory-memory/internal/optimizer"
	"github.com/johncarpenter/trajectory-memory/internal/store"
	"github.com/johncarpenter/trajectory-memory/internal/summarize"
	"github.com/johncarpenter/trajectory-memory/internal/types"
)

const (
	protocolVersion = "2024-11-05"
	serverName      = "trajectory-memory"
)

// Server implements the MCP protocol over stdio.
type Server struct {
	store           store.Store
	boltStore       *store.BoltStore // For optimization features
	optimizer       *optimizer.Optimizer
	ingestionServer *ingestion.Server
	version         string
	reader          *bufio.Reader
	writer          io.Writer
	socketPath      string
}

// NewServer creates a new MCP server.
func NewServer(s store.Store, socketPath, version string) *Server {
	srv := &Server{
		store:      s,
		version:    version,
		socketPath: socketPath,
		reader:     bufio.NewReader(os.Stdin),
		writer:     os.Stdout,
	}

	// If the store is a BoltStore, set up optimization features
	if bs, ok := s.(*store.BoltStore); ok {
		srv.boltStore = bs
		srv.optimizer = optimizer.NewOptimizer(bs)
	}

	return srv
}

// SetIO allows setting custom IO for testing.
func (s *Server) SetIO(r io.Reader, w io.Writer) {
	s.reader = bufio.NewReader(r)
	s.writer = w
}

// Run starts the MCP server and processes requests.
func (s *Server) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := s.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, ParseError, "Parse error", nil)
			continue
		}

		if req.JSONRPC != "2.0" {
			s.sendError(req.ID, InvalidRequest, "Invalid JSON-RPC version", nil)
			continue
		}

		s.handleRequest(&req)
	}
}

func (s *Server) handleRequest(req *Request) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "initialized":
		// Notification, no response needed
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(req)
	case "ping":
		s.sendResult(req.ID, map[string]interface{}{})
	default:
		s.sendError(req.ID, MethodNotFound, fmt.Sprintf("Method not found: %s", req.Method), nil)
	}
}

func (s *Server) handleInitialize(req *Request) {
	result := InitializeResult{
		ProtocolVersion: protocolVersion,
		ServerInfo: ServerInfo{
			Name:    serverName,
			Version: s.version,
		},
		Capabilities: Capabilities{
			Tools: map[string]interface{}{},
		},
	}
	s.sendResult(req.ID, result)
}

func (s *Server) handleToolsList(req *Request) {
	result := ToolsListResult{
		Tools: GetToolDefinitions(),
	}
	s.sendResult(req.ID, result)
}

func (s *Server) handleToolsCall(req *Request) {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, InvalidParams, "Invalid params", nil)
		return
	}

	var result ToolCallResult
	var err error

	switch params.Name {
	case "trajectory_start":
		result, err = s.handleTrajectoryStart(params.Arguments)
	case "trajectory_stop":
		result, err = s.handleTrajectoryStop(params.Arguments)
	case "trajectory_status":
		result, err = s.handleTrajectoryStatus()
	case "trajectory_search":
		result, err = s.handleTrajectorySearch(params.Arguments)
	case "trajectory_list":
		result, err = s.handleTrajectoryList(params.Arguments)
	case "trajectory_score":
		result, err = s.handleTrajectoryScore(params.Arguments)
	case "trajectory_summarize":
		result, err = s.handleTrajectorySummarize(params.Arguments)
	case "trajectory_optimize_propose":
		result, err = s.handleOptimizePropose(params.Arguments)
	case "trajectory_optimize_save":
		result, err = s.handleOptimizeSave(params.Arguments)
	case "trajectory_optimize_apply":
		result, err = s.handleOptimizeApply(params.Arguments)
	case "trajectory_optimize_rollback":
		result, err = s.handleOptimizeRollback(params.Arguments)
	case "trajectory_optimize_history":
		result, err = s.handleOptimizeHistory(params.Arguments)
	case "trajectory_curate_examples":
		result, err = s.handleCurateExamples(params.Arguments)
	case "trajectory_curate_apply":
		result, err = s.handleCurateApply(params.Arguments)
	case "trajectory_trigger_status":
		result, err = s.handleTriggerStatus()
	case "trajectory_trigger_configure":
		result, err = s.handleTriggerConfigure(params.Arguments)
	case "trajectory_strategies_list":
		result, err = s.handleStrategiesList(params.Arguments)
	case "trajectory_strategies_select":
		result, err = s.handleStrategiesSelect(params.Arguments)
	case "trajectory_strategies_record":
		result, err = s.handleStrategiesRecord(params.Arguments)
	case "trajectory_strategies_analyze":
		result, err = s.handleStrategiesAnalyze(params.Arguments)
	default:
		s.sendError(req.ID, InvalidParams, fmt.Sprintf("Unknown tool: %s", params.Name), nil)
		return
	}

	if err != nil {
		result = ToolCallResult{
			Content: []ContentBlock{{Type: "text", Text: err.Error()}},
			IsError: true,
		}
	}

	s.sendResult(req.ID, result)
}

func (s *Server) handleTrajectoryStart(args json.RawMessage) (ToolCallResult, error) {
	var input TrajectoryStartInput
	if err := json.Unmarshal(args, &input); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
	}

	if input.TaskPrompt == "" {
		return ToolCallResult{}, fmt.Errorf("task_prompt is required")
	}

	// Check if already recording
	if _, err := s.store.GetActiveSession(); err == nil {
		return ToolCallResult{}, fmt.Errorf("a session is already recording - stop it first")
	}

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		wd = ""
	}

	// Hash CLAUDE.md if present
	claudeMDHash := ""
	claudeMDPath := filepath.Join(wd, "CLAUDE.md")
	if data, err := os.ReadFile(claudeMDPath); err == nil {
		hash := sha256.Sum256(data)
		claudeMDHash = hex.EncodeToString(hash[:])
	}

	// Create session
	session := &types.Session{
		ID:            store.NewULID(),
		TaskPrompt:    input.TaskPrompt,
		WorkingDir:    wd,
		ClaudeMDHash:  claudeMDHash,
		LoadedContext: []string{},
		Steps:         []types.TrajectoryStep{},
		Tags:          input.Tags,
		Status:        types.StatusRecording,
		StartedAt:     time.Now(),
	}

	if err := s.store.CreateSession(session); err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to create session: %w", err)
	}

	if err := s.store.SetActiveSession(session.ID); err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to set active session: %w", err)
	}

	// Start ingestion server if not already running
	if s.ingestionServer == nil {
		s.ingestionServer = ingestion.NewServer(s.store, s.socketPath)
	}
	if !s.ingestionServer.IsRunning() {
		if err := s.ingestionServer.Start(context.Background()); err != nil {
			log.Printf("Warning: failed to start ingestion server: %v", err)
		}
	}

	output := TrajectoryStartOutput{
		SessionID: session.ID,
		Message:   "Recording started",
	}

	jsonOutput, _ := json.Marshal(output)
	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: string(jsonOutput)}},
	}, nil
}

func (s *Server) handleTrajectoryStop(args json.RawMessage) (ToolCallResult, error) {
	var input TrajectoryStopInput
	if len(args) > 0 {
		if err := json.Unmarshal(args, &input); err != nil {
			return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
		}
	}

	// Get active session
	session, err := s.store.GetActiveSession()
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("no active session to stop")
	}

	// Update session status
	session.Status = types.StatusCompleted
	now := time.Now()
	session.CompletedAt = &now

	// Set score if provided
	if input.Score != nil {
		session.Outcome = &types.Outcome{
			Score:    *input.Score,
			Notes:    input.Notes,
			ScoredAt: now,
		}
		session.Status = types.StatusScored
	}

	if err := s.store.UpdateSession(session); err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to update session: %w", err)
	}

	// Clear active session
	if err := s.store.ClearActiveSession(); err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to clear active session: %w", err)
	}

	// Format trajectory for summarization
	autoSummarize := true
	if input.AutoSummarize != nil {
		autoSummarize = *input.AutoSummarize
	}

	opts := summarize.FormatOptions{
		IncludeSummarizationPrompt: autoSummarize,
		Verbose:                    true,
	}
	trajectory := summarize.FormatTrajectoryWithOptions(session, opts)

	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: trajectory}},
	}, nil
}

func (s *Server) handleTrajectoryStatus() (ToolCallResult, error) {
	session, err := s.store.GetActiveSession()

	output := TrajectoryStatusOutput{
		Active: err == nil,
	}

	if session != nil {
		output.SessionID = session.ID
		output.StepCount = len(session.Steps)
		output.DurationSeconds = int(time.Since(session.StartedAt).Seconds())
	}

	jsonOutput, _ := json.Marshal(output)
	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: string(jsonOutput)}},
	}, nil
}

func (s *Server) handleTrajectorySearch(args json.RawMessage) (ToolCallResult, error) {
	var input TrajectorySearchInput
	if err := json.Unmarshal(args, &input); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
	}

	if input.Query == "" {
		return ToolCallResult{}, fmt.Errorf("query is required")
	}

	limit := 5
	if input.Limit > 0 {
		limit = input.Limit
	}

	results, err := s.store.SearchSessions(input.Query, limit)
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("search failed: %w", err)
	}

	// Filter by min_score if specified
	if input.MinScore != nil {
		filtered := make([]types.SessionMetadata, 0)
		for _, r := range results {
			if r.Score != nil && *r.Score >= *input.MinScore {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// Convert to output format
	var searchResults []TrajectorySearchResult
	for _, r := range results {
		searchResults = append(searchResults, TrajectorySearchResult{
			SessionID:  r.ID,
			TaskPrompt: r.TaskPrompt,
			Summary:    r.Summary,
			Score:      r.Score,
			StepCount:  r.StepCount,
			Tags:       r.Tags,
			StartedAt:  r.StartedAt.Format(time.RFC3339),
		})
	}

	jsonOutput, _ := json.Marshal(searchResults)
	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: string(jsonOutput)}},
	}, nil
}

func (s *Server) handleTrajectoryList(args json.RawMessage) (ToolCallResult, error) {
	var input TrajectoryListInput
	if len(args) > 0 {
		json.Unmarshal(args, &input)
	}

	limit := 10
	if input.Limit > 0 {
		limit = input.Limit
	}

	results, err := s.store.ListSessions(limit, 0)
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("list failed: %w", err)
	}

	jsonOutput, _ := json.Marshal(results)
	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: string(jsonOutput)}},
	}, nil
}

func (s *Server) handleTrajectoryScore(args json.RawMessage) (ToolCallResult, error) {
	var input TrajectoryScoreInput
	if err := json.Unmarshal(args, &input); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
	}

	if input.SessionID == "" {
		return ToolCallResult{}, fmt.Errorf("session_id is required")
	}

	if input.Score < 0 || input.Score > 1 {
		return ToolCallResult{}, fmt.Errorf("score must be between 0.0 and 1.0")
	}

	outcome := types.Outcome{
		Score:    input.Score,
		Notes:    input.Notes,
		ScoredAt: time.Now(),
	}

	if err := s.store.SetOutcome(input.SessionID, outcome); err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to set outcome: %w", err)
	}

	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Session %s scored %.2f", input.SessionID, input.Score)}},
	}, nil
}

func (s *Server) handleTrajectorySummarize(args json.RawMessage) (ToolCallResult, error) {
	var input TrajectorySummarizeInput
	if err := json.Unmarshal(args, &input); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
	}

	if input.SessionID == "" {
		return ToolCallResult{}, fmt.Errorf("session_id is required")
	}
	if input.Summary == "" {
		return ToolCallResult{}, fmt.Errorf("summary is required")
	}

	session, err := s.store.GetSession(input.SessionID)
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("session not found: %w", err)
	}

	session.Summary = input.Summary
	if err := s.store.UpdateSession(session); err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to update session: %w", err)
	}

	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Summary stored for session %s", input.SessionID)}},
	}, nil
}

func (s *Server) handleOptimizePropose(args json.RawMessage) (ToolCallResult, error) {
	if s.optimizer == nil {
		return ToolCallResult{}, fmt.Errorf("optimization features not available")
	}

	var input TrajectoryOptimizeProposeInput
	if err := json.Unmarshal(args, &input); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
	}

	if input.FilePath == "" {
		return ToolCallResult{}, fmt.Errorf("file_path is required")
	}

	parser := optimizer.NewParser()

	// Find targets in the file
	targets, err := parser.FindTargets(input.FilePath)
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to parse file: %w", err)
	}

	if len(targets) == 0 {
		return ToolCallResult{}, fmt.Errorf("no optimization targets found in file")
	}

	// Filter by tag if specified
	if input.Tag != "" {
		var filtered []types.OptimizationTarget
		for _, t := range targets {
			if t.Tag == input.Tag {
				filtered = append(filtered, t)
			}
		}
		targets = filtered
		if len(targets) == 0 {
			return ToolCallResult{}, fmt.Errorf("no target found for tag: %s", input.Tag)
		}
	}

	// Generate proposals for each target
	var output strings.Builder
	for _, target := range targets {
		result, err := s.optimizer.Propose(target)
		if err != nil {
			output.WriteString(fmt.Sprintf("## Target: %s (SKIPPED)\n%v\n\n", target.Tag, err))
			continue
		}

		output.WriteString(result.Prompt)
		output.WriteString(fmt.Sprintf("\n---\n**Record ID:** %s\n", result.Record.ID))
		output.WriteString(fmt.Sprintf("**File:** %s\n", target.FilePath))
		output.WriteString(fmt.Sprintf("**Tag:** %s\n", target.Tag))
		output.WriteString(fmt.Sprintf("**Previous Content:** (saved for diff)\n\n"))
		output.WriteString("After generating optimized content, call `trajectory_optimize_save` with:\n")
		output.WriteString(fmt.Sprintf("- record_id: \"%s\"\n", result.Record.ID))
		output.WriteString(fmt.Sprintf("- file_path: \"%s\"\n", target.FilePath))
		output.WriteString(fmt.Sprintf("- tag: \"%s\"\n", target.Tag))
		output.WriteString("- previous_content: (the current content shown above)\n")
		output.WriteString("- content: (your generated optimized content)\n\n")
	}

	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: output.String()}},
	}, nil
}

func (s *Server) handleOptimizeSave(args json.RawMessage) (ToolCallResult, error) {
	if s.optimizer == nil {
		return ToolCallResult{}, fmt.Errorf("optimization features not available")
	}

	var input TrajectoryOptimizeSaveInput
	if err := json.Unmarshal(args, &input); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
	}

	record, err := s.optimizer.SaveProposal(
		input.RecordID,
		input.FilePath,
		input.Tag,
		input.PreviousContent,
		input.Content,
	)
	if err != nil {
		return ToolCallResult{}, err
	}

	var output strings.Builder
	output.WriteString("## Optimization Proposal Saved\n\n")
	output.WriteString(fmt.Sprintf("**Record ID:** %s\n", record.ID))
	output.WriteString(fmt.Sprintf("**Status:** %s\n\n", record.Status))
	output.WriteString("### Diff\n```diff\n")
	output.WriteString(record.Diff)
	output.WriteString("```\n\n")
	output.WriteString("To apply this optimization, call `trajectory_optimize_apply` with:\n")
	output.WriteString(fmt.Sprintf("- record_id: \"%s\"\n\n", record.ID))
	output.WriteString("To reject, call `trajectory_optimize_rollback` with the same record_id.\n")

	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: output.String()}},
	}, nil
}

func (s *Server) handleOptimizeApply(args json.RawMessage) (ToolCallResult, error) {
	if s.optimizer == nil {
		return ToolCallResult{}, fmt.Errorf("optimization features not available")
	}

	var input TrajectoryOptimizeApplyInput
	if err := json.Unmarshal(args, &input); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
	}

	if err := s.optimizer.Apply(input.RecordID); err != nil {
		return ToolCallResult{}, err
	}

	record, _ := s.optimizer.GetRecord(input.RecordID)

	var output strings.Builder
	output.WriteString("## Optimization Applied\n\n")
	output.WriteString(fmt.Sprintf("**Record ID:** %s\n", input.RecordID))
	if record != nil {
		output.WriteString(fmt.Sprintf("**File:** %s\n", record.TargetFile))
		output.WriteString(fmt.Sprintf("**Tag:** %s\n", record.Tag))
	}
	output.WriteString("\nThe optimization has been applied to the file.\n")
	output.WriteString("The previous version is stored for rollback if needed.\n")

	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: output.String()}},
	}, nil
}

func (s *Server) handleOptimizeRollback(args json.RawMessage) (ToolCallResult, error) {
	if s.optimizer == nil {
		return ToolCallResult{}, fmt.Errorf("optimization features not available")
	}

	var input TrajectoryOptimizeRollbackInput
	if err := json.Unmarshal(args, &input); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
	}

	if err := s.optimizer.Rollback(input.RecordID); err != nil {
		return ToolCallResult{}, err
	}

	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Optimization %s has been rolled back. Previous content restored.", input.RecordID)}},
	}, nil
}

func (s *Server) handleOptimizeHistory(args json.RawMessage) (ToolCallResult, error) {
	if s.optimizer == nil {
		return ToolCallResult{}, fmt.Errorf("optimization features not available")
	}

	var input TrajectoryOptimizeHistoryInput
	if len(args) > 0 {
		json.Unmarshal(args, &input)
	}

	limit := 10
	if input.Limit > 0 {
		limit = input.Limit
	}

	records, err := s.optimizer.History(input.FilePath, input.Tag, limit)
	if err != nil {
		return ToolCallResult{}, err
	}

	output := optimizer.FormatHistoryForCLI(records)

	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: output}},
	}, nil
}

func (s *Server) handleCurateExamples(args json.RawMessage) (ToolCallResult, error) {
	if s.boltStore == nil {
		return ToolCallResult{}, fmt.Errorf("curation features not available")
	}

	var input TrajectoryCurateExamplesInput
	if err := json.Unmarshal(args, &input); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
	}

	if input.Tag == "" {
		return ToolCallResult{}, fmt.Errorf("tag is required")
	}

	maxExamples := 3
	if input.MaxExamples > 0 {
		maxExamples = input.MaxExamples
	}

	// Use analyzer to get curated examples
	analyzer := optimizer.NewAnalyzer(s.boltStore)
	analysis, err := analyzer.Analyze(input.Tag, 3) // Low minimum for curation
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("analysis failed: %w", err)
	}

	// Limit to maxExamples
	examples := analysis.CuratedExamples
	if len(examples) > maxExamples+1 { // +1 for potential negative example
		examples = examples[:maxExamples+1]
	}

	// Format for markdown
	content := formatCuratedExamples(examples, input.IncludeNegative)

	// Save curated examples
	if err := s.boltStore.SaveCuratedExamples(input.Tag, examples); err != nil {
		log.Printf("Warning: failed to save curated examples: %v", err)
	}

	var output strings.Builder
	output.WriteString("## Curated Examples for \"")
	output.WriteString(input.Tag)
	output.WriteString("\"\n\n")
	output.WriteString(content)
	output.WriteString("\n---\n\nTo apply these examples to a file, call `trajectory_curate_apply` with:\n")
	output.WriteString("- file_path: <path to your CLAUDE.md>\n")
	output.WriteString(fmt.Sprintf("- tag: \"%s\"\n", input.Tag))
	output.WriteString("- content: (the formatted content above)\n")

	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: output.String()}},
	}, nil
}

func (s *Server) handleCurateApply(args json.RawMessage) (ToolCallResult, error) {
	var input TrajectoryCurateApplyInput
	if err := json.Unmarshal(args, &input); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
	}

	parser := optimizer.NewParser()

	// Find examples targets
	targets, err := parser.FindExamplesTargets(input.FilePath)
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to parse file: %w", err)
	}

	// Find matching target
	var target *types.ExamplesTarget
	for _, t := range targets {
		if t.Tag == input.Tag {
			target = &t
			break
		}
	}

	if target == nil {
		return ToolCallResult{}, fmt.Errorf("no examples target found for tag: %s", input.Tag)
	}

	// Replace content
	if err := parser.ReplaceExamplesTarget(input.FilePath, *target, input.Content); err != nil {
		return ToolCallResult{}, err
	}

	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Curated examples applied to %s for tag '%s'", input.FilePath, input.Tag)}},
	}, nil
}

func (s *Server) handleTriggerStatus() (ToolCallResult, error) {
	if s.boltStore == nil {
		return ToolCallResult{}, fmt.Errorf("trigger features not available")
	}

	config, err := s.boltStore.GetTriggerConfig()
	if err != nil {
		return ToolCallResult{}, err
	}

	output := TrajectoryTriggerStatusOutput{
		Config: TriggerConfigOutput{
			SessionThreshold: config.SessionThreshold,
			MinScoreGap:      config.MinScoreGap,
			Enabled:          config.Enabled,
			WatchFiles:       config.WatchFiles,
		},
		PendingOptimizations: []PendingOptOutput{}, // TODO: Implement pending check
	}

	jsonOutput, _ := json.Marshal(output)
	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: string(jsonOutput)}},
	}, nil
}

func (s *Server) handleTriggerConfigure(args json.RawMessage) (ToolCallResult, error) {
	if s.boltStore == nil {
		return ToolCallResult{}, fmt.Errorf("trigger features not available")
	}

	var input TrajectoryTriggerConfigureInput
	if err := json.Unmarshal(args, &input); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
	}

	config, err := s.boltStore.GetTriggerConfig()
	if err != nil {
		return ToolCallResult{}, err
	}

	// Merge updates
	if input.Enabled != nil {
		config.Enabled = *input.Enabled
	}
	if input.SessionThreshold != nil {
		config.SessionThreshold = *input.SessionThreshold
	}
	if input.MinScoreGap != nil {
		config.MinScoreGap = *input.MinScoreGap
	}
	if input.WatchFiles != nil {
		config.WatchFiles = input.WatchFiles
	}

	if err := s.boltStore.SaveTriggerConfig(config); err != nil {
		return ToolCallResult{}, err
	}

	jsonOutput, _ := json.Marshal(config)
	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: "Trigger configuration updated:\n" + string(jsonOutput)}},
	}, nil
}

// formatCuratedExamples formats curated examples as markdown.
func formatCuratedExamples(examples []types.CuratedExample, includeNegative bool) string {
	var buf strings.Builder

	buf.WriteString("### What Works Well (from past sessions)\n\n")

	for _, ex := range examples {
		if ex.Score >= 0.75 {
			buf.WriteString(fmt.Sprintf("**Example: %s** (scored %.0f%%)\n",
				truncateString(ex.TaskPrompt, 50), ex.Score*100))
			if ex.Summary != "" {
				buf.WriteString(ex.Summary)
				buf.WriteString("\n")
			}
			buf.WriteString("\n")
		}
	}

	if includeNegative {
		for _, ex := range examples {
			if ex.Score < 0.5 {
				buf.WriteString("### What to Avoid\n\n")
				buf.WriteString(fmt.Sprintf("**Anti-example: %s** (scored %.0f%%)\n",
					truncateString(ex.TaskPrompt, 50), ex.Score*100))
				if ex.Summary != "" {
					buf.WriteString(ex.Summary)
					buf.WriteString("\n")
				}
				break
			}
		}
	}

	return buf.String()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func (s *Server) sendResult(id json.RawMessage, result interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	s.send(resp)
}

func (s *Server) sendError(id json.RawMessage, code int, message string, data interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	s.send(resp)
}

func (s *Server) send(resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("failed to marshal response: %v", err)
		return
	}
	s.writer.Write(append(data, '\n'))
}

// Strategy handlers

func (s *Server) handleStrategiesList(args json.RawMessage) (ToolCallResult, error) {
	var input TrajectoryStrategiesListInput
	if err := json.Unmarshal(args, &input); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
	}

	if input.Tag == "" {
		return ToolCallResult{}, fmt.Errorf("tag is required")
	}

	// Default to CLAUDE.md in current directory
	filePath := input.FilePath
	if filePath == "" {
		filePath = "CLAUDE.md"
	}

	parser := optimizer.NewParser()
	targets, err := parser.FindStrategiesTargets(filePath)
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to parse file: %w", err)
	}

	// Find matching target
	var matchingTarget *types.StrategiesTarget
	for _, t := range targets {
		if t.Tag == input.Tag {
			matchingTarget = &t
			break
		}
	}

	if matchingTarget == nil {
		return ToolCallResult{}, fmt.Errorf("no strategies found for tag: %s", input.Tag)
	}

	// Parse strategies from content
	strategies, err := parser.ParseStrategies(matchingTarget.Content)
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to parse strategies: %w", err)
	}

	// Get usage stats if available
	if s.boltStore != nil {
		stats, _ := s.boltStore.GetStrategyStats(input.Tag)
		for i, strat := range strategies {
			if stat, ok := stats[strat.Name]; ok {
				strategies[i].AvgScore = stat.AvgScore
				strategies[i].SessionCount = stat.SessionCount
			}
		}
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("## Strategies for '%s'\n\n", input.Tag))

	for _, strat := range strategies {
		output.WriteString(fmt.Sprintf("### %s\n", strat.Name))
		if strat.Description != "" {
			output.WriteString(fmt.Sprintf("*%s*\n\n", strat.Description))
		}
		if strat.SessionCount > 0 {
			output.WriteString(fmt.Sprintf("**Stats:** %d sessions, avg score: %.2f\n\n", strat.SessionCount, strat.AvgScore))
		}
		output.WriteString("**Approach:**\n```\n")
		output.WriteString(strat.ApproachPrompt)
		output.WriteString("\n```\n\n")
	}

	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: output.String()}},
	}, nil
}

func (s *Server) handleStrategiesSelect(args json.RawMessage) (ToolCallResult, error) {
	var input TrajectoryStrategiesSelectInput
	if err := json.Unmarshal(args, &input); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
	}

	if input.Tag == "" {
		return ToolCallResult{}, fmt.Errorf("tag is required")
	}
	if input.Mode == "" {
		return ToolCallResult{}, fmt.Errorf("mode is required")
	}

	// Default to CLAUDE.md
	filePath := input.FilePath
	if filePath == "" {
		filePath = "CLAUDE.md"
	}

	parser := optimizer.NewParser()
	targets, err := parser.FindStrategiesTargets(filePath)
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to parse file: %w", err)
	}

	// Find matching target
	var matchingTarget *types.StrategiesTarget
	for _, t := range targets {
		if t.Tag == input.Tag {
			matchingTarget = &t
			break
		}
	}

	if matchingTarget == nil {
		return ToolCallResult{}, fmt.Errorf("no strategies found for tag: %s", input.Tag)
	}

	// Parse strategies
	strategies, err := parser.ParseStrategies(matchingTarget.Content)
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to parse strategies: %w", err)
	}

	if len(strategies) == 0 {
		return ToolCallResult{}, fmt.Errorf("no strategies defined for tag: %s", input.Tag)
	}

	var selectedStrategy *types.Strategy
	var reason string

	switch input.Mode {
	case "explicit":
		if input.StrategyName == "" {
			return ToolCallResult{}, fmt.Errorf("strategy_name is required for explicit mode")
		}
		for _, strat := range strategies {
			if strat.Name == input.StrategyName {
				selectedStrategy = &strat
				reason = "Explicitly selected"
				break
			}
		}
		if selectedStrategy == nil {
			return ToolCallResult{}, fmt.Errorf("strategy '%s' not found", input.StrategyName)
		}

	case "recommend":
		// Get stats and recommend best performer
		var stats map[string]*types.Strategy
		if s.boltStore != nil {
			stats, _ = s.boltStore.GetStrategyStats(input.Tag)
		}

		var bestScore float64 = -1
		for _, strat := range strategies {
			if stat, ok := stats[strat.Name]; ok && stat.SessionCount >= 2 {
				if stat.AvgScore > bestScore {
					bestScore = stat.AvgScore
					stratCopy := strat
					stratCopy.AvgScore = stat.AvgScore
					stratCopy.SessionCount = stat.SessionCount
					selectedStrategy = &stratCopy
					reason = fmt.Sprintf("Best performer (%.2f avg over %d sessions)", stat.AvgScore, stat.SessionCount)
				}
			}
		}

		// Fall back to first strategy if no stats
		if selectedStrategy == nil {
			selectedStrategy = &strategies[0]
			reason = "Default (no performance data yet)"
		}

	case "rotate":
		// Find least-used strategy for exploration
		var stats map[string]*types.Strategy
		if s.boltStore != nil {
			stats, _ = s.boltStore.GetStrategyStats(input.Tag)
		}

		var minCount int = 999999
		for _, strat := range strategies {
			count := 0
			if stat, ok := stats[strat.Name]; ok {
				count = stat.SessionCount
			}
			if count < minCount {
				minCount = count
				stratCopy := strat
				if stat, ok := stats[strat.Name]; ok {
					stratCopy.SessionCount = stat.SessionCount
				}
				selectedStrategy = &stratCopy
				reason = fmt.Sprintf("Rotation for exploration (%d previous uses)", count)
			}
		}

	default:
		return ToolCallResult{}, fmt.Errorf("invalid mode: %s (use explicit, recommend, or rotate)", input.Mode)
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("## Selected Strategy: %s\n\n", selectedStrategy.Name))
	output.WriteString(fmt.Sprintf("**Selection reason:** %s\n\n", reason))
	if selectedStrategy.Description != "" {
		output.WriteString(fmt.Sprintf("**Description:** %s\n\n", selectedStrategy.Description))
	}
	output.WriteString("**Approach to use:**\n```\n")
	output.WriteString(selectedStrategy.ApproachPrompt)
	output.WriteString("\n```\n\n")
	output.WriteString(fmt.Sprintf("After completing the task, record this strategy with `trajectory_strategies_record` using strategy_name=\"%s\"", selectedStrategy.Name))

	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: output.String()}},
	}, nil
}

func (s *Server) handleStrategiesRecord(args json.RawMessage) (ToolCallResult, error) {
	if s.boltStore == nil {
		return ToolCallResult{}, fmt.Errorf("strategy recording not available")
	}

	var input TrajectoryStrategiesRecordInput
	if err := json.Unmarshal(args, &input); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
	}

	if input.SessionID == "" {
		return ToolCallResult{}, fmt.Errorf("session_id is required")
	}
	if input.Tag == "" {
		return ToolCallResult{}, fmt.Errorf("tag is required")
	}
	if input.StrategyName == "" {
		return ToolCallResult{}, fmt.Errorf("strategy_name is required")
	}

	usage := types.StrategyUsage{
		Tag:          input.Tag,
		StrategyName: input.StrategyName,
		SessionID:    input.SessionID,
		UsedAt:       time.Now(),
	}

	if err := s.boltStore.RecordStrategyUsage(usage); err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to record strategy usage: %w", err)
	}

	// Also update the session's Strategy field
	session, err := s.store.GetSession(input.SessionID)
	if err == nil {
		session.Strategy = input.StrategyName
		s.store.UpdateSession(session)
	}

	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Recorded strategy '%s' for session %s (tag: %s). When you stop the session, the score will be associated with this strategy.", input.StrategyName, input.SessionID, input.Tag)}},
	}, nil
}

func (s *Server) handleStrategiesAnalyze(args json.RawMessage) (ToolCallResult, error) {
	if s.boltStore == nil {
		return ToolCallResult{}, fmt.Errorf("strategy analysis not available")
	}

	var input TrajectoryStrategiesAnalyzeInput
	if err := json.Unmarshal(args, &input); err != nil {
		return ToolCallResult{}, fmt.Errorf("invalid input: %w", err)
	}

	if input.Tag == "" {
		return ToolCallResult{}, fmt.Errorf("tag is required")
	}

	stats, err := s.boltStore.GetStrategyStats(input.Tag)
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to get strategy stats: %w", err)
	}

	if len(stats) == 0 {
		return ToolCallResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("No strategy usage data for tag '%s' yet. Use `trajectory_strategies_record` to associate strategies with sessions.", input.Tag)}},
		}, nil
	}

	// Build analysis
	var output strings.Builder
	output.WriteString(fmt.Sprintf("## Strategy Analysis for '%s'\n\n", input.Tag))
	output.WriteString("| Strategy | Sessions | Avg Score |\n")
	output.WriteString("|----------|----------|----------|\n")

	var bestStrategy string
	var bestScore float64 = -1
	var totalSessions int
	var leastUsed string
	var leastUsedCount int = 999999

	for name, strat := range stats {
		scoreStr := "N/A"
		if strat.SessionCount > 0 && strat.AvgScore > 0 {
			scoreStr = fmt.Sprintf("%.2f", strat.AvgScore)
		}
		output.WriteString(fmt.Sprintf("| %s | %d | %s |\n", name, strat.SessionCount, scoreStr))

		totalSessions += strat.SessionCount

		if strat.AvgScore > bestScore && strat.SessionCount >= 2 {
			bestScore = strat.AvgScore
			bestStrategy = name
		}

		if strat.SessionCount < leastUsedCount {
			leastUsedCount = strat.SessionCount
			leastUsed = name
		}
	}

	output.WriteString("\n### Recommendations\n\n")

	if bestStrategy != "" {
		output.WriteString(fmt.Sprintf("**Best performer:** %s (%.2f avg score)\n", bestStrategy, bestScore))
	} else {
		output.WriteString("**Best performer:** Not enough data yet (need at least 2 sessions per strategy)\n")
	}

	// Check if we need more exploration
	needsExploration := false
	for _, strat := range stats {
		if strat.SessionCount < 3 {
			needsExploration = true
			break
		}
	}

	if needsExploration {
		output.WriteString(fmt.Sprintf("\n**Suggested next:** Try '%s' (only %d sessions) to gather more comparison data\n", leastUsed, leastUsedCount))
	} else if bestStrategy != "" {
		output.WriteString(fmt.Sprintf("\n**Suggested next:** Use '%s' (best performer) for optimal results\n", bestStrategy))
	}

	output.WriteString(fmt.Sprintf("\n**Total sessions analyzed:** %d\n", totalSessions))

	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: output.String()}},
	}, nil
}

