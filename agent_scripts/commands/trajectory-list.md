# Trajectory List

List past recorded trajectory sessions.

## Context

This command lists previously recorded trajectory sessions from the trajectory-memory database. Useful for reviewing past work, finding sessions to reference, or checking what's been captured.

## Instructions

### 1. List Trajectories

Call the trajectory-memory MCP server to list past sessions:

```
Use mcp__trajectory-memory__trajectory_list tool
```

### 2. Present Results

Format the results for the user showing:
- Session date/time
- Summary (if available)
- Score (if scored)
- Tags (if tagged)

### 3. Optional: Search

If the user provides a search term, use search instead:

```
Use mcp__trajectory-memory__trajectory_search tool with the search query
```

## Example Usage

User: `/trajectory-list`
-> List recent trajectories

User: `/trajectory-list rss`
-> Search for trajectories related to "rss"

User: `/trajectory-list --limit 5`
-> List only the 5 most recent trajectories
