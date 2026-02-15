# Quick Start Guide

Get trajectory-memory running with Claude Code in 5 minutes.

## Prerequisites

- [Go 1.21+](https://golang.org/dl/) installed
- [Claude Code CLI](https://github.com/anthropics/claude-code) installed

## Step 1: Install trajectory-memory

```bash
go install github.com/johncarpenter/trajectory-memory/cmd/trajectory-memory@v0.2.0
```

Or build from source:

```bash
git clone https://github.com/johncarpenter/trajectory-memory
cd trajectory-memory
go build -o trajectory-memory ./cmd/trajectory-memory
# Move to your PATH
mv trajectory-memory /usr/local/bin/
```

Verify installation:

```bash
trajectory-memory version
```

## Step 2: Install Hooks

trajectory-memory uses hooks to capture tool invocations. Install them with:

```bash
trajectory-memory install
```

This creates hook scripts in `<project>/.trajectory-memory/hooks/` and configures Claude Code to use them.

**Note:** Each project gets its own database at `.trajectory-memory/tm.db` in the project root. Add `.trajectory-memory/` to your `.gitignore`.

To install globally (user-level instead of project-level):

```bash
trajectory-memory install --global
```

## Step 3: Configure MCP Server

Add the MCP server to your Claude Code settings. Edit `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "trajectory-memory": {
      "command": "trajectory-memory",
      "args": ["serve"]
    }
  }
}
```

Restart Claude Code to load the new MCP server.

## Step 4: Install Slash Commands (Optional)

For convenient shortcuts, install the pre-built Claude Code slash commands:

```bash
# Global installation (available in all projects)
cp -r agent_scripts/commands/* ~/.claude/commands/

# Or project-specific
cp -r agent_scripts/commands/* .claude/commands/
```

This enables commands like:
- `/trajectory-start` - Begin recording (supports `--strategy`, `--recommend`, `--rotate`)
- `/trajectory-stop` - Stop and summarize
- `/trajectory-status` - Check recording state
- `/trajectory-list` - Browse past sessions
- `/trajectory-propose` - Suggest CLAUDE.md improvements
- `/trajectory-strategies-analyze` - Analyze strategy performance

## Step 5: Add Instructions to CLAUDE.md

Copy the snippet from `claude-md-snippet.md` to your project's `CLAUDE.md` or global `~/.claude/CLAUDE.md`:

```markdown
## Trajectory Memory Integration

Before starting tasks, search for past approaches:
trajectory_search({ "query": "<task type>", "limit": 3, "min_score": 0.8 })

Start recording for substantive tasks:
trajectory_start({ "task_prompt": "<description>", "tags": ["<type>"] })
```

See the full [claude-md-snippet.md](claude-md-snippet.md) for complete instructions.

## Step 6: Start Recording

In Claude Code, start a recording session:

```
You: Start logging this session - I'm working on implementing user authentication
```

Claude will call `trajectory_start` to begin recording. Work normally - all tool invocations are captured.

## Step 7: Stop and Summarize

When finished:

```
You: Stop logging and summarize what we did
```

Claude will:
1. Call `trajectory_stop` to end recording
2. Review the full trajectory
3. Generate a summary
4. Call `trajectory_summarize` to store it

## Step 8: Score the Session

Score how well the session went (0.0-1.0):

```
You: Score this session 0.85 - completed successfully with clean implementation
```

Or from CLI:

```bash
trajectory-memory score <session-id> 0.85 --notes "Clean implementation"
```

## Using Past Sessions

Search for relevant past sessions when starting new work:

```
You: Search for past sessions about authentication
```

Claude will call `trajectory_search` and use high-scoring sessions to inform approach.

## View Statistics

```bash
# List recent sessions
trajectory-memory list

# Show session details
trajectory-memory show <session-id>

# View statistics
trajectory-memory stats
```

## Context Optimization

After accumulating 10+ scored sessions, optimize your instructions:

```bash
# Analyze and propose optimizations
trajectory-memory optimize propose CLAUDE.md --tag research

# View history
trajectory-memory optimize history

# Apply a proposal
trajectory-memory optimize apply <record-id>
```

Add optimization markers to your CLAUDE.md sections:

```markdown
<!-- trajectory-optimize:research min_sessions=10 -->
Your instructions here...
<!-- /trajectory-optimize:research -->

<!-- trajectory-examples:research max=3 -->
(curated examples appear here after scoring)
<!-- /trajectory-examples:research -->
```

## Example Workflow

```
Day 1: Record 5 research sessions, score them
Day 2: Record 5 more sessions, patterns emerge
Day 3: Run optimize propose, apply improvements
Day 4: New sessions benefit from optimized instructions
```

## Troubleshooting

### MCP Server Not Loading

1. Check `~/.claude/settings.json` is valid JSON
2. Ensure `trajectory-memory` is in your PATH
3. Restart Claude Code

### Hooks Not Firing

1. Run `trajectory-memory install` again
2. Check hook scripts exist in `.trajectory-memory/hooks/` (in project root)
3. Verify hooks are configured in Claude Code settings

### Database Issues

```bash
# Check database location (per-project)
ls .trajectory-memory/tm.db

# Export and reimport
trajectory-memory export --output backup.jsonl
rm .trajectory-memory/tm.db
trajectory-memory import backup.jsonl
```

## Using Strategies

Define multiple approaches for recurring tasks and let trajectory-memory learn which works best.

### Define Strategies in CLAUDE.md

```markdown
<!-- trajectory-strategies:daily-briefing -->
strategies:
  - name: comprehensive
    description: Summarize everything
    approach_prompt: |
      Summarize all articles from all feeds.
      Group by source. Include every article.

  - name: curated
    description: Pick the best
    approach_prompt: |
      Select the 5-7 most important articles.
      Provide deeper summaries (4-5 sentences each).

  - name: thematic
    description: Find connections
    approach_prompt: |
      Identify 3-4 themes across all feeds.
      Group articles by theme and synthesize.
<!-- /trajectory-strategies:daily-briefing -->
```

### Use Strategies

```bash
# Start with a specific strategy
/trajectory-start --strategy curated daily-briefing

# Let trajectory-memory recommend based on past scores
/trajectory-start --recommend daily-briefing

# Rotate to gather comparative data
/trajectory-start --rotate daily-briefing
```

### Analyze Performance

```bash
/trajectory-strategies-analyze daily-briefing
```

Shows average scores per strategy and recommends which to use next.

## Next Steps

- Explore the [examples/](examples/) directory for sample configurations
- Set up auto-optimization triggers: `trajectory-memory trigger configure --enabled=true`
- Add files to watch: `trajectory-memory trigger watch CLAUDE.md`
- Define strategies for recurring tasks in your CLAUDE.md

## Getting Help

- GitHub Issues: https://github.com/johncarpenter/trajectory-memory/issues
- Full documentation: [README.md](README.md)
