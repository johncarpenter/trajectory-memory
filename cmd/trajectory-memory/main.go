// Package main provides the entry point for trajectory-memory.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/johncarpenter/trajectory-memory/internal/config"
	"github.com/johncarpenter/trajectory-memory/internal/ingestion"
	"github.com/johncarpenter/trajectory-memory/internal/installer"
	"github.com/johncarpenter/trajectory-memory/internal/mcp"
	"github.com/johncarpenter/trajectory-memory/internal/optimizer"
	"github.com/johncarpenter/trajectory-memory/internal/store"
	"github.com/johncarpenter/trajectory-memory/internal/summarize"
	"github.com/johncarpenter/trajectory-memory/internal/types"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "serve":
		cmdServe(args)
	case "install":
		cmdInstall(args)
	case "uninstall":
		cmdUninstall(args)
	case "list":
		cmdList(args)
	case "show":
		cmdShow(args)
	case "score":
		cmdScore(args)
	case "search":
		cmdSearch(args)
	case "export":
		cmdExport(args)
	case "import":
		cmdImport(args)
	case "stats":
		cmdStats(args)
	case "prune":
		cmdPrune(args)
	case "optimize":
		cmdOptimize(args)
	case "curate":
		cmdCurate(args)
	case "trigger":
		cmdTrigger(args)
	case "version":
		fmt.Printf("trajectory-memory %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`trajectory-memory - Record, score, and learn from agent execution traces

Usage:
  trajectory-memory <command> [options]

Commands:
  serve                   Run MCP server on stdio (how Claude Code launches it)
  install [--global]      Install hooks into Claude Code settings
  uninstall [--global]    Remove hooks from Claude Code settings
  list [--limit N]        Show recent sessions with scores
  show <session-id>       Print full trajectory for a session
  score <session-id> <score> [--notes "..."]  Score or re-score a session
  search <query> [--limit N] [--min-score F]  Search past sessions
  export [--output file.jsonl]  Export all sessions to JSONL
  import <file.jsonl>     Import sessions from JSONL
  stats                   Summary statistics
  prune [--before DATE] [--min-score F]  Delete old or low-scoring sessions

Context Optimization:
  optimize propose <file> [--tag TAG]   Analyze trajectories and propose optimized content
  optimize apply <record-id>            Apply a proposed optimization
  optimize reject <record-id>           Reject a proposed optimization
  optimize rollback <record-id>         Revert an applied optimization
  optimize history [--file F] [--tag T] Show optimization history
  optimize diff <record-id>             Show diff for an optimization
  curate <tag> [--max N] [--file F]     Curate best examples for a tag
  trigger status                        Show trigger configuration
  trigger configure [flags]             Update trigger settings
  trigger watch <file>                  Add file to watch list

  version                 Print version information
  help                    Show this help message

Environment Variables:
  TM_DB_PATH       Database path (default: ~/.trajectory-memory/tm.db)
  TM_SOCKET_PATH   Unix socket path (default: /tmp/trajectory-memory.sock)
  TM_DATA_DIR      Data directory (default: ~/.trajectory-memory)
`)
}

func openStore() (*store.BoltStore, error) {
	cfg := config.Load()
	if err := cfg.EnsureDataDir(); err != nil {
		return nil, err
	}
	return store.NewBoltStore(cfg.DBPath)
}

func cmdServe(args []string) {
	cfg := config.Load()
	if err := cfg.EnsureDataDir(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	s, err := store.NewBoltStore(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	// Start ingestion server
	ingestionServer := ingestion.NewServer(s, cfg.SocketPath)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ingestionServer.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to start ingestion server: %v\n", err)
	}

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		cancel()
	}()

	// Run MCP server
	mcpServer := mcp.NewServer(s, cfg.SocketPath, version)
	if err := mcpServer.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	global := fs.Bool("global", false, "Install to user-level settings instead of project-level")
	fs.Parse(args)

	cfg := config.Load()
	inst := installer.NewInstaller(cfg.DataDir)

	opts := installer.InstallOptions{Global: *global}

	if inst.IsInstalled(opts) {
		fmt.Println("trajectory-memory is already installed")
		return
	}

	if err := inst.Install(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("trajectory-memory installed successfully!")
	fmt.Println()
	fmt.Println("Hook script installed to:", inst.GetHookPath())
	fmt.Println()
	fmt.Println("Add this to your Claude Code settings to enable the MCP server:")
	fmt.Println(inst.GetMCPConfig())
	fmt.Println()
	fmt.Println("Add this to your CLAUDE.md for usage instructions:")
	fmt.Println(inst.GetClaudeMDSnippet())
}

func cmdUninstall(args []string) {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	global := fs.Bool("global", false, "Uninstall from user-level settings")
	fs.Parse(args)

	cfg := config.Load()
	inst := installer.NewInstaller(cfg.DataDir)

	opts := installer.InstallOptions{Global: *global}

	if err := inst.Uninstall(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("trajectory-memory uninstalled successfully!")
	fmt.Println()
	fmt.Printf("Note: Database not removed. To delete all data: rm -rf %s\n", cfg.DataDir)
}

func cmdList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	limit := fs.Int("limit", 10, "Maximum number of sessions to show")
	fs.Parse(args)

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	sessions, err := s.ListSessions(*limit, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTASK\tSTEPS\tSCORE\tSTATUS\tDATE")
	fmt.Fprintln(w, "--\t----\t-----\t-----\t------\t----")

	for _, sess := range sessions {
		task := truncate(sess.TaskPrompt, 40)
		scoreStr := "-"
		if sess.Score != nil {
			scoreStr = fmt.Sprintf("%.2f", *sess.Score)
		}
		date := sess.StartedAt.Format("2006-01-02")
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n",
			sess.ID[:12], task, sess.StepCount, scoreStr, sess.Status, date)
	}
	w.Flush()
}

func cmdShow(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: trajectory-memory show <session-id>")
		os.Exit(1)
	}

	sessionID := args[0]

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	// Support partial ID matching
	session, err := findSession(s, sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	opts := summarize.FormatOptions{
		IncludeSummarizationPrompt: false,
		Verbose:                    true,
	}
	fmt.Println(summarize.FormatTrajectoryWithOptions(session, opts))
}

func cmdScore(args []string) {
	fs := flag.NewFlagSet("score", flag.ExitOnError)
	notes := fs.String("notes", "", "Notes about the scoring")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: trajectory-memory score <session-id> <score> [--notes \"...\"]")
		os.Exit(1)
	}

	sessionID := remaining[0]
	scoreVal, err := strconv.ParseFloat(remaining[1], 64)
	if err != nil || scoreVal < 0 || scoreVal > 1 {
		fmt.Fprintln(os.Stderr, "Error: score must be a number between 0.0 and 1.0")
		os.Exit(1)
	}

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	session, err := findSession(s, sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	outcome := types.Outcome{
		Score:    scoreVal,
		Notes:    *notes,
		ScoredAt: time.Now(),
	}

	if err := s.SetOutcome(session.ID, outcome); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Session %s scored %.2f\n", session.ID[:12], scoreVal)
}

func cmdSearch(args []string) {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	limit := fs.Int("limit", 5, "Maximum number of results")
	minScore := fs.Float64("min-score", -1, "Minimum score filter")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: trajectory-memory search <query> [--limit N] [--min-score F]")
		os.Exit(1)
	}

	query := strings.Join(remaining, " ")

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	results, err := s.SearchSessions(query, *limit*2) // Get more to allow filtering
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Filter by min score if specified
	if *minScore >= 0 {
		filtered := make([]types.SessionMetadata, 0)
		for _, r := range results {
			if r.Score != nil && *r.Score >= *minScore {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// Limit results
	if len(results) > *limit {
		results = results[:*limit]
	}

	if len(results) == 0 {
		fmt.Println("No matching sessions found")
		return
	}

	for _, r := range results {
		scoreStr := "unscored"
		if r.Score != nil {
			scoreStr = fmt.Sprintf("%.2f", *r.Score)
		}
		fmt.Printf("\n[%s] %s (score: %s)\n", r.ID[:12], r.StartedAt.Format("2006-01-02"), scoreStr)
		fmt.Printf("  Task: %s\n", truncate(r.TaskPrompt, 60))
		if r.Summary != "" {
			fmt.Printf("  Summary: %s\n", truncate(r.Summary, 80))
		}
		if len(r.Tags) > 0 {
			fmt.Printf("  Tags: %v\n", r.Tags)
		}
	}
}

func cmdExport(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	output := fs.String("output", "", "Output file (default: stdout)")
	fs.Parse(args)

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	var out *os.File
	if *output != "" {
		out, err = os.Create(*output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
			os.Exit(1)
		}
		defer out.Close()
	} else {
		out = os.Stdout
	}

	if err := s.ExportAll(out); err != nil {
		fmt.Fprintf(os.Stderr, "Error exporting: %v\n", err)
		os.Exit(1)
	}

	if *output != "" {
		fmt.Fprintf(os.Stderr, "Exported to %s\n", *output)
	}
}

func cmdImport(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: trajectory-memory import <file.jsonl>")
		os.Exit(1)
	}

	inputPath := args[0]

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	file, err := os.Open(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	if err := s.ImportAll(file); err != nil {
		fmt.Fprintf(os.Stderr, "Error importing: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Imported sessions from %s\n", inputPath)
}

func cmdStats(args []string) {
	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	sessions, err := s.ListSessions(1000, 0) // Get all sessions
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	total := len(sessions)
	scored := 0
	var scoreSum float64
	var scoreCounts [5]int // 0-0.2, 0.2-0.4, 0.4-0.6, 0.6-0.8, 0.8-1.0
	tagCounts := make(map[string]int)
	statusCounts := make(map[string]int)

	for _, sess := range sessions {
		statusCounts[sess.Status]++
		for _, tag := range sess.Tags {
			tagCounts[tag]++
		}
		if sess.Score != nil {
			scored++
			scoreSum += *sess.Score
			bucket := int(*sess.Score * 5)
			if bucket > 4 {
				bucket = 4
			}
			scoreCounts[bucket]++
		}
	}

	fmt.Println("=== Trajectory Memory Statistics ===")
	fmt.Println()
	fmt.Printf("Total sessions: %d\n", total)
	fmt.Printf("Scored sessions: %d\n", scored)
	if scored > 0 {
		fmt.Printf("Average score: %.2f\n", scoreSum/float64(scored))
	}
	fmt.Println()

	fmt.Println("Status breakdown:")
	for status, count := range statusCounts {
		fmt.Printf("  %s: %d\n", status, count)
	}
	fmt.Println()

	if scored > 0 {
		fmt.Println("Score distribution:")
		ranges := []string{"0.0-0.2", "0.2-0.4", "0.4-0.6", "0.6-0.8", "0.8-1.0"}
		for i, count := range scoreCounts {
			if count > 0 {
				fmt.Printf("  %s: %d\n", ranges[i], count)
			}
		}
		fmt.Println()
	}

	if len(tagCounts) > 0 {
		fmt.Println("Top tags:")
		// Simple top 10 - not sorted, but shows counts
		shown := 0
		for tag, count := range tagCounts {
			if shown >= 10 {
				break
			}
			fmt.Printf("  %s: %d\n", tag, count)
			shown++
		}
	}
}

func cmdPrune(args []string) {
	fs := flag.NewFlagSet("prune", flag.ExitOnError)
	before := fs.String("before", "", "Delete sessions before this date (YYYY-MM-DD)")
	minScore := fs.Float64("min-score", -1, "Delete sessions with score below this (requires --confirm)")
	confirm := fs.Bool("confirm", false, "Actually delete (dry run without this)")
	fs.Parse(args)

	if *before == "" && *minScore < 0 {
		fmt.Fprintln(os.Stderr, "Usage: trajectory-memory prune [--before DATE] [--min-score F] [--confirm]")
		fmt.Fprintln(os.Stderr, "At least one of --before or --min-score is required")
		os.Exit(1)
	}

	var beforeDate time.Time
	if *before != "" {
		var err error
		beforeDate, err = time.Parse("2006-01-02", *before)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing date: %v\n", err)
			os.Exit(1)
		}
	}

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	sessions, err := s.ListSessions(10000, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var toDelete []string
	for _, sess := range sessions {
		shouldDelete := false

		if *before != "" && sess.StartedAt.Before(beforeDate) {
			shouldDelete = true
		}

		if *minScore >= 0 && sess.Score != nil && *sess.Score < *minScore {
			shouldDelete = true
		}

		if shouldDelete {
			toDelete = append(toDelete, sess.ID)
		}
	}

	if len(toDelete) == 0 {
		fmt.Println("No sessions match the criteria")
		return
	}

	if !*confirm {
		fmt.Printf("Would delete %d sessions (use --confirm to actually delete):\n", len(toDelete))
		for i, id := range toDelete {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(toDelete)-10)
				break
			}
			fmt.Printf("  %s\n", id[:12])
		}
		return
	}

	deleted := 0
	for _, id := range toDelete {
		if err := s.DeleteSession(id); err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting %s: %v\n", id[:12], err)
		} else {
			deleted++
		}
	}

	fmt.Printf("Deleted %d sessions\n", deleted)
}

// findSession finds a session by full or partial ID.
func findSession(s *store.BoltStore, partialID string) (*types.Session, error) {
	// Try exact match first
	session, err := s.GetSession(partialID)
	if err == nil {
		return session, nil
	}

	// Try partial match
	sessions, err := s.ListSessions(100, 0)
	if err != nil {
		return nil, err
	}

	var matches []*types.Session
	for _, sess := range sessions {
		if strings.HasPrefix(sess.ID, partialID) {
			full, err := s.GetSession(sess.ID)
			if err == nil {
				matches = append(matches, full)
			}
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("session not found: %s", partialID)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("ambiguous session ID '%s' matches %d sessions", partialID, len(matches))
	}

	return matches[0], nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Context optimization commands

func cmdOptimize(args []string) {
	if len(args) < 1 {
		printOptimizeUsage()
		os.Exit(1)
	}

	subCmd := args[0]
	subArgs := args[1:]

	switch subCmd {
	case "propose":
		cmdOptimizePropose(subArgs)
	case "apply":
		cmdOptimizeApply(subArgs)
	case "reject":
		cmdOptimizeReject(subArgs)
	case "rollback":
		cmdOptimizeRollback(subArgs)
	case "history":
		cmdOptimizeHistory(subArgs)
	case "diff":
		cmdOptimizeDiff(subArgs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown optimize subcommand: %s\n", subCmd)
		printOptimizeUsage()
		os.Exit(1)
	}
}

func printOptimizeUsage() {
	fmt.Print(`Usage: trajectory-memory optimize <subcommand>

Subcommands:
  propose <file> [--tag TAG]   Analyze trajectories and propose optimized content
  apply <record-id>            Apply a proposed optimization
  reject <record-id>           Reject a proposed optimization
  rollback <record-id>         Revert an applied optimization
  history [--file F] [--tag T] Show optimization history
  diff <record-id>             Show diff for an optimization
`)
}

func cmdOptimizePropose(args []string) {
	fs := flag.NewFlagSet("optimize propose", flag.ExitOnError)
	tag := fs.String("tag", "", "Specific tag to optimize")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: trajectory-memory optimize propose <file> [--tag TAG]")
		os.Exit(1)
	}

	filePath := remaining[0]

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	opt := optimizer.NewOptimizer(s)
	parser := optimizer.NewParser()

	// Find targets
	targets, err := parser.FindTargets(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing file: %v\n", err)
		os.Exit(1)
	}

	if len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "No optimization targets found in file")
		os.Exit(1)
	}

	// Filter by tag if specified
	if *tag != "" {
		var filtered []types.OptimizationTarget
		for _, t := range targets {
			if t.Tag == *tag {
				filtered = append(filtered, t)
			}
		}
		targets = filtered
		if len(targets) == 0 {
			fmt.Fprintf(os.Stderr, "No target found for tag: %s\n", *tag)
			os.Exit(1)
		}
	}

	// Analyze each target
	for _, target := range targets {
		result, err := opt.Propose(target)
		if err != nil {
			fmt.Printf("\n## Target: %s (SKIPPED)\n%v\n", target.Tag, err)
			continue
		}

		fmt.Println(optimizer.FormatAnalysisForCLI(filePath, target, result.Analysis))
	}
}

func cmdOptimizeApply(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: trajectory-memory optimize apply <record-id>")
		os.Exit(1)
	}

	recordID := args[0]

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	opt := optimizer.NewOptimizer(s)

	if err := opt.Apply(recordID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Optimization %s applied successfully\n", recordID)
}

func cmdOptimizeReject(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: trajectory-memory optimize reject <record-id>")
		os.Exit(1)
	}

	recordID := args[0]

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	opt := optimizer.NewOptimizer(s)

	if err := opt.Reject(recordID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Optimization %s rejected\n", recordID)
}

func cmdOptimizeRollback(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: trajectory-memory optimize rollback <record-id>")
		os.Exit(1)
	}

	recordID := args[0]

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	opt := optimizer.NewOptimizer(s)

	if err := opt.Rollback(recordID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Optimization %s rolled back\n", recordID)
}

func cmdOptimizeHistory(args []string) {
	fs := flag.NewFlagSet("optimize history", flag.ExitOnError)
	filePath := fs.String("file", "", "Filter by file path")
	tag := fs.String("tag", "", "Filter by tag")
	limit := fs.Int("limit", 10, "Maximum number of records")
	fs.Parse(args)

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	opt := optimizer.NewOptimizer(s)

	records, err := opt.History(*filePath, *tag, *limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(optimizer.FormatHistoryForCLI(records))
}

func cmdOptimizeDiff(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: trajectory-memory optimize diff <record-id>")
		os.Exit(1)
	}

	recordID := args[0]

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	opt := optimizer.NewOptimizer(s)

	record, err := opt.GetRecord(recordID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Optimization: %s\n", record.ID)
	fmt.Printf("File: %s\n", record.TargetFile)
	fmt.Printf("Tag: %s\n", record.Tag)
	fmt.Printf("Status: %s\n", record.Status)
	fmt.Printf("Created: %s\n\n", record.CreatedAt.Format(time.RFC3339))
	fmt.Println("Diff:")
	fmt.Println(record.Diff)
}

func cmdCurate(args []string) {
	fs := flag.NewFlagSet("curate", flag.ExitOnError)
	maxExamples := fs.Int("max", 3, "Maximum positive examples")
	filePath := fs.String("file", "", "File to apply curated examples to")
	includeNegative := fs.Bool("include-negative", true, "Include negative example")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: trajectory-memory curate <tag> [--max N] [--file F]")
		os.Exit(1)
	}

	tag := remaining[0]

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	analyzer := optimizer.NewAnalyzer(s)
	analysis, err := analyzer.Analyze(tag, 3) // Low minimum for curation
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Limit examples
	examples := analysis.CuratedExamples
	if len(examples) > *maxExamples+1 {
		examples = examples[:*maxExamples+1]
	}

	// Format
	content := formatCuratedExamplesForCLI(examples, *includeNegative)
	fmt.Println(content)

	// Apply to file if specified
	if *filePath != "" {
		parser := optimizer.NewParser()
		targets, err := parser.FindExamplesTargets(*filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing file: %v\n", err)
			os.Exit(1)
		}

		var target *types.ExamplesTarget
		for _, t := range targets {
			if t.Tag == tag {
				target = &t
				break
			}
		}

		if target == nil {
			fmt.Fprintf(os.Stderr, "No examples target found for tag '%s' in %s\n", tag, *filePath)
			os.Exit(1)
		}

		if err := parser.ReplaceExamplesTarget(*filePath, *target, content); err != nil {
			fmt.Fprintf(os.Stderr, "Error applying examples: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nExamples applied to %s\n", *filePath)
	}
}

func formatCuratedExamplesForCLI(examples []types.CuratedExample, includeNegative bool) string {
	var buf strings.Builder

	buf.WriteString("### What Works Well (from past sessions)\n\n")

	for _, ex := range examples {
		if ex.Score >= 0.75 {
			buf.WriteString(fmt.Sprintf("**Example: %s** (scored %.0f%%)\n",
				truncate(ex.TaskPrompt, 50), ex.Score*100))
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
					truncate(ex.TaskPrompt, 50), ex.Score*100))
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

func cmdTrigger(args []string) {
	if len(args) < 1 {
		printTriggerUsage()
		os.Exit(1)
	}

	subCmd := args[0]
	subArgs := args[1:]

	switch subCmd {
	case "status":
		cmdTriggerStatus()
	case "configure":
		cmdTriggerConfigure(subArgs)
	case "watch":
		cmdTriggerWatch(subArgs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown trigger subcommand: %s\n", subCmd)
		printTriggerUsage()
		os.Exit(1)
	}
}

func printTriggerUsage() {
	fmt.Print(`Usage: trajectory-memory trigger <subcommand>

Subcommands:
  status                        Show trigger configuration
  configure [flags]             Update trigger settings
    --enabled=true|false        Enable/disable auto-optimization
    --threshold=N               Sessions before triggering
    --min-gap=F                 Minimum score improvement gap
  watch <file>                  Add file to watch list
`)
}

func cmdTriggerStatus() {
	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	config, err := s.GetTriggerConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Trigger Configuration ===")
	fmt.Printf("Enabled: %v\n", config.Enabled)
	fmt.Printf("Session Threshold: %d\n", config.SessionThreshold)
	fmt.Printf("Min Score Gap: %.2f\n", config.MinScoreGap)
	fmt.Printf("Watch Files: %v\n", config.WatchFiles)
}

func cmdTriggerConfigure(args []string) {
	fs := flag.NewFlagSet("trigger configure", flag.ExitOnError)
	enabled := fs.Bool("enabled", false, "Enable auto-optimization")
	threshold := fs.Int("threshold", 0, "Sessions before triggering")
	minGap := fs.Float64("min-gap", 0, "Minimum score improvement gap")
	fs.Parse(args)

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	config, err := s.GetTriggerConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Apply updates based on what was set
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "enabled":
			config.Enabled = *enabled
		case "threshold":
			config.SessionThreshold = *threshold
		case "min-gap":
			config.MinScoreGap = *minGap
		}
	})

	if err := s.SaveTriggerConfig(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Trigger configuration updated:")
	fmt.Printf("  Enabled: %v\n", config.Enabled)
	fmt.Printf("  Session Threshold: %d\n", config.SessionThreshold)
	fmt.Printf("  Min Score Gap: %.2f\n", config.MinScoreGap)
	fmt.Printf("  Watch Files: %v\n", config.WatchFiles)
}

func cmdTriggerWatch(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: trajectory-memory trigger watch <file>")
		os.Exit(1)
	}

	filePath := args[0]

	s, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	config, err := s.GetTriggerConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Check if already watching
	for _, f := range config.WatchFiles {
		if f == filePath {
			fmt.Printf("Already watching: %s\n", filePath)
			return
		}
	}

	config.WatchFiles = append(config.WatchFiles, filePath)

	if err := s.SaveTriggerConfig(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Now watching: %s\n", filePath)
	fmt.Printf("Watch files: %v\n", config.WatchFiles)
}
