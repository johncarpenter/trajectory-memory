// Package summarize provides trajectory formatting for model summarization.
package summarize

import (
	"fmt"
	"strings"
	"time"

	"github.com/2lines/trajectory-memory/internal/types"
)

// FormatOptions controls trajectory formatting.
type FormatOptions struct {
	// IncludeSummarizationPrompt adds the prompt asking the model to summarize.
	IncludeSummarizationPrompt bool
	// MaxSteps limits the number of steps to include (0 = no limit).
	MaxSteps int
	// Verbose includes additional details like duration and loaded context.
	Verbose bool
}

// DefaultOptions returns the default formatting options.
func DefaultOptions() FormatOptions {
	return FormatOptions{
		IncludeSummarizationPrompt: true,
		MaxSteps:                   0,
		Verbose:                    true,
	}
}

// FormatTrajectoryForSummarization formats a session for model summarization.
func FormatTrajectoryForSummarization(s *types.Session) string {
	return FormatTrajectoryWithOptions(s, DefaultOptions())
}

// FormatTrajectoryWithOptions formats a session with custom options.
func FormatTrajectoryWithOptions(s *types.Session, opts FormatOptions) string {
	var sb strings.Builder

	// Header
	sb.WriteString("## Session Trajectory\n\n")
	sb.WriteString(fmt.Sprintf("**Session ID:** %s\n", s.ID))
	sb.WriteString(fmt.Sprintf("**Task:** %q\n", s.TaskPrompt))

	if opts.Verbose {
		sb.WriteString(fmt.Sprintf("**Working Directory:** %s\n", s.WorkingDir))
		sb.WriteString(fmt.Sprintf("**Duration:** %s\n", formatDuration(s.StartedAt, s.CompletedAt)))

		if len(s.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("**Tags:** %s\n", strings.Join(s.Tags, ", ")))
		}
		if len(s.LoadedContext) > 0 {
			sb.WriteString(fmt.Sprintf("**Loaded Context:** %s\n", strings.Join(s.LoadedContext, ", ")))
		}
		if s.ClaudeMDHash != "" {
			sb.WriteString(fmt.Sprintf("**CLAUDE.md Hash:** %s\n", s.ClaudeMDHash[:8]))
		}
	}

	// Steps
	totalSteps := len(s.Steps)
	sb.WriteString(fmt.Sprintf("\n### Steps (%d total):\n\n", totalSteps))

	steps := s.Steps
	elided := 0

	// For long sessions, show first 5, last 5, and important steps
	if totalSteps > 50 {
		steps = selectRelevantSteps(s.Steps)
		elided = totalSteps - len(steps)
		sb.WriteString(fmt.Sprintf("*(Showing %d of %d steps; %d steps elided)*\n\n", len(steps), totalSteps, elided))
	} else if opts.MaxSteps > 0 && totalSteps > opts.MaxSteps {
		steps = s.Steps[:opts.MaxSteps]
		elided = totalSteps - opts.MaxSteps
		sb.WriteString(fmt.Sprintf("*(Showing first %d of %d steps)*\n\n", opts.MaxSteps, totalSteps))
	}

	// Format steps
	for i, step := range steps {
		if totalSteps <= 50 {
			// Numbered for shorter sessions
			sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, step.ToolName, truncateStepSummary(step.InputSummary)))
		} else {
			// Bulleted for longer sessions (order less meaningful)
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", step.ToolName, truncateStepSummary(step.InputSummary)))
		}

		// Include output for Write operations
		if step.ToolName == "Write" || step.ToolName == "Edit" {
			if step.OutputSummary != "" {
				sb.WriteString(fmt.Sprintf("  → %s\n", truncateStepSummary(step.OutputSummary)))
			}
		}
	}

	// Outcome if scored
	if s.Outcome != nil {
		sb.WriteString(fmt.Sprintf("\n**Outcome:** Score %.2f", s.Outcome.Score))
		if s.Outcome.Notes != "" {
			sb.WriteString(fmt.Sprintf(" - %s", s.Outcome.Notes))
		}
		sb.WriteString("\n")
	}

	// Summarization prompt
	if opts.IncludeSummarizationPrompt {
		sb.WriteString("\n---\n\n")
		sb.WriteString("Please provide a 2-3 sentence summary of this execution trace: what task was accomplished, what approach was taken, and any notable patterns in the execution.\n\n")
		sb.WriteString(fmt.Sprintf("After generating the summary, call `trajectory_summarize` with session_id \"%s\" and your summary text.\n", s.ID))
	}

	return sb.String()
}

// FormatCompactTrajectory returns a minimal trajectory representation.
func FormatCompactTrajectory(s *types.Session) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Task: %s\n", truncate(s.TaskPrompt, 100)))
	sb.WriteString(fmt.Sprintf("Steps: %d | Status: %s", len(s.Steps), s.Status))

	if s.Outcome != nil {
		sb.WriteString(fmt.Sprintf(" | Score: %.2f", s.Outcome.Score))
	}
	sb.WriteString("\n")

	// Tool usage summary
	toolCounts := make(map[string]int)
	for _, step := range s.Steps {
		toolCounts[step.ToolName]++
	}

	var tools []string
	for tool, count := range toolCounts {
		tools = append(tools, fmt.Sprintf("%s:%d", tool, count))
	}
	if len(tools) > 0 {
		sb.WriteString(fmt.Sprintf("Tools: %s\n", strings.Join(tools, " ")))
	}

	if s.Summary != "" {
		sb.WriteString(fmt.Sprintf("Summary: %s\n", truncate(s.Summary, 200)))
	}

	return sb.String()
}

// selectRelevantSteps selects the most relevant steps for a long session.
// Returns first 5, last 5, and all Write/Edit steps in between.
func selectRelevantSteps(steps []types.TrajectoryStep) []types.TrajectoryStep {
	if len(steps) <= 10 {
		return steps
	}

	var result []types.TrajectoryStep

	// First 5 steps (show initial approach)
	for i := 0; i < 5 && i < len(steps); i++ {
		result = append(result, steps[i])
	}

	// Important steps in the middle (Write, Edit, Bash with significant output)
	for i := 5; i < len(steps)-5; i++ {
		step := steps[i]
		switch step.ToolName {
		case "Write", "Edit", "NotebookEdit":
			result = append(result, step)
		case "Bash":
			// Include bash commands that look significant
			if isSignificantBashCommand(step.InputSummary) {
				result = append(result, step)
			}
		}
	}

	// Last 5 steps (show conclusion)
	start := len(steps) - 5
	if start < 5 {
		start = 5
	}
	for i := start; i < len(steps); i++ {
		result = append(result, steps[i])
	}

	return result
}

// isSignificantBashCommand checks if a bash command is worth including.
func isSignificantBashCommand(summary string) bool {
	significantPrefixes := []string{
		"git commit",
		"git push",
		"npm ",
		"yarn ",
		"go build",
		"go test",
		"make",
		"docker",
		"pytest",
		"cargo",
	}

	lower := strings.ToLower(summary)
	for _, prefix := range significantPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// formatDuration formats the duration between two times.
func formatDuration(start time.Time, end *time.Time) string {
	var endTime time.Time
	if end != nil {
		endTime = *end
	} else {
		endTime = time.Now()
	}

	d := endTime.Sub(start)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// truncate truncates a string to maxLen with ellipsis.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// truncateStepSummary truncates step summaries for display.
func truncateStepSummary(s string) string {
	// Remove newlines for cleaner display
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	// Collapse multiple spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return truncate(strings.TrimSpace(s), 100)
}
