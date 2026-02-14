// Package optimizer provides context optimization functionality.
package optimizer

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/johncarpenter/trajectory-memory/internal/types"
)

var (
	// ErrUnpairedMarkers is returned when start and end markers don't match.
	ErrUnpairedMarkers = errors.New("unpaired optimization markers")
	// ErrNestedMarkers is returned when markers are nested.
	ErrNestedMarkers = errors.New("nested optimization markers are not supported")
	// ErrMissingTag is returned when the tag attribute is missing.
	ErrMissingTag = errors.New("missing required 'tag' attribute in optimization marker")
	// ErrInvalidMarker is returned when a marker is malformed.
	ErrInvalidMarker = errors.New("invalid marker format")
)

// Marker patterns
var (
	// <!-- trajectory-optimize:start tag="research" min_sessions=10 -->
	optimizeStartPattern = regexp.MustCompile(`<!--\s*trajectory-optimize:start\s+(.+?)\s*-->`)
	optimizeEndPattern   = regexp.MustCompile(`<!--\s*trajectory-optimize:end\s*-->`)

	// <!-- trajectory-examples:start tag="research" max=3 include_negative=true -->
	examplesStartPattern = regexp.MustCompile(`<!--\s*trajectory-examples:start\s+(.+?)\s*-->`)
	examplesEndPattern   = regexp.MustCompile(`<!--\s*trajectory-examples:end\s*-->`)

	// Attribute patterns
	tagAttrPattern          = regexp.MustCompile(`tag\s*=\s*"([^"]+)"`)
	minSessionsAttrPattern  = regexp.MustCompile(`min_sessions\s*=\s*(\d+)`)
	maxAttrPattern          = regexp.MustCompile(`max\s*=\s*(\d+)`)
	includeNegativeAttrPattern = regexp.MustCompile(`include_negative\s*=\s*(true|false)`)
)

// Parser provides methods for finding and replacing optimization targets in markdown files.
type Parser struct{}

// NewParser creates a new Parser instance.
func NewParser() *Parser {
	return &Parser{}
}

// FindTargets scans a markdown file for optimization target markers.
func (p *Parser) FindTargets(filePath string) ([]types.OptimizationTarget, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var targets []types.OptimizationTarget
	var currentStart *pendingTarget
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Check for start marker
		if match := optimizeStartPattern.FindStringSubmatch(line); match != nil {
			if currentStart != nil {
				return nil, fmt.Errorf("%w: nested start marker at line %d", ErrNestedMarkers, lineNum)
			}

			attrs := match[1]
			tag, minSessions, err := parseOptimizeAttrs(attrs)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNum, err)
			}

			currentStart = &pendingTarget{
				filePath:    filePath,
				tag:         tag,
				minSessions: minSessions,
				startLine:   lineNum,
				content:     strings.Builder{},
			}
			continue
		}

		// Check for end marker
		if optimizeEndPattern.MatchString(line) {
			if currentStart == nil {
				return nil, fmt.Errorf("%w: end marker without start at line %d", ErrUnpairedMarkers, lineNum)
			}

			targets = append(targets, types.OptimizationTarget{
				FilePath:    currentStart.filePath,
				Tag:         currentStart.tag,
				MinSessions: currentStart.minSessions,
				StartLine:   currentStart.startLine,
				EndLine:     lineNum,
				Content:     strings.TrimSpace(currentStart.content.String()),
			})
			currentStart = nil
			continue
		}

		// Accumulate content if inside a target
		if currentStart != nil {
			if currentStart.content.Len() > 0 {
				currentStart.content.WriteString("\n")
			}
			currentStart.content.WriteString(line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	if currentStart != nil {
		return nil, fmt.Errorf("%w: start marker at line %d without end marker", ErrUnpairedMarkers, currentStart.startLine)
	}

	return targets, nil
}

// FindExamplesTargets scans a markdown file for examples target markers.
func (p *Parser) FindExamplesTargets(filePath string) ([]types.ExamplesTarget, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var targets []types.ExamplesTarget
	var currentStart *pendingExamplesTarget
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Check for start marker
		if match := examplesStartPattern.FindStringSubmatch(line); match != nil {
			if currentStart != nil {
				return nil, fmt.Errorf("%w: nested start marker at line %d", ErrNestedMarkers, lineNum)
			}

			attrs := match[1]
			tag, maxExamples, includeNegative, err := parseExamplesAttrs(attrs)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNum, err)
			}

			currentStart = &pendingExamplesTarget{
				filePath:        filePath,
				tag:             tag,
				maxExamples:     maxExamples,
				includeNegative: includeNegative,
				startLine:       lineNum,
				content:         strings.Builder{},
			}
			continue
		}

		// Check for end marker
		if examplesEndPattern.MatchString(line) {
			if currentStart == nil {
				return nil, fmt.Errorf("%w: end marker without start at line %d", ErrUnpairedMarkers, lineNum)
			}

			targets = append(targets, types.ExamplesTarget{
				FilePath:        currentStart.filePath,
				Tag:             currentStart.tag,
				MaxExamples:     currentStart.maxExamples,
				IncludeNegative: currentStart.includeNegative,
				StartLine:       currentStart.startLine,
				EndLine:         lineNum,
				Content:         strings.TrimSpace(currentStart.content.String()),
			})
			currentStart = nil
			continue
		}

		// Accumulate content if inside a target
		if currentStart != nil {
			if currentStart.content.Len() > 0 {
				currentStart.content.WriteString("\n")
			}
			currentStart.content.WriteString(line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	if currentStart != nil {
		return nil, fmt.Errorf("%w: start marker at line %d without end marker", ErrUnpairedMarkers, currentStart.startLine)
	}

	return targets, nil
}

// ReplaceTarget replaces the content between markers with new content.
// Writes atomically using temp file + rename.
func (p *Parser) ReplaceTarget(filePath string, target types.OptimizationTarget, newContent string) error {
	// Read the entire file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Validate line numbers
	if target.StartLine < 1 || target.EndLine > len(lines) || target.StartLine >= target.EndLine {
		return fmt.Errorf("invalid line range: start=%d, end=%d, total=%d", target.StartLine, target.EndLine, len(lines))
	}

	// Build new content
	var result strings.Builder

	// Lines before start marker (1-indexed, so we use StartLine-1)
	for i := 0; i < target.StartLine; i++ {
		result.WriteString(lines[i])
		result.WriteString("\n")
	}

	// New content
	result.WriteString(newContent)
	result.WriteString("\n")

	// Lines from end marker onwards (EndLine is 1-indexed)
	for i := target.EndLine - 1; i < len(lines); i++ {
		result.WriteString(lines[i])
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	// Write atomically
	return atomicWrite(filePath, result.String())
}

// ReplaceExamplesTarget replaces the content between examples markers with new content.
func (p *Parser) ReplaceExamplesTarget(filePath string, target types.ExamplesTarget, newContent string) error {
	// Read the entire file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Validate line numbers
	if target.StartLine < 1 || target.EndLine > len(lines) || target.StartLine >= target.EndLine {
		return fmt.Errorf("invalid line range: start=%d, end=%d, total=%d", target.StartLine, target.EndLine, len(lines))
	}

	// Build new content
	var result strings.Builder

	// Lines before start marker (1-indexed, so we use StartLine-1)
	for i := 0; i < target.StartLine; i++ {
		result.WriteString(lines[i])
		result.WriteString("\n")
	}

	// New content
	result.WriteString(newContent)
	result.WriteString("\n")

	// Lines from end marker onwards (EndLine is 1-indexed)
	for i := target.EndLine - 1; i < len(lines); i++ {
		result.WriteString(lines[i])
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	// Write atomically
	return atomicWrite(filePath, result.String())
}

// atomicWrite writes content to a file atomically using temp file + rename.
func atomicWrite(filePath, content string) error {
	dir := filepath.Dir(filePath)
	tmpFile, err := os.CreateTemp(dir, ".tmp-")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure cleanup on error
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	// Write content
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Copy permissions from original file if it exists
	if info, err := os.Stat(filePath); err == nil {
		if err := os.Chmod(tmpPath, info.Mode()); err != nil {
			return fmt.Errorf("failed to set permissions: %w", err)
		}
	}

	// Atomic rename
	if err := os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	success = true
	return nil
}

// Helper types for parsing

type pendingTarget struct {
	filePath    string
	tag         string
	minSessions int
	startLine   int
	content     strings.Builder
}

type pendingExamplesTarget struct {
	filePath        string
	tag             string
	maxExamples     int
	includeNegative bool
	startLine       int
	content         strings.Builder
}

// parseOptimizeAttrs extracts tag and min_sessions from attribute string.
func parseOptimizeAttrs(attrs string) (tag string, minSessions int, err error) {
	// Extract tag (required)
	tagMatch := tagAttrPattern.FindStringSubmatch(attrs)
	if tagMatch == nil {
		return "", 0, ErrMissingTag
	}
	tag = tagMatch[1]

	// Extract min_sessions (default 10)
	minSessions = 10
	if match := minSessionsAttrPattern.FindStringSubmatch(attrs); match != nil {
		if n, err := strconv.Atoi(match[1]); err == nil {
			minSessions = n
		}
	}

	return tag, minSessions, nil
}

// parseExamplesAttrs extracts tag, max, and include_negative from attribute string.
func parseExamplesAttrs(attrs string) (tag string, maxExamples int, includeNegative bool, err error) {
	// Extract tag (required)
	tagMatch := tagAttrPattern.FindStringSubmatch(attrs)
	if tagMatch == nil {
		return "", 0, false, ErrMissingTag
	}
	tag = tagMatch[1]

	// Extract max (default 3)
	maxExamples = 3
	if match := maxAttrPattern.FindStringSubmatch(attrs); match != nil {
		if n, err := strconv.Atoi(match[1]); err == nil {
			maxExamples = n
		}
	}

	// Extract include_negative (default true)
	includeNegative = true
	if match := includeNegativeAttrPattern.FindStringSubmatch(attrs); match != nil {
		includeNegative = match[1] == "true"
	}

	return tag, maxExamples, includeNegative, nil
}
