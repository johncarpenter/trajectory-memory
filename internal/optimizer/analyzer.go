package optimizer

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/2lines/trajectory-memory/internal/store"
	"github.com/2lines/trajectory-memory/internal/types"
)

var (
	// ErrInsufficientData is returned when there aren't enough sessions for analysis.
	ErrInsufficientData = errors.New("insufficient scored sessions for analysis")
)

// Score thresholds for cohort classification
const (
	HighScoreThreshold = 0.75
	LowScoreThreshold  = 0.5
)

// Analyzer provides trajectory analysis functionality.
type Analyzer struct {
	store store.Store
}

// NewAnalyzer creates a new Analyzer instance.
func NewAnalyzer(s store.Store) *Analyzer {
	return &Analyzer{store: s}
}

// Analyze performs analysis on all scored trajectories for a given tag.
func (a *Analyzer) Analyze(tag string, minSessions int) (*types.TrajectoryAnalysis, error) {
	// Get all sessions with this tag
	sessions, err := a.getSessionsByTag(tag)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}

	// Filter to scored sessions only
	var scored []*types.Session
	for _, s := range sessions {
		if s.Outcome != nil {
			scored = append(scored, s)
		}
	}

	if len(scored) < minSessions {
		return nil, fmt.Errorf("%w: have %d scored sessions for tag '%s', need at least %d",
			ErrInsufficientData, len(scored), tag, minSessions)
	}

	// Split into cohorts
	var high, medium, low []*types.Session
	for _, s := range scored {
		switch {
		case s.Outcome.Score >= HighScoreThreshold:
			high = append(high, s)
		case s.Outcome.Score >= LowScoreThreshold:
			medium = append(medium, s)
		default:
			low = append(low, s)
		}
	}

	// Calculate averages
	avgHigh := calculateAvgScore(high)
	avgLow := calculateAvgScore(low)

	// Extract patterns from high-scoring sessions
	highPatterns := a.extractPatterns(high, true)

	// Extract anti-patterns from low-scoring sessions
	lowAntiPatterns := a.extractAntiPatterns(low, high)

	// Generate recommended practices by combining insights
	recommendations := a.generateRecommendations(highPatterns, lowAntiPatterns)

	// Select curated examples
	curatedExamples := a.curateExamples(high, low)

	return &types.TrajectoryAnalysis{
		Tag:                  tag,
		TotalSessions:        len(scored),
		HighScoreSessions:    len(high),
		LowScoreSessions:     len(low),
		AvgScoreHigh:         avgHigh,
		AvgScoreLow:          avgLow,
		HighScorePatterns:    highPatterns,
		LowScoreAntiPatterns: lowAntiPatterns,
		RecommendedPractices: recommendations,
		CuratedExamples:      curatedExamples,
	}, nil
}

// getSessionsByTag retrieves all sessions with a specific tag.
func (a *Analyzer) getSessionsByTag(tag string) ([]*types.Session, error) {
	// Search for sessions with the tag
	metas, err := a.store.SearchSessions(tag, 1000)
	if err != nil {
		return nil, err
	}

	var sessions []*types.Session
	for _, meta := range metas {
		session, err := a.store.GetSession(meta.ID)
		if err != nil {
			continue
		}
		// Verify tag match (search might be fuzzy)
		for _, t := range session.Tags {
			if t == tag {
				sessions = append(sessions, session)
				break
			}
		}
	}

	return sessions, nil
}

// calculateAvgScore computes the average score for a list of sessions.
func calculateAvgScore(sessions []*types.Session) float64 {
	if len(sessions) == 0 {
		return 0
	}
	var sum float64
	for _, s := range sessions {
		if s.Outcome != nil {
			sum += s.Outcome.Score
		}
	}
	return sum / float64(len(sessions))
}

// extractPatterns identifies common patterns in high-scoring sessions.
func (a *Analyzer) extractPatterns(high []*types.Session, isHigh bool) []string {
	if len(high) == 0 {
		return nil
	}

	var patterns []string

	// 1. Read-before-write ratio
	readBeforeWrite := a.analyzeReadBeforeWrite(high)
	if readBeforeWrite.ratio > 0.6 && readBeforeWrite.avgReads >= 2 {
		patterns = append(patterns,
			fmt.Sprintf("Read source material before writing (%.0f%% of sessions read %.1f+ files first)",
				readBeforeWrite.ratio*100, readBeforeWrite.avgReads))
	}

	// 2. Revision detection
	revisionStats := a.analyzeRevisions(high)
	if revisionStats.ratio > 0.5 {
		patterns = append(patterns,
			fmt.Sprintf("Revise and iterate on output (%.0f%% of sessions made revisions)",
				revisionStats.ratio*100))
	}

	// 3. Tool diversity
	diversityStats := a.analyzeToolDiversity(high)
	if diversityStats.avgDistinct >= 3 {
		patterns = append(patterns,
			fmt.Sprintf("Use diverse tool set (average %.1f distinct tools used)",
				diversityStats.avgDistinct))
	}

	// 4. Step count
	stepStats := a.analyzeStepCount(high)
	if stepStats.avg >= 5 {
		patterns = append(patterns,
			fmt.Sprintf("Invest thoroughness with multiple steps (average %.0f steps)",
				stepStats.avg))
	}

	// 5. Self-critique detection
	selfCritiqueStats := a.analyzeSelfCritique(high)
	if selfCritiqueStats.ratio > 0.3 {
		patterns = append(patterns,
			fmt.Sprintf("Re-read output for self-critique (%.0f%% of sessions)",
				selfCritiqueStats.ratio*100))
	}

	// 6. Checkpoint detection
	checkpointStats := a.analyzeCheckpoints(high)
	if checkpointStats.ratio > 0.4 {
		patterns = append(patterns,
			fmt.Sprintf("Work incrementally with checkpoints (%.0f%% of sessions)",
				checkpointStats.ratio*100))
	}

	return patterns
}

// extractAntiPatterns identifies what low-scoring sessions did wrong.
func (a *Analyzer) extractAntiPatterns(low, high []*types.Session) []string {
	if len(low) == 0 {
		return nil
	}

	var antiPatterns []string

	// Compare against high-scoring sessions
	lowRead := a.analyzeReadBeforeWrite(low)
	highRead := a.analyzeReadBeforeWrite(high)
	if lowRead.avgReads < highRead.avgReads-1 && highRead.avgReads > 1 {
		antiPatterns = append(antiPatterns,
			fmt.Sprintf("Insufficient research before writing (%.1f reads vs %.1f in successful sessions)",
				lowRead.avgReads, highRead.avgReads))
	}

	lowRevision := a.analyzeRevisions(low)
	highRevision := a.analyzeRevisions(high)
	if lowRevision.ratio < highRevision.ratio-0.2 {
		antiPatterns = append(antiPatterns,
			"No revision pass - submitted first draft without review")
	}

	lowDiversity := a.analyzeToolDiversity(low)
	highDiversity := a.analyzeToolDiversity(high)
	if lowDiversity.avgDistinct < highDiversity.avgDistinct-1 {
		antiPatterns = append(antiPatterns,
			fmt.Sprintf("Limited tool usage (%.1f tools vs %.1f in successful sessions)",
				lowDiversity.avgDistinct, highDiversity.avgDistinct))
	}

	lowSteps := a.analyzeStepCount(low)
	highSteps := a.analyzeStepCount(high)
	if lowSteps.avg < highSteps.avg*0.5 {
		antiPatterns = append(antiPatterns,
			fmt.Sprintf("Rushed execution (%.0f steps vs %.0f in successful sessions)",
				lowSteps.avg, highSteps.avg))
	}

	return antiPatterns
}

// generateRecommendations creates actionable practices from patterns.
func (a *Analyzer) generateRecommendations(patterns, antiPatterns []string) []string {
	// Start with patterns, they're already positive practices
	recommendations := make([]string, 0, len(patterns)+len(antiPatterns))
	recommendations = append(recommendations, patterns...)

	// Convert anti-patterns to positive recommendations
	for _, anti := range antiPatterns {
		if strings.Contains(strings.ToLower(anti), "research") || strings.Contains(strings.ToLower(anti), "read") {
			recommendations = append(recommendations, "Read all available context before starting work")
		}
		if strings.Contains(strings.ToLower(anti), "revision") || strings.Contains(strings.ToLower(anti), "review") {
			recommendations = append(recommendations, "Always review and revise output before finalizing")
		}
		if strings.Contains(strings.ToLower(anti), "tool") {
			recommendations = append(recommendations, "Use multiple tools (search, read, validate) for thorough analysis")
		}
		if strings.Contains(strings.ToLower(anti), "rushed") || strings.Contains(strings.ToLower(anti), "steps") {
			recommendations = append(recommendations, "Take time for thorough analysis - avoid rushing")
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	unique := make([]string, 0, len(recommendations))
	for _, r := range recommendations {
		if !seen[r] {
			seen[r] = true
			unique = append(unique, r)
		}
	}

	return unique
}

// curateExamples selects the best examples for few-shot learning.
func (a *Analyzer) curateExamples(high, low []*types.Session) []types.CuratedExample {
	var examples []types.CuratedExample

	// Sort high by score descending
	sort.Slice(high, func(i, j int) bool {
		return high[i].Outcome.Score > high[j].Outcome.Score
	})

	// Select up to 3 high-scoring examples with diversity
	selected := a.selectDiverse(high, 3)
	for _, s := range selected {
		notes := ""
		if s.Outcome != nil {
			notes = s.Outcome.Notes
		}
		examples = append(examples, types.CuratedExample{
			SessionID:   s.ID,
			TaskPrompt:  s.TaskPrompt,
			Summary:     s.Summary,
			Score:       s.Outcome.Score,
			Notes:       notes,
			WhySelected: generateSelectionReason(s, true),
		})
	}

	// Select 1 low-scoring example as negative example
	if len(low) > 0 {
		// Sort by score ascending to get lowest
		sort.Slice(low, func(i, j int) bool {
			return low[i].Outcome.Score < low[j].Outcome.Score
		})

		// Find one with a summary
		for _, s := range low {
			if s.Summary != "" {
				notes := ""
				if s.Outcome != nil {
					notes = s.Outcome.Notes
				}
				examples = append(examples, types.CuratedExample{
					SessionID:   s.ID,
					TaskPrompt:  s.TaskPrompt,
					Summary:     s.Summary,
					Score:       s.Outcome.Score,
					Notes:       notes,
					WhySelected: generateSelectionReason(s, false),
				})
				break
			}
		}
	}

	return examples
}

// selectDiverse selects up to n sessions with diverse task prompts.
func (a *Analyzer) selectDiverse(sessions []*types.Session, n int) []*types.Session {
	if len(sessions) <= n {
		return sessions
	}

	var selected []*types.Session

	for _, s := range sessions {
		if len(selected) >= n {
			break
		}

		// Check diversity against already selected
		isDiverse := true
		for _, existing := range selected {
			if jaccardSimilarity(tokenize(s.TaskPrompt), tokenize(existing.TaskPrompt)) > 0.6 {
				isDiverse = false
				break
			}
		}

		if isDiverse && s.Summary != "" { // Prefer sessions with summaries
			selected = append(selected, s)
		}
	}

	// If we couldn't fill with diverse sessions, add remaining high scorers
	for _, s := range sessions {
		if len(selected) >= n {
			break
		}
		found := false
		for _, sel := range selected {
			if sel.ID == s.ID {
				found = true
				break
			}
		}
		if !found {
			selected = append(selected, s)
		}
	}

	return selected
}

// generateSelectionReason creates a rationale for why a session was selected.
func generateSelectionReason(s *types.Session, isPositive bool) string {
	if isPositive {
		return fmt.Sprintf("High-scoring session (%.0f%%) with %d steps demonstrating thorough approach",
			s.Outcome.Score*100, len(s.Steps))
	}
	return fmt.Sprintf("Low-scoring session (%.0f%%) showing common pitfalls to avoid",
		s.Outcome.Score*100)
}

// Analysis helper structs and functions

type readStats struct {
	ratio    float64 // proportion of sessions with reads before writes
	avgReads float64 // average reads before first write
}

func (a *Analyzer) analyzeReadBeforeWrite(sessions []*types.Session) readStats {
	if len(sessions) == 0 {
		return readStats{}
	}

	var withReads int
	var totalReads int

	for _, s := range sessions {
		readsBeforeWrite := 0
		for _, step := range s.Steps {
			if isWriteTool(step.ToolName) {
				break
			}
			if isReadTool(step.ToolName) {
				readsBeforeWrite++
			}
		}
		if readsBeforeWrite > 0 {
			withReads++
			totalReads += readsBeforeWrite
		}
	}

	ratio := float64(withReads) / float64(len(sessions))
	avgReads := 0.0
	if withReads > 0 {
		avgReads = float64(totalReads) / float64(withReads)
	}

	return readStats{ratio: ratio, avgReads: avgReads}
}

type revisionStats struct {
	ratio float64 // proportion of sessions with revisions
}

func (a *Analyzer) analyzeRevisions(sessions []*types.Session) revisionStats {
	if len(sessions) == 0 {
		return revisionStats{}
	}

	var withRevisions int
	for _, s := range sessions {
		writtenFiles := make(map[string]int)
		for _, step := range s.Steps {
			if isWriteTool(step.ToolName) {
				// Extract file path from input if possible
				path := extractFilePath(step.InputSummary)
				writtenFiles[path]++
			}
		}
		// Check if any file was written multiple times
		for _, count := range writtenFiles {
			if count > 1 {
				withRevisions++
				break
			}
		}
	}

	return revisionStats{ratio: float64(withRevisions) / float64(len(sessions))}
}

type diversityStats struct {
	avgDistinct float64
}

func (a *Analyzer) analyzeToolDiversity(sessions []*types.Session) diversityStats {
	if len(sessions) == 0 {
		return diversityStats{}
	}

	var totalDistinct int
	for _, s := range sessions {
		tools := make(map[string]bool)
		for _, step := range s.Steps {
			tools[step.ToolName] = true
		}
		totalDistinct += len(tools)
	}

	return diversityStats{avgDistinct: float64(totalDistinct) / float64(len(sessions))}
}

type stepCountStats struct {
	avg float64
}

func (a *Analyzer) analyzeStepCount(sessions []*types.Session) stepCountStats {
	if len(sessions) == 0 {
		return stepCountStats{}
	}

	var totalSteps int
	for _, s := range sessions {
		totalSteps += len(s.Steps)
	}

	return stepCountStats{avg: float64(totalSteps) / float64(len(sessions))}
}

type selfCritiqueStats struct {
	ratio float64
}

func (a *Analyzer) analyzeSelfCritique(sessions []*types.Session) selfCritiqueStats {
	if len(sessions) == 0 {
		return selfCritiqueStats{}
	}

	var withSelfCritique int
	for _, s := range sessions {
		recentWrites := make(map[string]bool)
		for _, step := range s.Steps {
			if isWriteTool(step.ToolName) {
				path := extractFilePath(step.InputSummary)
				recentWrites[path] = true
			}
			if isReadTool(step.ToolName) {
				path := extractFilePath(step.InputSummary)
				if recentWrites[path] {
					withSelfCritique++
					break
				}
			}
		}
	}

	return selfCritiqueStats{ratio: float64(withSelfCritique) / float64(len(sessions))}
}

type checkpointStats struct {
	ratio float64
}

func (a *Analyzer) analyzeCheckpoints(sessions []*types.Session) checkpointStats {
	if len(sessions) == 0 {
		return checkpointStats{}
	}

	var withCheckpoints int
	for _, s := range sessions {
		writeIndices := []int{}
		for i, step := range s.Steps {
			if isWriteTool(step.ToolName) {
				writeIndices = append(writeIndices, i)
			}
		}
		// Check if there are gaps between writes (other tools in between)
		if len(writeIndices) >= 2 {
			for i := 1; i < len(writeIndices); i++ {
				if writeIndices[i]-writeIndices[i-1] > 1 {
					withCheckpoints++
					break
				}
			}
		}
	}

	return checkpointStats{ratio: float64(withCheckpoints) / float64(len(sessions))}
}

// Helper functions

func isReadTool(name string) bool {
	lname := strings.ToLower(name)
	return lname == "read" || lname == "grep" || lname == "glob" ||
		strings.Contains(lname, "search") || strings.Contains(lname, "fetch")
}

func isWriteTool(name string) bool {
	lname := strings.ToLower(name)
	return lname == "write" || lname == "edit" || lname == "notebookedit" ||
		strings.Contains(lname, "create") || strings.Contains(lname, "update")
}

func extractFilePath(summary string) string {
	// Simple heuristic: look for common path patterns
	// Return the first word that looks like a path
	words := strings.Fields(summary)
	for _, w := range words {
		if strings.Contains(w, "/") || strings.Contains(w, ".") {
			return strings.Trim(w, "\"'`")
		}
	}
	return summary
}

func tokenize(text string) []string {
	// Simple word tokenization
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	// Filter common stop words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"to": true, "in": true, "for": true, "of": true, "on": true,
		"with": true, "this": true, "that": true, "is": true, "are": true,
	}
	var filtered []string
	for _, w := range words {
		if !stopWords[w] && len(w) > 2 {
			filtered = append(filtered, w)
		}
	}
	return filtered
}

func jaccardSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	setA := make(map[string]bool)
	for _, w := range a {
		setA[w] = true
	}

	setB := make(map[string]bool)
	for _, w := range b {
		setB[w] = true
	}

	intersection := 0
	for w := range setA {
		if setB[w] {
			intersection++
		}
	}

	union := len(setA)
	for w := range setB {
		if !setA[w] {
			union++
		}
	}

	return float64(intersection) / float64(union)
}
