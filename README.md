# trajectory-memory

Record, summarize, score, and retrieve agent execution traces. Enables reinforcement learning over prompt strategies with a fixed model by learning which context assemblies work best for different task types.

## Overview

trajectory-memory integrates with [Claude Code](https://github.com/anthropics/claude-code) to:

1. **Record** every tool invocation during a session
2. **Summarize** trajectories with model-generated descriptions
3. **Score** outcomes (0.0-1.0) to build training signal
4. **Search** past sessions to inform future approaches
5. **Optimize** instructions based on what works

Over time, this creates a feedback loop where high-scoring approaches inform future sessions.

## Installation

### Using Go Install

```bash
go install github.com/johncarpenter/trajectory-memory/cmd/trajectory-memory@v0.1.4
```

### Build from Source

```bash
git clone https://github.com/johncarpenter/trajectory-memory
cd trajectory-memory
go build -o trajectory-memory ./cmd/trajectory-memory
```

## Quick Start

See [QUICKSTART.md](QUICKSTART.md) for a complete setup guide.

**TL;DR:**

```bash
# Install hooks
trajectory-memory install

# Add MCP server to ~/.claude/settings.json
{
  "mcpServers": {
    "trajectory-memory": {
      "command": "trajectory-memory",
      "args": ["serve"]
    }
  }
}

# In Claude Code, start recording
> Start logging this session

# Work normally, then stop
> Stop logging and summarize
```

## CLI Commands

### Core Commands

| Command | Description |
|---------|-------------|
| `serve` | Run MCP server on stdio |
| `install [--global]` | Install hooks into Claude Code settings |
| `uninstall [--global]` | Remove hooks from Claude Code settings |
| `list [--limit N]` | Show recent sessions with scores |
| `show <session-id>` | Print full trajectory |
| `score <id> <score>` | Score a session (0.0-1.0) |
| `search <query>` | Search past sessions |
| `export` | Export all sessions to JSONL |
| `import <file>` | Import sessions from JSONL |
| `stats` | Summary statistics |
| `prune` | Delete old/low-scoring sessions |

### Context Optimization

| Command | Description |
|---------|-------------|
| `optimize propose <file>` | Analyze trajectories and propose optimized content |
| `optimize apply <id>` | Apply a proposed optimization |
| `optimize reject <id>` | Reject a proposed optimization |
| `optimize rollback <id>` | Revert an applied optimization |
| `optimize history` | Show optimization history |
| `optimize diff <id>` | Show diff for an optimization |
| `curate <tag>` | Curate best examples for a tag |
| `trigger status` | Show trigger configuration |
| `trigger configure` | Update trigger settings |
| `trigger watch <file>` | Add file to watch list |

## MCP Tools

When running as an MCP server, these tools are available:

### Session Management
- `trajectory_start` - Begin recording a session
- `trajectory_stop` - Stop recording, returns trajectory for summarization
- `trajectory_status` - Check if recording is active
- `trajectory_search` - Find past sessions by keyword
- `trajectory_list` - List recent sessions
- `trajectory_score` - Score a completed session
- `trajectory_summarize` - Store model-generated summary

### Context Optimization
- `trajectory_optimize_propose` - Analyze trajectories and propose optimized content
- `trajectory_optimize_save` - Save generated optimization as proposal
- `trajectory_optimize_apply` - Apply a proposed optimization
- `trajectory_optimize_rollback` - Revert an applied optimization
- `trajectory_optimize_history` - List optimization history
- `trajectory_curate_examples` - Curate best examples for few-shot learning
- `trajectory_curate_apply` - Apply curated examples to a file
- `trajectory_trigger_status` - Check trigger configuration
- `trajectory_trigger_configure` - Configure auto-optimization triggers

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TM_DB_PATH` | `~/.trajectory-memory/tm.db` | Database location |
| `TM_SOCKET_PATH` | `/tmp/trajectory-memory.sock` | Unix socket for hook communication |
| `TM_DATA_DIR` | `~/.trajectory-memory` | Data directory |

## How It Works

### Recording Flow

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Claude Code    │───>│  Hook Scripts    │───>│  Ingestion      │
│  Tool Calls     │    │  (on each tool)  │    │  Server         │
└─────────────────┘    └──────────────────┘    └────────┬────────┘
                                                        │
                                                        v
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  User Scores    │───>│  MCP Server      │<───│  BBolt Store    │
│  & Feedback     │    │  (trajectory_*)  │    │  (sessions)     │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

1. **Recording**: Hook scripts capture every tool invocation during a Claude Code session
2. **Summarization**: When recording stops, the trajectory is formatted and the model generates a summary
3. **Scoring**: Users rate session outcomes (0.0-1.0) to build a training signal
4. **Search**: Find high-scoring past sessions with similar tasks to inform future approaches
5. **Optimization**: Analyze patterns across sessions to improve instructions

### Scoring Guidelines

| Score | Meaning |
|-------|---------|
| 0.0-0.3 | Task failed or had major issues |
| 0.4-0.6 | Task completed but with problems |
| 0.7-0.8 | Task completed well |
| 0.9-1.0 | Excellent execution, exemplary approach |

### Context Optimization

trajectory-memory can automatically improve instructions based on what works. Add markers to your CLAUDE.md:

```markdown
<!-- trajectory-optimize:start tag="research" min_sessions=10 -->
1. Define your research scope before starting
2. Read all available context files
3. Use 5-8 targeted searches
<!-- trajectory-optimize:end -->
```

After accumulating scored sessions, run:

```bash
trajectory-memory optimize propose CLAUDE.md --tag research
```

## Examples

The `examples/` directory contains sample configurations for different use cases:

- **market-research/** - Research task optimization
- **meeting-summary/** - Meeting summarization patterns
- **code-review-analysis/** - Code review workflows

## Development

### Requirements

- Go 1.21+
- No external dependencies beyond stdlib and [BBolt](https://github.com/etcd-io/bbolt)

### Building

```bash
go build -o trajectory-memory ./cmd/trajectory-memory
```

### Testing

```bash
go test ./... -v
```

### Project Structure

```
trajectory-memory/
├── cmd/trajectory-memory/     # CLI entrypoint
├── internal/
│   ├── types/                 # Core data structures
│   ├── store/                 # BBolt persistence layer
│   ├── config/                # Configuration
│   ├── ingestion/             # Unix socket HTTP server
│   ├── mcp/                   # MCP JSON-RPC server
│   ├── installer/             # Hook installation
│   ├── summarize/             # Trajectory formatting
│   └── optimizer/             # Context optimization
└── examples/                  # Sample configurations
```

## Contributing

Contributions are welcome. Please:

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) for details.
