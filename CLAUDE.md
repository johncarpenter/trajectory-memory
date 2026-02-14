# trajectory-memory

A Go binary that acts as both an MCP server (for Claude Code integration) and a CLI tool for managing trajectory memory.

## Project Structure

```
trajectory-memory/
├── cmd/trajectory-memory/main.go    # CLI entrypoint with subcommand routing
├── internal/
│   ├── types/types.go               # Core data structures (Session, TrajectoryStep, Outcome, OptimizationRecord)
│   ├── store/store.go               # BBolt-backed persistence layer
│   ├── store/optimization_store.go  # Optimization record persistence
│   ├── config/config.go             # Configuration from env vars
│   ├── ingestion/server.go          # Unix socket HTTP server for hook events
│   ├── mcp/server.go                # MCP JSON-RPC server over stdio
│   ├── installer/installer.go       # Hook installation/uninstallation
│   ├── summarize/summarize.go       # Trajectory formatting for summarization
│   └── optimizer/                   # Context optimization engine
│       ├── parser.go                # Markdown marker parsing
│       ├── analyzer.go              # Trajectory pattern analysis
│       └── optimizer.go             # Optimization proposal/apply/rollback
└── examples/                        # Knowledge work examples
    ├── market-research/
    ├── meeting-summary/
    └── code-review-analysis/
```

## Building

```bash
go build -o trajectory-memory ./cmd/trajectory-memory
```

## Testing

```bash
go test ./... -v
```

## Key Concepts

- **Session**: A recording of tool invocations during a task
- **TrajectoryStep**: A single tool call with input/output summaries
- **Outcome**: Score (0.0-1.0) and notes for a completed session

## Store Operations

The BBolt store uses three buckets:
- `sessions`: Full session JSON by ID
- `active`: Current recording session ID
- `index`: SessionMetadata for fast listing

## MCP Tools

### Core Tools
- `trajectory_start`: Begin recording a session
- `trajectory_stop`: Stop recording, returns trajectory for summarization
- `trajectory_status`: Check if recording is active
- `trajectory_search`: Find past sessions by keyword
- `trajectory_list`: List recent sessions
- `trajectory_score`: Score a completed session
- `trajectory_summarize`: Store model-generated summary

### Optimization Tools
- `trajectory_optimize_propose`: Analyze trajectories and propose optimized content
- `trajectory_optimize_save`: Save generated optimization as proposal
- `trajectory_optimize_apply`: Apply a proposed optimization
- `trajectory_optimize_rollback`: Revert an applied optimization
- `trajectory_optimize_history`: List optimization history
- `trajectory_curate_examples`: Curate best examples for few-shot learning
- `trajectory_curate_apply`: Apply curated examples to a file
- `trajectory_trigger_status`: Check trigger configuration
- `trajectory_trigger_configure`: Configure auto-optimization triggers

## CLI Commands

### Core
- `serve`: Run MCP server on stdio
- `install/uninstall`: Manage hooks
- `list/show/search`: Browse sessions
- `score`: Score a session
- `export/import`: Data management
- `stats/prune`: Maintenance

### Context Optimization
- `optimize propose <file> [--tag TAG]`: Analyze and propose
- `optimize apply/reject/rollback <record-id>`: Manage proposals
- `optimize history/diff`: Review history
- `curate <tag> [--file F]`: Curate examples
- `trigger status/configure/watch`: Configure triggers

## Development Guidelines

- Keep dependencies minimal (stdlib + BBolt only)
- All stdout is MCP protocol in serve mode - use stderr for logging
- Hook scripts must be fire-and-forget resilient
- Use table-driven tests
