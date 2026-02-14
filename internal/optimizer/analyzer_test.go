package optimizer

import (
	"io"
	"testing"
	"time"

	"github.com/johncarpenter/trajectory-memory/internal/types"
)

// mockStore implements store.Store for testing
type mockStore struct {
	sessions map[string]*types.Session
}

func newMockStore() *mockStore {
	return &mockStore{sessions: make(map[string]*types.Session)}
}

func (m *mockStore) CreateSession(s *types.Session) error {
	m.sessions[s.ID] = s
	return nil
}

func (m *mockStore) GetSession(id string) (*types.Session, error) {
	if s, ok := m.sessions[id]; ok {
		return s, nil
	}
	return nil, nil
}

func (m *mockStore) UpdateSession(s *types.Session) error {
	m.sessions[s.ID] = s
	return nil
}

func (m *mockStore) AppendStep(sessionID string, step types.TrajectoryStep) error {
	return nil
}

func (m *mockStore) ListSessions(limit int, offset int) ([]types.SessionMetadata, error) {
	return nil, nil
}

func (m *mockStore) SearchSessions(query string, limit int) ([]types.SessionMetadata, error) {
	var results []types.SessionMetadata
	for _, s := range m.sessions {
		for _, tag := range s.Tags {
			if tag == query {
				results = append(results, s.ToMetadata())
				break
			}
		}
	}
	return results, nil
}

func (m *mockStore) SetOutcome(sessionID string, outcome types.Outcome) error {
	return nil
}

func (m *mockStore) GetActiveSession() (*types.Session, error) {
	return nil, nil
}

func (m *mockStore) SetActiveSession(sessionID string) error {
	return nil
}

func (m *mockStore) ClearActiveSession() error {
	return nil
}

func (m *mockStore) DeleteSession(id string) error {
	return nil
}

func (m *mockStore) ExportAll(w io.Writer) error {
	return nil
}

func (m *mockStore) ImportAll(r io.Reader) error {
	return nil
}

func (m *mockStore) Close() error {
	return nil
}

func TestAnalyzer_Analyze_InsufficientData(t *testing.T) {
	store := newMockStore()

	// Add only 2 sessions when min is 5
	store.CreateSession(createTestSession("1", "research", 0.9))
	store.CreateSession(createTestSession("2", "research", 0.8))

	analyzer := NewAnalyzer(store)
	_, err := analyzer.Analyze("research", 5)

	if err == nil {
		t.Fatal("expected error for insufficient data")
	}
	if !containsString(err.Error(), "insufficient") {
		t.Errorf("expected insufficient data error, got: %v", err)
	}
}

func TestAnalyzer_Analyze_CohortSplitting(t *testing.T) {
	store := newMockStore()

	// High scoring (>= 0.75)
	store.CreateSession(createTestSession("1", "research", 0.95))
	store.CreateSession(createTestSession("2", "research", 0.85))
	store.CreateSession(createTestSession("3", "research", 0.80))

	// Medium scoring (0.5 - 0.75)
	store.CreateSession(createTestSession("4", "research", 0.65))
	store.CreateSession(createTestSession("5", "research", 0.55))

	// Low scoring (< 0.5)
	store.CreateSession(createTestSession("6", "research", 0.40))
	store.CreateSession(createTestSession("7", "research", 0.30))

	analyzer := NewAnalyzer(store)
	analysis, err := analyzer.Analyze("research", 5)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis.TotalSessions != 7 {
		t.Errorf("expected 7 total sessions, got %d", analysis.TotalSessions)
	}
	if analysis.HighScoreSessions != 3 {
		t.Errorf("expected 3 high sessions, got %d", analysis.HighScoreSessions)
	}
	if analysis.LowScoreSessions != 2 {
		t.Errorf("expected 2 low sessions, got %d", analysis.LowScoreSessions)
	}
}

func TestAnalyzer_Analyze_AverageScores(t *testing.T) {
	store := newMockStore()

	// High scoring
	store.CreateSession(createTestSession("1", "research", 0.90))
	store.CreateSession(createTestSession("2", "research", 0.80))

	// Low scoring
	store.CreateSession(createTestSession("3", "research", 0.30))
	store.CreateSession(createTestSession("4", "research", 0.20))

	// Medium (to reach min)
	store.CreateSession(createTestSession("5", "research", 0.60))

	analyzer := NewAnalyzer(store)
	analysis, err := analyzer.Analyze("research", 5)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// High average should be (0.90 + 0.80) / 2 = 0.85
	if analysis.AvgScoreHigh < 0.84 || analysis.AvgScoreHigh > 0.86 {
		t.Errorf("expected avg high ~0.85, got %.2f", analysis.AvgScoreHigh)
	}

	// Low average should be (0.30 + 0.20) / 2 = 0.25
	if analysis.AvgScoreLow < 0.24 || analysis.AvgScoreLow > 0.26 {
		t.Errorf("expected avg low ~0.25, got %.2f", analysis.AvgScoreLow)
	}
}

func TestAnalyzer_PatternExtraction_ReadBeforeWrite(t *testing.T) {
	store := newMockStore()

	// Sessions with reads before writes
	s1 := createTestSession("1", "research", 0.90)
	s1.Steps = []types.TrajectoryStep{
		{ToolName: "Read", InputSummary: "file1.md"},
		{ToolName: "Read", InputSummary: "file2.md"},
		{ToolName: "Read", InputSummary: "file3.md"},
		{ToolName: "Write", InputSummary: "output.md"},
	}
	store.CreateSession(s1)

	s2 := createTestSession("2", "research", 0.85)
	s2.Steps = []types.TrajectoryStep{
		{ToolName: "Read", InputSummary: "source.md"},
		{ToolName: "Read", InputSummary: "context.md"},
		{ToolName: "Write", InputSummary: "result.md"},
	}
	store.CreateSession(s2)

	// Session with write first (low score)
	s3 := createTestSession("3", "research", 0.30)
	s3.Steps = []types.TrajectoryStep{
		{ToolName: "Write", InputSummary: "output.md"},
	}
	store.CreateSession(s3)

	// Add more to meet minimum
	store.CreateSession(createTestSession("4", "research", 0.50))
	store.CreateSession(createTestSession("5", "research", 0.55))

	analyzer := NewAnalyzer(store)
	analysis, err := analyzer.Analyze("research", 5)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect read-before-write pattern in high scorers
	found := false
	for _, p := range analysis.HighScorePatterns {
		if containsString(p, "Read") || containsString(p, "read") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected read-before-write pattern in high scorers")
	}
}

func TestAnalyzer_PatternExtraction_Revisions(t *testing.T) {
	store := newMockStore()

	// Session with revisions (multiple writes to same file)
	s1 := createTestSession("1", "research", 0.90)
	s1.Steps = []types.TrajectoryStep{
		{ToolName: "Write", InputSummary: "output.md"},
		{ToolName: "Read", InputSummary: "output.md"},
		{ToolName: "Write", InputSummary: "output.md"}, // Revision
	}
	store.CreateSession(s1)

	s2 := createTestSession("2", "research", 0.85)
	s2.Steps = []types.TrajectoryStep{
		{ToolName: "Write", InputSummary: "draft.md"},
		{ToolName: "Write", InputSummary: "draft.md"}, // Revision
	}
	store.CreateSession(s2)

	// Add more to meet minimum
	store.CreateSession(createTestSession("3", "research", 0.60))
	store.CreateSession(createTestSession("4", "research", 0.55))
	store.CreateSession(createTestSession("5", "research", 0.30))

	analyzer := NewAnalyzer(store)
	analysis, err := analyzer.Analyze("research", 5)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect revision pattern
	found := false
	for _, p := range analysis.HighScorePatterns {
		if containsString(p, "Revise") || containsString(p, "revision") || containsString(p, "iterate") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected revision pattern in high scorers")
	}
}

func TestAnalyzer_PatternExtraction_ToolDiversity(t *testing.T) {
	store := newMockStore()

	// Session with high tool diversity
	s1 := createTestSession("1", "research", 0.90)
	s1.Steps = []types.TrajectoryStep{
		{ToolName: "Glob"},
		{ToolName: "Grep"},
		{ToolName: "Read"},
		{ToolName: "WebSearch"},
		{ToolName: "Write"},
	}
	store.CreateSession(s1)

	s2 := createTestSession("2", "research", 0.85)
	s2.Steps = []types.TrajectoryStep{
		{ToolName: "Read"},
		{ToolName: "Grep"},
		{ToolName: "WebFetch"},
		{ToolName: "Write"},
	}
	store.CreateSession(s2)

	// Low diversity session
	s3 := createTestSession("3", "research", 0.30)
	s3.Steps = []types.TrajectoryStep{
		{ToolName: "Write"},
	}
	store.CreateSession(s3)

	store.CreateSession(createTestSession("4", "research", 0.55))
	store.CreateSession(createTestSession("5", "research", 0.50))

	analyzer := NewAnalyzer(store)
	analysis, err := analyzer.Analyze("research", 5)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect tool diversity pattern
	found := false
	for _, p := range analysis.HighScorePatterns {
		if containsString(p, "diverse") || containsString(p, "tool") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected tool diversity pattern")
	}
}

func TestAnalyzer_CuratedExamples(t *testing.T) {
	store := newMockStore()

	// High scoring sessions with summaries
	s1 := createTestSession("1", "research", 0.95)
	s1.Summary = "Excellent systematic research with clear methodology"
	store.CreateSession(s1)

	s2 := createTestSession("2", "research", 0.90)
	s2.Summary = "Good analysis with multiple sources"
	store.CreateSession(s2)

	s3 := createTestSession("3", "research", 0.85)
	s3.Summary = "Thorough investigation with actionable results"
	store.CreateSession(s3)

	// Low scoring session with summary
	s4 := createTestSession("4", "research", 0.25)
	s4.Summary = "Quick shallow analysis with no sources"
	store.CreateSession(s4)

	// Add one more to meet minimum
	store.CreateSession(createTestSession("5", "research", 0.60))

	analyzer := NewAnalyzer(store)
	analysis, err := analyzer.Analyze("research", 5)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have up to 4 curated examples (3 high + 1 low)
	if len(analysis.CuratedExamples) < 2 {
		t.Errorf("expected at least 2 curated examples, got %d", len(analysis.CuratedExamples))
	}

	// Check that there's a low-score example
	hasLow := false
	for _, ex := range analysis.CuratedExamples {
		if ex.Score < 0.5 {
			hasLow = true
			break
		}
	}
	if !hasLow {
		t.Error("expected a low-score curated example")
	}
}

func TestAnalyzer_CuratedExamples_Diversity(t *testing.T) {
	store := newMockStore()

	// Similar task prompts (should dedupe)
	s1 := createTestSession("1", "research", 0.95)
	s1.TaskPrompt = "Research AI assistants for code"
	s1.Summary = "Analysis of AI tools"
	store.CreateSession(s1)

	s2 := createTestSession("2", "research", 0.90)
	s2.TaskPrompt = "Research AI assistants for coding"
	s2.Summary = "Similar analysis"
	store.CreateSession(s2)

	// Different task prompt
	s3 := createTestSession("3", "research", 0.85)
	s3.TaskPrompt = "Market analysis of renewable energy sector"
	s3.Summary = "Energy market study"
	store.CreateSession(s3)

	store.CreateSession(createTestSession("4", "research", 0.60))
	store.CreateSession(createTestSession("5", "research", 0.30))

	analyzer := NewAnalyzer(store)
	analysis, err := analyzer.Analyze("research", 5)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should prefer diverse examples
	if len(analysis.CuratedExamples) == 0 {
		t.Fatal("expected curated examples")
	}

	// Check that the energy analysis was included (it's different)
	foundEnergy := false
	for _, ex := range analysis.CuratedExamples {
		if containsString(ex.TaskPrompt, "energy") {
			foundEnergy = true
			break
		}
	}
	if !foundEnergy {
		t.Error("expected diverse example (energy) to be included")
	}
}

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected float64
	}{
		{"identical", []string{"hello", "world"}, []string{"hello", "world"}, 1.0},
		{"no overlap", []string{"hello"}, []string{"world"}, 0.0},
		{"partial", []string{"hello", "world"}, []string{"hello", "there"}, 0.333},
		{"empty both", []string{}, []string{}, 1.0},
		{"empty one", []string{"hello"}, []string{}, 0.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := jaccardSimilarity(tc.a, tc.b)
			if result < tc.expected-0.01 || result > tc.expected+0.01 {
				t.Errorf("expected ~%.3f, got %.3f", tc.expected, result)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	text := "Research the AI assistants for code review"
	tokens := tokenize(text)

	// Should have meaningful words, no stop words, no short words
	if len(tokens) == 0 {
		t.Error("expected some tokens")
	}

	for _, token := range tokens {
		if token == "the" || token == "for" {
			t.Errorf("stop word not filtered: %s", token)
		}
		if len(token) <= 2 {
			t.Errorf("short word not filtered: %s", token)
		}
	}
}

// Helper functions

func createTestSession(id, tag string, score float64) *types.Session {
	return &types.Session{
		ID:         id,
		TaskPrompt: "Test task prompt for " + tag,
		Tags:       []string{tag},
		Status:     types.StatusScored,
		StartedAt:  time.Now(),
		Outcome: &types.Outcome{
			Score:    score,
			ScoredAt: time.Now(),
		},
		Steps: []types.TrajectoryStep{
			{ToolName: "Read"},
			{ToolName: "Write"},
		},
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
