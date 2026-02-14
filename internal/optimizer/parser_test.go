package optimizer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParser_FindTargets_SingleTarget(t *testing.T) {
	content := `# Test File

## Best Practices

<!-- trajectory-optimize:start tag="research" min_sessions=10 -->
1. Do this first
2. Then do this
3. Finally do this
<!-- trajectory-optimize:end -->

## Other Section
Some other content.
`
	filePath := writeTempFile(t, content)
	defer os.Remove(filePath)

	p := NewParser()
	targets, err := p.FindTargets(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	target := targets[0]
	if target.Tag != "research" {
		t.Errorf("expected tag 'research', got '%s'", target.Tag)
	}
	if target.MinSessions != 10 {
		t.Errorf("expected min_sessions 10, got %d", target.MinSessions)
	}
	if target.StartLine != 5 {
		t.Errorf("expected start line 5, got %d", target.StartLine)
	}
	if target.EndLine != 9 {
		t.Errorf("expected end line 9, got %d", target.EndLine)
	}
	expectedContent := "1. Do this first\n2. Then do this\n3. Finally do this"
	if target.Content != expectedContent {
		t.Errorf("content mismatch:\nexpected: %q\ngot: %q", expectedContent, target.Content)
	}
}

func TestParser_FindTargets_MultipleTargets(t *testing.T) {
	content := `# Config File

<!-- trajectory-optimize:start tag="research" min_sessions=5 -->
Research practices here
<!-- trajectory-optimize:end -->

Some content in between.

<!-- trajectory-optimize:start tag="writing" min_sessions=15 -->
Writing practices here
<!-- trajectory-optimize:end -->
`
	filePath := writeTempFile(t, content)
	defer os.Remove(filePath)

	p := NewParser()
	targets, err := p.FindTargets(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	if targets[0].Tag != "research" || targets[0].MinSessions != 5 {
		t.Errorf("first target mismatch: tag=%s, min_sessions=%d", targets[0].Tag, targets[0].MinSessions)
	}
	if targets[1].Tag != "writing" || targets[1].MinSessions != 15 {
		t.Errorf("second target mismatch: tag=%s, min_sessions=%d", targets[1].Tag, targets[1].MinSessions)
	}
}

func TestParser_FindTargets_DefaultMinSessions(t *testing.T) {
	content := `<!-- trajectory-optimize:start tag="testing" -->
Content
<!-- trajectory-optimize:end -->
`
	filePath := writeTempFile(t, content)
	defer os.Remove(filePath)

	p := NewParser()
	targets, err := p.FindTargets(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].MinSessions != 10 {
		t.Errorf("expected default min_sessions 10, got %d", targets[0].MinSessions)
	}
}

func TestParser_FindTargets_FlexibleWhitespace(t *testing.T) {
	content := `<!--trajectory-optimize:start   tag="test"   min_sessions=20  -->
Content
<!--  trajectory-optimize:end   -->
`
	filePath := writeTempFile(t, content)
	defer os.Remove(filePath)

	p := NewParser()
	targets, err := p.FindTargets(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Tag != "test" {
		t.Errorf("expected tag 'test', got '%s'", targets[0].Tag)
	}
	if targets[0].MinSessions != 20 {
		t.Errorf("expected min_sessions 20, got %d", targets[0].MinSessions)
	}
}

func TestParser_FindTargets_NestedMarkersError(t *testing.T) {
	content := `<!-- trajectory-optimize:start tag="outer" -->
Outer content
<!-- trajectory-optimize:start tag="inner" -->
Inner content
<!-- trajectory-optimize:end -->
<!-- trajectory-optimize:end -->
`
	filePath := writeTempFile(t, content)
	defer os.Remove(filePath)

	p := NewParser()
	_, err := p.FindTargets(filePath)
	if err == nil {
		t.Fatal("expected error for nested markers, got nil")
	}
	if !strings.Contains(err.Error(), "nested") {
		t.Errorf("expected nested marker error, got: %v", err)
	}
}

func TestParser_FindTargets_UnpairedStartMarker(t *testing.T) {
	content := `<!-- trajectory-optimize:start tag="test" -->
Content without end marker
`
	filePath := writeTempFile(t, content)
	defer os.Remove(filePath)

	p := NewParser()
	_, err := p.FindTargets(filePath)
	if err == nil {
		t.Fatal("expected error for unpaired marker, got nil")
	}
	if !strings.Contains(err.Error(), "without end") {
		t.Errorf("expected unpaired marker error, got: %v", err)
	}
}

func TestParser_FindTargets_UnpairedEndMarker(t *testing.T) {
	content := `Some content
<!-- trajectory-optimize:end -->
`
	filePath := writeTempFile(t, content)
	defer os.Remove(filePath)

	p := NewParser()
	_, err := p.FindTargets(filePath)
	if err == nil {
		t.Fatal("expected error for unpaired end marker, got nil")
	}
	if !strings.Contains(err.Error(), "without start") {
		t.Errorf("expected unpaired marker error, got: %v", err)
	}
}

func TestParser_FindTargets_MissingTagAttribute(t *testing.T) {
	content := `<!-- trajectory-optimize:start min_sessions=10 -->
Content
<!-- trajectory-optimize:end -->
`
	filePath := writeTempFile(t, content)
	defer os.Remove(filePath)

	p := NewParser()
	_, err := p.FindTargets(filePath)
	if err == nil {
		t.Fatal("expected error for missing tag, got nil")
	}
	if !strings.Contains(err.Error(), "tag") {
		t.Errorf("expected missing tag error, got: %v", err)
	}
}

func TestParser_FindExamplesTargets(t *testing.T) {
	content := `# Examples

<!-- trajectory-examples:start tag="research" max=5 include_negative=false -->
Example content here
<!-- trajectory-examples:end -->
`
	filePath := writeTempFile(t, content)
	defer os.Remove(filePath)

	p := NewParser()
	targets, err := p.FindExamplesTargets(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	target := targets[0]
	if target.Tag != "research" {
		t.Errorf("expected tag 'research', got '%s'", target.Tag)
	}
	if target.MaxExamples != 5 {
		t.Errorf("expected max 5, got %d", target.MaxExamples)
	}
	if target.IncludeNegative != false {
		t.Errorf("expected include_negative false, got %v", target.IncludeNegative)
	}
}

func TestParser_FindExamplesTargets_Defaults(t *testing.T) {
	content := `<!-- trajectory-examples:start tag="test" -->
Content
<!-- trajectory-examples:end -->
`
	filePath := writeTempFile(t, content)
	defer os.Remove(filePath)

	p := NewParser()
	targets, err := p.FindExamplesTargets(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	target := targets[0]
	if target.MaxExamples != 3 {
		t.Errorf("expected default max 3, got %d", target.MaxExamples)
	}
	if target.IncludeNegative != true {
		t.Errorf("expected default include_negative true, got %v", target.IncludeNegative)
	}
}

func TestParser_ReplaceTarget(t *testing.T) {
	content := `# Test File

## Best Practices

<!-- trajectory-optimize:start tag="research" min_sessions=10 -->
Old content line 1
Old content line 2
<!-- trajectory-optimize:end -->

## Other Section
Some other content.
`
	filePath := writeTempFile(t, content)
	defer os.Remove(filePath)

	p := NewParser()
	targets, err := p.FindTargets(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newContent := "New content line 1\nNew content line 2\nNew content line 3"
	if err := p.ReplaceTarget(filePath, targets[0], newContent); err != nil {
		t.Fatalf("replace error: %v", err)
	}

	// Read the file back
	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	resultStr := string(result)

	// Check that old content is gone and new content is present
	if strings.Contains(resultStr, "Old content") {
		t.Error("old content still present")
	}
	if !strings.Contains(resultStr, "New content line 1") {
		t.Error("new content not present")
	}
	if !strings.Contains(resultStr, "New content line 3") {
		t.Error("new content line 3 not present")
	}

	// Check that markers are preserved
	if !strings.Contains(resultStr, "trajectory-optimize:start") {
		t.Error("start marker not preserved")
	}
	if !strings.Contains(resultStr, "trajectory-optimize:end") {
		t.Error("end marker not preserved")
	}

	// Check that surrounding content is preserved
	if !strings.Contains(resultStr, "# Test File") {
		t.Error("header not preserved")
	}
	if !strings.Contains(resultStr, "## Other Section") {
		t.Error("other section not preserved")
	}
}

func TestParser_ReplaceTarget_PreservesAttributes(t *testing.T) {
	content := `<!-- trajectory-optimize:start tag="test" min_sessions=20 -->
old
<!-- trajectory-optimize:end -->
`
	filePath := writeTempFile(t, content)
	defer os.Remove(filePath)

	p := NewParser()
	targets, err := p.FindTargets(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := p.ReplaceTarget(filePath, targets[0], "new"); err != nil {
		t.Fatalf("replace error: %v", err)
	}

	// Parse again to verify markers are intact
	targets2, err := p.FindTargets(filePath)
	if err != nil {
		t.Fatalf("failed to re-parse: %v", err)
	}

	if len(targets2) != 1 {
		t.Fatalf("expected 1 target after replace, got %d", len(targets2))
	}
	if targets2[0].Tag != "test" {
		t.Errorf("tag changed after replace: %s", targets2[0].Tag)
	}
	if targets2[0].MinSessions != 20 {
		t.Errorf("min_sessions changed after replace: %d", targets2[0].MinSessions)
	}
	if targets2[0].Content != "new" {
		t.Errorf("content not updated: %s", targets2[0].Content)
	}
}

func TestParser_ReplaceTarget_MultipleTargets(t *testing.T) {
	content := `<!-- trajectory-optimize:start tag="first" -->
first content
<!-- trajectory-optimize:end -->

Middle content.

<!-- trajectory-optimize:start tag="second" -->
second content
<!-- trajectory-optimize:end -->
`
	filePath := writeTempFile(t, content)
	defer os.Remove(filePath)

	p := NewParser()
	targets, err := p.FindTargets(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Replace only the second target
	if err := p.ReplaceTarget(filePath, targets[1], "NEW SECOND"); err != nil {
		t.Fatalf("replace error: %v", err)
	}

	// Parse again
	newTargets, err := p.FindTargets(filePath)
	if err != nil {
		t.Fatalf("failed to re-parse: %v", err)
	}

	if len(newTargets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(newTargets))
	}

	// First should be unchanged
	if newTargets[0].Content != "first content" {
		t.Errorf("first target content changed: %s", newTargets[0].Content)
	}

	// Second should have new content
	if newTargets[1].Content != "NEW SECOND" {
		t.Errorf("second target content not updated: %s", newTargets[1].Content)
	}
}

func TestParser_MixedTargetTypes(t *testing.T) {
	content := `# Mixed File

<!-- trajectory-optimize:start tag="research" -->
Optimization target
<!-- trajectory-optimize:end -->

<!-- trajectory-examples:start tag="research" max=2 -->
Examples target
<!-- trajectory-examples:end -->
`
	filePath := writeTempFile(t, content)
	defer os.Remove(filePath)

	p := NewParser()

	optTargets, err := p.FindTargets(filePath)
	if err != nil {
		t.Fatalf("FindTargets error: %v", err)
	}

	exTargets, err := p.FindExamplesTargets(filePath)
	if err != nil {
		t.Fatalf("FindExamplesTargets error: %v", err)
	}

	if len(optTargets) != 1 {
		t.Errorf("expected 1 optimization target, got %d", len(optTargets))
	}
	if len(exTargets) != 1 {
		t.Errorf("expected 1 examples target, got %d", len(exTargets))
	}

	if optTargets[0].Tag != "research" {
		t.Errorf("optimization target wrong tag: %s", optTargets[0].Tag)
	}
	if exTargets[0].Tag != "research" {
		t.Errorf("examples target wrong tag: %s", exTargets[0].Tag)
	}
}

func TestParser_EmptyContent(t *testing.T) {
	content := `<!-- trajectory-optimize:start tag="empty" -->
<!-- trajectory-optimize:end -->
`
	filePath := writeTempFile(t, content)
	defer os.Remove(filePath)

	p := NewParser()
	targets, err := p.FindTargets(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Content != "" {
		t.Errorf("expected empty content, got: %q", targets[0].Content)
	}
}

// writeTempFile creates a temporary file with the given content for testing.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return filePath
}
