# Trajectory Memory Integration

This project uses trajectory-memory to record, score, and learn from agent execution traces.

## MCP Server Configuration

Add this to your Claude Code settings (`~/.claude/settings.json`):

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

## Available Tools

The trajectory-memory MCP server provides these tools:

- `trajectory_start` - Begin recording a session
- `trajectory_stop` - Stop recording and get trajectory for summarization
- `trajectory_status` - Check if recording is active
- `trajectory_search` - Find past sessions by keyword
- `trajectory_list` - List recent sessions
- `trajectory_score` - Score a completed session
- `trajectory_summarize` - Store a model-generated summary

## Recording Sessions

**Start recording when:**
- User says "start logging", "record this session", or "track this"
- Beginning a substantive multi-step task (if auto-record is enabled)

**To start:** Call `trajectory_start` with a task_prompt describing what you're doing.

```
trajectory_start({ "task_prompt": "Implement user authentication", "tags": ["feature", "auth"] })
```

**Stop recording when:**
- User says "stop logging", "end recording", or "done"
- A substantive task is clearly complete

**To stop:** Call `trajectory_stop`. This returns the execution trace.

```
trajectory_stop({ "score": 0.85, "notes": "Completed successfully" })
```

## Summarization Flow

After `trajectory_stop` returns, you will receive the full trajectory formatted for summarization. You should:

1. Read the trajectory carefully
2. Generate a 2-3 sentence summary describing:
   - What task was accomplished
   - What approach was taken
   - Any notable patterns or outcomes
3. Call `trajectory_summarize` with the session_id and your summary

Example:
```
trajectory_summarize({
  "session_id": "01HTEST...",
  "summary": "Implemented JWT-based authentication with refresh tokens. Used a middleware pattern for route protection. All tests passing."
})
```

## Learning from Past Sessions

When starting a complex task, search for relevant past trajectories:

```
trajectory_search({ "query": "authentication", "limit": 3, "min_score": 0.7 })
```

Use high-scoring past sessions to inform your approach:
- Review what tools and patterns worked well
- Avoid approaches that led to low scores
- Build on successful patterns

## Status Checks

When asked "are you recording?" or similar:

```
trajectory_status()
```

Returns: `{ "active": true/false, "session_id": "...", "step_count": N, "duration_seconds": N }`

## Scoring Guidelines

Scores are 0.0 to 1.0:
- **0.0-0.3**: Task failed or had major issues
- **0.4-0.6**: Task completed but with problems
- **0.7-0.8**: Task completed well
- **0.9-1.0**: Excellent execution, exemplary approach

When scoring, consider:
- Did the task complete successfully?
- Was the approach efficient?
- Were best practices followed?
- Could this trajectory teach good patterns?

## Best Practices

1. **Start recording early** - Capture the full context of how you approach tasks
2. **Use descriptive task prompts** - Make sessions searchable later
3. **Tag consistently** - Use tags like "bugfix", "feature", "refactor", "docs"
4. **Score honestly** - Accurate scores improve future recommendations
5. **Write good summaries** - Help future sessions learn from past work
6. **Search before starting** - Learn from similar past tasks

---

## Context Optimization

Trajectory-memory can automatically improve instructions based on what works. After accumulating scored sessions, it analyzes patterns in high vs low-scoring trajectories and suggests optimized instructions.

### Optimization Tools

- `trajectory_optimize_propose` - Analyze trajectories and propose optimized content
- `trajectory_optimize_save` - Save generated optimized content as proposal
- `trajectory_optimize_apply` - Apply a proposed optimization
- `trajectory_optimize_rollback` - Revert an applied optimization
- `trajectory_optimize_history` - View optimization history
- `trajectory_curate_examples` - Curate best examples for few-shot learning
- `trajectory_curate_apply` - Apply curated examples to a file
- `trajectory_trigger_status` - Check trigger configuration
- `trajectory_trigger_configure` - Configure auto-optimization triggers

### After Scoring

When you score a session with `trajectory_score`, check the response for optimization notifications. If the system indicates optimizations are pending, inform the user and offer to run `trajectory_optimize_propose`.

### Optimization Flow

1. **When asked to "improve" or "optimize" instructions:**
   - Call `trajectory_optimize_propose` on the relevant file
   - Review the analysis and patterns identified
   - Generate optimized content based on the meta-prompt
   - Call `trajectory_optimize_save` with the new content
   - Present the diff to the user
   - Apply or reject based on user feedback

2. **Be transparent:** When presenting proposals, explain the data:
   > "Based on 18 research sessions, I'm suggesting these changes because sessions that read 3+ source files scored 84% average vs 38% for sessions that jumped straight to writing."

3. **Periodic curation:** Every ~10 sessions, suggest running `trajectory_curate_examples` to refresh few-shot examples.

### Optimization Markers

Add markers to CLAUDE.md sections that should be data-driven:

```markdown
## Research Best Practices

<!-- trajectory-optimize:start tag="research" min_sessions=10 -->
1. Define your research scope with specific questions before starting
2. Read all available context files and source material (aim for 3+ sources)
3. Use 5-8 targeted searches covering different angles
4. Structure output with: executive summary, detailed findings, recommendations
<!-- trajectory-optimize:end -->
```

The optimizer will only modify content between markers, leaving everything else intact.

### Example Markers for Curated Examples

```markdown
<!-- trajectory-examples:start tag="research" max=3 include_negative=true -->
### What Works Well (from past sessions)

(Curated examples will appear here after sufficient scored sessions)
<!-- trajectory-examples:end -->
```

### Example CLAUDE.md Template

```markdown
# Project Workspace

## Trajectory Memory Integration

Before starting tasks, search for past approaches:
\`\`\`
trajectory_search({ "query": "<task type>", "limit": 3, "min_score": 0.8 })
\`\`\`

Start recording for substantive tasks:
\`\`\`
trajectory_start({ "task_prompt": "<description>", "tags": ["<type>"] })
\`\`\`

## Task Instructions

<!-- trajectory-optimize:start tag="research" min_sessions=10 -->
### Research Best Practices

1. Define your research scope with specific questions before starting
2. Read all available context files and source material (aim for 3+ sources)
3. Use 5-8 targeted searches covering different angles of the topic
4. Structure output with: executive summary, detailed findings, recommendations
5. Cite sources and note information gaps
6. Review your output and revise at least once before finalizing
<!-- trajectory-optimize:end -->

<!-- trajectory-examples:start tag="research" max=3 include_negative=true -->
### What Works Well (from past sessions)

(Curated examples will appear here after sufficient scored sessions)
<!-- trajectory-examples:end -->

## Output Format
...
```

### CLI Commands

```bash
# Analyze and propose optimization
trajectory-memory optimize propose CLAUDE.md --tag research

# View optimization history
trajectory-memory optimize history --tag research

# Curate best examples for a tag
trajectory-memory curate research --max 3

# Configure auto-triggers
trajectory-memory trigger configure --enabled=true --threshold=10
trajectory-memory trigger watch CLAUDE.md
```
