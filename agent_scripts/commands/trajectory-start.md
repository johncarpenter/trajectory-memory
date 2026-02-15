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

### 1. Start Recording

Call the trajectory-memory MCP server to begin recording:

```
Use mcp__trajectory-memory__trajectory_start tool
```

### 2. Confirm to User

After starting, inform the user:
- Recording has begun
- All tool calls and conversation turns will be captured
- Remind them to use `/trajectory-stop` when done to save the trajectory

### 3. Optional: Search Related Trajectories

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

## Notes

- Recording is automatically associated with the current session
- The hook in `.claude/settings.json` captures step data automatically once recording is active
- Use `/trajectory-stop` to end recording and generate a summary
