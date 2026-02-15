# Trajectory Start

Begin recording the current Claude Code session for trajectory memory.

## Context

This command starts recording the current session's execution trace using the trajectory-memory MCP server. The recorded trajectory can later be summarized and used for learning from past sessions.

The trajectory-memory system captures:
- Tool calls and results
- Conversation turns
- Decisions and approaches taken
- Errors and recoveries

## Instructions

### 1. Parse Arguments

Check if the user specified a strategy tag or strategy name:
- `--strategy <name> <tag>` - Use a specific strategy for a tag
- `--recommend <tag>` - Get AI recommendation for best strategy
- `--rotate <tag>` - Cycle through strategies to gather comparative data

### 2. If Strategy Specified, Select Strategy

If a strategy tag was provided, first select the strategy:

**For explicit strategy:**
```
Use mcp__trajectory-memory__trajectory_strategies_select tool with:
- tag: The strategy tag (e.g., "daily-briefing")
- mode: "explicit"
- strategy_name: The strategy name (e.g., "curated")
```

**For recommended strategy:**
```
Use mcp__trajectory-memory__trajectory_strategies_select tool with:
- tag: The strategy tag
- mode: "recommend"
```

**For rotation:**
```
Use mcp__trajectory-memory__trajectory_strategies_select tool with:
- tag: The strategy tag
- mode: "rotate"
```

### 3. Start Recording

Call the trajectory-memory MCP server to begin recording:

```
Use mcp__trajectory-memory__trajectory_start tool with:
- task_prompt: Description of the task (from user args or strategy description)
- tags: Include the strategy tag if one was selected
```

### 4. Confirm to User

After starting, inform the user:
- Recording has begun
- If a strategy was selected, show the strategy name and approach prompt
- All tool calls and conversation turns will be captured
- Remind them to use `/trajectory-stop` when done to save the trajectory

### 5. Optional: Search Related Trajectories

If the user mentioned what task they're working on, optionally search for relevant past trajectories:

```
Use mcp__trajectory-memory__trajectory_search tool with the task description
```

Present any relevant past approaches that might help with the current task.

## Example Usage

User: `/trajectory-start`
-> Start recording, confirm to user

User: `/trajectory-start working on RSS feed parser`
-> Start recording, search for related past trajectories about RSS/feed parsing, present relevant insights

User: `/trajectory-start --strategy curated daily-briefing`
-> Select "curated" strategy for "daily-briefing", start recording with strategy context

User: `/trajectory-start --recommend daily-briefing`
-> Get best performing strategy recommendation, start recording with that strategy

User: `/trajectory-start --rotate daily-briefing`
-> Cycle to next underused strategy, start recording with that strategy

## Notes

- Recording is automatically associated with the current session
- The hook in `.claude/settings.json` captures step data automatically once recording is active
- When using strategies, the selected approach_prompt is included in the task context
- Use `/trajectory-stop` to end recording and generate a summary
- Strategy usage is automatically tracked for performance analysis
