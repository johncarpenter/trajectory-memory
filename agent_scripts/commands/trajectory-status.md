# Trajectory Status

Check if trajectory recording is currently active.

## Context

This command checks the current state of trajectory recording - whether a session is being recorded, and if so, basic info about the active recording.

## Instructions

### 1. Check Status

Call the trajectory-memory MCP server to get recording status:

```
Use mcp__trajectory-memory__trajectory_status tool
```

### 2. Report to User

Tell the user:
- Whether recording is active or not
- If active, how long it's been recording
- Remind them of `/trajectory-stop` to end, or `/trajectory-start` to begin

## Example Usage

User: `/trajectory-status`
-> Check status, report "Recording active (started 15 minutes ago)" or "No active recording"
