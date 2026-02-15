# Trajectory Stop

End recording the current session and generate a trajectory summary.

## Context

This command stops the active trajectory recording, generates a summary of what was accomplished, and stores it for future reference. The summary becomes searchable via `trajectory_search` for learning from past sessions.

## Instructions

### 1. Stop Recording

Call the trajectory-memory MCP server to stop recording:

```
Use mcp__trajectory-memory__trajectory_stop tool
```

### 2. Generate Session Summary

Create a summary of the session including:
- **Task**: What was the user trying to accomplish?
- **Approach**: What strategy/tools were used?
- **Outcome**: Was it successful? Any blockers?
- **Key Decisions**: Important choices made
- **Lessons Learned**: What worked well or could be improved?

### 3. Score the Session

Call the trajectory-memory MCP server to score the session:

```
Use mcp__trajectory-memory__trajectory_score tool with:
- score: Rate the session quality (0.0 to 1.0)
  - 1.0: Perfect execution, reusable approach
  - 0.7-0.9: Successful with minor issues
  - 0.4-0.6: Completed but had problems
  - 0.1-0.3: Failed or significant issues
```

### 4. Save the Summary

Call the trajectory-memory MCP server to save the summary:

```
Use mcp__trajectory-memory__trajectory_summarize tool with:
- summary: The generated summary text
- tags: Relevant keywords for search (e.g., "rss", "mcp", "cli", "debugging")
```

### 5. Confirm to User

After saving, inform the user:
- Recording has stopped
- Summary of what was captured
- The trajectory is now searchable for future sessions

## Example Usage

User: `/trajectory-stop`
-> Stop recording, generate summary based on session history, save with score and tags

User: `/trajectory-stop score:0.8 tags:rss,mcp,cli`
-> Stop recording, use provided score and tags in the summary

## Notes

- Always generate a thoughtful summary - this is training data for future sessions
- Be honest with scoring - accurate scores improve future recommendations
- Include enough detail in tags for searchability
- If the session was problematic, document what went wrong to avoid repeating mistakes
