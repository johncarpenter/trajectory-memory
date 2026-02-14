package optimizer

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/johncarpenter/trajectory-memory/internal/store"
	"github.com/johncarpenter/trajectory-memory/internal/types"
)

var (
	// ErrOptimizationNotProposed is returned when trying to apply a non-proposed optimization.
	ErrOptimizationNotProposed = errors.New("optimization is not in proposed status")
	// ErrOptimizationNotApplied is returned when trying to rollback a non-applied optimization.
	ErrOptimizationNotApplied = errors.New("optimization is not in accepted status")
	// ErrTargetNotFound is returned when the optimization target cannot be found in the file.
	ErrTargetNotFound = errors.New("optimization target not found in file")
)

// Optimizer provides context optimization functionality.
type Optimizer struct {
	store    *store.BoltStore
	analyzer *Analyzer
	parser   *Parser
}

// NewOptimizer creates a new Optimizer instance.
func NewOptimizer(s *store.BoltStore) *Optimizer {
	return &Optimizer{
		store:    s,
		analyzer: NewAnalyzer(s),
		parser:   NewParser(),
	}
}

// ProposeResult contains the result of a propose operation.
type ProposeResult struct {
	Target   types.OptimizationTarget
	Analysis *types.TrajectoryAnalysis
	Record   *types.OptimizationRecord
	Prompt   string // Meta-prompt for the model to generate optimized content
}

// Propose analyzes trajectories and generates a meta-prompt for optimization.
func (o *Optimizer) Propose(target types.OptimizationTarget) (*ProposeResult, error) {
	// Run analysis
	analysis, err := o.analyzer.Analyze(target.Tag, target.MinSessions)
	if err != nil {
		return nil, fmt.Errorf("analysis failed: %w", err)
	}

	// Generate meta-prompt
	prompt := o.generateMetaPrompt(target, analysis)

	// Create a preliminary record (content will be filled in by SaveProposal)
	record := &types.OptimizationRecord{
		ID:              store.NewULID(),
		TargetFile:      target.FilePath,
		Tag:             target.Tag,
		SessionsUsed:    analysis.TotalSessions,
		AvgScoreHigh:    analysis.AvgScoreHigh,
		AvgScoreLow:     analysis.AvgScoreLow,
		PreviousContent: target.Content,
		Status:          types.OptStatusProposed,
	}

	return &ProposeResult{
		Target:   target,
		Analysis: analysis,
		Record:   record,
		Prompt:   prompt,
	}, nil
}

// SaveProposal saves a proposed optimization with the generated content.
func (o *Optimizer) SaveProposal(recordID string, filePath string, tag string, previousContent string, newContent string) (*types.OptimizationRecord, error) {
	diff := generateUnifiedDiff(previousContent, newContent, "current", "proposed")

	record := &types.OptimizationRecord{
		ID:              recordID,
		TargetFile:      filePath,
		Tag:             tag,
		PreviousContent: previousContent,
		NewContent:      newContent,
		Diff:            diff,
		Status:          types.OptStatusProposed,
		CreatedAt:       time.Now(),
	}

	if err := o.store.CreateOptimization(record); err != nil {
		return nil, fmt.Errorf("failed to save proposal: %w", err)
	}

	return record, nil
}

// Apply applies a proposed optimization to the target file.
func (o *Optimizer) Apply(recordID string) error {
	record, err := o.store.GetOptimization(recordID)
	if err != nil {
		return err
	}

	if record.Status != types.OptStatusProposed {
		return ErrOptimizationNotProposed
	}

	// Find the target in the file
	targets, err := o.parser.FindTargets(record.TargetFile)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	var target *types.OptimizationTarget
	for _, t := range targets {
		if t.Tag == record.Tag {
			target = &t
			break
		}
	}

	if target == nil {
		return ErrTargetNotFound
	}

	// Replace the content
	if err := o.parser.ReplaceTarget(record.TargetFile, *target, record.NewContent); err != nil {
		return fmt.Errorf("failed to replace content: %w", err)
	}

	// Update record status
	now := time.Now()
	record.Status = types.OptStatusAccepted
	record.AppliedAt = &now

	return o.store.UpdateOptimization(record)
}

// Reject rejects a proposed optimization.
func (o *Optimizer) Reject(recordID string) error {
	record, err := o.store.GetOptimization(recordID)
	if err != nil {
		return err
	}

	if record.Status != types.OptStatusProposed {
		return ErrOptimizationNotProposed
	}

	record.Status = types.OptStatusRejected
	return o.store.UpdateOptimization(record)
}

// Rollback reverts an applied optimization.
func (o *Optimizer) Rollback(recordID string) error {
	record, err := o.store.GetOptimization(recordID)
	if err != nil {
		return err
	}

	if record.Status != types.OptStatusAccepted {
		return ErrOptimizationNotApplied
	}

	// Find the target in the file
	targets, err := o.parser.FindTargets(record.TargetFile)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	var target *types.OptimizationTarget
	for _, t := range targets {
		if t.Tag == record.Tag {
			target = &t
			break
		}
	}

	if target == nil {
		return ErrTargetNotFound
	}

	// Replace with previous content
	if err := o.parser.ReplaceTarget(record.TargetFile, *target, record.PreviousContent); err != nil {
		return fmt.Errorf("failed to restore content: %w", err)
	}

	// Update record status
	now := time.Now()
	record.Status = types.OptStatusRolledBack
	record.RolledBackAt = &now

	return o.store.UpdateOptimization(record)
}

// History returns optimization history, optionally filtered by file and/or tag.
func (o *Optimizer) History(filePath string, tag string, limit int) ([]types.OptimizationRecord, error) {
	return o.store.ListOptimizations(filePath, tag, limit)
}

// GetRecord retrieves an optimization record by ID.
func (o *Optimizer) GetRecord(recordID string) (*types.OptimizationRecord, error) {
	return o.store.GetOptimization(recordID)
}

// ProposeAll proposes optimizations for all eligible targets in a file.
func (o *Optimizer) ProposeAll(filePath string) ([]ProposeResult, error) {
	targets, err := o.parser.FindTargets(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	var results []ProposeResult
	for _, target := range targets {
		result, err := o.Propose(target)
		if err != nil {
			// Skip targets that don't have enough data
			if errors.Is(err, ErrInsufficientData) {
				continue
			}
			return nil, err
		}
		results = append(results, *result)
	}

	return results, nil
}

// generateMetaPrompt creates the prompt for the model to generate optimized content.
func (o *Optimizer) generateMetaPrompt(target types.OptimizationTarget, analysis *types.TrajectoryAnalysis) string {
	var buf bytes.Buffer

	buf.WriteString("## Context Optimization Request\n\n")
	buf.WriteString("You are analyzing trajectory data to improve agent instructions.\n\n")

	buf.WriteString("### Current Instructions\n")
	buf.WriteString("```\n")
	buf.WriteString(target.Content)
	buf.WriteString("\n```\n\n")

	buf.WriteString(fmt.Sprintf("### Trajectory Analysis for \"%s\" tasks\n\n", analysis.Tag))

	buf.WriteString(fmt.Sprintf("**%d high-scoring sessions (avg %.0f%%):**\n",
		analysis.HighScoreSessions, analysis.AvgScoreHigh*100))
	buf.WriteString("Patterns observed:\n")
	for _, pattern := range analysis.HighScorePatterns {
		buf.WriteString(fmt.Sprintf("- %s\n", pattern))
	}
	buf.WriteString("\n")

	buf.WriteString(fmt.Sprintf("**%d low-scoring sessions (avg %.0f%%):**\n",
		analysis.LowScoreSessions, analysis.AvgScoreLow*100))
	buf.WriteString("Anti-patterns observed:\n")
	for _, anti := range analysis.LowScoreAntiPatterns {
		buf.WriteString(fmt.Sprintf("- %s\n", anti))
	}
	buf.WriteString("\n")

	// Add curated examples
	hasHighExamples := false
	hasLowExample := false
	for _, ex := range analysis.CuratedExamples {
		if ex.Score >= HighScoreThreshold && !hasHighExamples {
			buf.WriteString("### Example High-Scoring Sessions\n\n")
			hasHighExamples = true
		}
		if ex.Score >= HighScoreThreshold {
			buf.WriteString(fmt.Sprintf("**Score: %.0f%%** — %s\n", ex.Score*100, ex.TaskPrompt))
			if ex.Summary != "" {
				buf.WriteString(ex.Summary)
				buf.WriteString("\n")
			}
			if ex.Notes != "" {
				buf.WriteString(fmt.Sprintf("User notes: %s\n", ex.Notes))
			}
			buf.WriteString("\n")
		}
	}

	for _, ex := range analysis.CuratedExamples {
		if ex.Score < LowScoreThreshold && !hasLowExample {
			buf.WriteString("### Example Low-Scoring Session\n\n")
			hasLowExample = true
			buf.WriteString(fmt.Sprintf("**Score: %.0f%%** — %s\n", ex.Score*100, ex.TaskPrompt))
			if ex.Summary != "" {
				buf.WriteString(ex.Summary)
				buf.WriteString("\n")
			}
			if ex.Notes != "" {
				buf.WriteString(fmt.Sprintf("User notes: %s\n", ex.Notes))
			}
			buf.WriteString("\n")
			break
		}
	}

	buf.WriteString("### Your Task\n\n")
	buf.WriteString(`Based on this trajectory data, rewrite the instructions section to help the
agent consistently achieve high-scoring outcomes. Your output should:

1. Be concrete and actionable (not vague advice)
2. Reference specific patterns that correlated with success
3. Warn against anti-patterns from low-scoring sessions
4. Be formatted as a numbered list of best practices
5. Include quantitative guidance where the data supports it
   (e.g., "use 5-8 searches" not "use multiple searches")
6. Be concise — aim for 8-12 practices maximum

Output ONLY the new instructions content, no preamble or explanation.
`)

	return buf.String()
}

// generateUnifiedDiff creates a unified diff between old and new content.
func generateUnifiedDiff(oldContent, newContent, oldName, newName string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("--- %s\n", oldName))
	buf.WriteString(fmt.Sprintf("+++ %s\n", newName))

	// Simple diff: show all removed lines, then all added lines
	// (A proper unified diff would use LCS, but this is sufficient for review)
	buf.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n", len(oldLines), len(newLines)))

	for _, line := range oldLines {
		buf.WriteString(fmt.Sprintf("-%s\n", line))
	}
	for _, line := range newLines {
		buf.WriteString(fmt.Sprintf("+%s\n", line))
	}

	return buf.String()
}

// FormatAnalysisForCLI formats the analysis for CLI output.
func FormatAnalysisForCLI(filePath string, target types.OptimizationTarget, analysis *types.TrajectoryAnalysis) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("Optimization Analysis for \"%s\" (tag) in %s\n\n", target.Tag, filePath))

	buf.WriteString(fmt.Sprintf("Sessions analyzed: %d scored (%d high, %d medium, %d low)\n",
		analysis.TotalSessions,
		analysis.HighScoreSessions,
		analysis.TotalSessions-analysis.HighScoreSessions-analysis.LowScoreSessions,
		analysis.LowScoreSessions))
	buf.WriteString(fmt.Sprintf("High-scoring average: %.2f\n", analysis.AvgScoreHigh))
	buf.WriteString(fmt.Sprintf("Low-scoring average: %.2f\n\n", analysis.AvgScoreLow))

	buf.WriteString("Patterns in high-scoring sessions:\n")
	for _, p := range analysis.HighScorePatterns {
		buf.WriteString(fmt.Sprintf("  ✓ %s\n", p))
	}
	buf.WriteString("\n")

	buf.WriteString("Anti-patterns in low-scoring sessions:\n")
	for _, a := range analysis.LowScoreAntiPatterns {
		buf.WriteString(fmt.Sprintf("  ✗ %s\n", a))
	}
	buf.WriteString("\n")

	buf.WriteString(fmt.Sprintf("Current content (lines %d-%d of %s):\n",
		target.StartLine+1, target.EndLine-1, filePath))
	for _, line := range strings.Split(target.Content, "\n") {
		buf.WriteString(fmt.Sprintf("  %s\n", line))
	}
	buf.WriteString("\n")

	buf.WriteString("To generate optimized content, run this analysis through a model\n")
	buf.WriteString("or use the MCP tool in a Claude Code session.\n")

	return buf.String()
}

// FormatHistoryForCLI formats optimization history for CLI output.
func FormatHistoryForCLI(records []types.OptimizationRecord) string {
	var buf bytes.Buffer

	buf.WriteString("Optimization History\n\n")

	if len(records) == 0 {
		buf.WriteString("No optimization records found.\n")
		return buf.String()
	}

	// Simple table format
	buf.WriteString("┌────────────┬──────────────────┬──────────┬──────────┬───────────┬────────────┐\n")
	buf.WriteString("│ ID         │ File             │ Tag      │ Status   │ Sessions  │ Date       │\n")
	buf.WriteString("├────────────┼──────────────────┼──────────┼──────────┼───────────┼────────────┤\n")

	for _, r := range records {
		id := r.ID
		if len(id) > 10 {
			id = id[:8] + ".."
		}
		file := r.TargetFile
		if len(file) > 16 {
			// Show just filename
			parts := strings.Split(file, "/")
			file = parts[len(parts)-1]
			if len(file) > 16 {
				file = file[:14] + ".."
			}
		}
		tag := r.Tag
		if len(tag) > 8 {
			tag = tag[:6] + ".."
		}
		status := r.Status
		if len(status) > 8 {
			status = status[:6] + ".."
		}

		date := r.CreatedAt.Format("2006-01-02")

		buf.WriteString(fmt.Sprintf("│ %-10s │ %-16s │ %-8s │ %-8s │ %-9d │ %-10s │\n",
			id, file, tag, status, r.SessionsUsed, date))
	}

	buf.WriteString("└────────────┴──────────────────┴──────────┴──────────┴───────────┴────────────┘\n")

	return buf.String()
}
