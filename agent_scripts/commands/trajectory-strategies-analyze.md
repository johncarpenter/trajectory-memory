# Trajectory Strategies Analyze

Analyze strategy performance based on trajectory scores.

## Context

This command analyzes how different strategies are performing for a given task type. It calculates average scores across sessions, identifies the best performer, and recommends which strategy to use next.

## Instructions

### 1. Get the Tag

The user should provide a strategy tag (e.g., "daily-briefing", "code-review").

### 2. Analyze Strategies

Call the trajectory-memory MCP server to analyze strategy performance:

```
Use mcp__trajectory-memory__trajectory_strategies_analyze tool with:
- tag: The strategy tag to analyze
```

### 3. Present Results

Display the analysis results:
- **Total Sessions**: How many sessions have used this strategy tag
- **Strategy Performance**: For each strategy, show:
  - Name and description
  - Average score
  - Number of sessions
- **Best Strategy**: Which strategy has the highest average score
- **Recommended Next**: What the system recommends for the next session
- **Rotation Suggested**: Whether more exploration is needed

### 4. Provide Insights

Based on the analysis, offer insights:
- If one strategy clearly outperforms others, recommend using it
- If there's insufficient data, suggest rotating through strategies
- If scores are similar, suggest continuing to gather data

## Example Usage

User: `/trajectory-strategies-analyze daily-briefing`
-> Analyze all sessions using daily-briefing strategies, show performance comparison

User: `/trajectory-strategies-analyze`
-> Ask user which strategy tag they want to analyze

## Example Output

```
Strategy Analysis for: daily-briefing
=====================================
Total Sessions: 12

Strategy Performance:
┌─────────────┬─────────────────────────┬───────────┬──────────┐
│ Strategy    │ Description             │ Avg Score │ Sessions │
├─────────────┼─────────────────────────┼───────────┼──────────┤
│ curated     │ Pick the best          │ 0.85      │ 5        │
│ thematic    │ Find connections       │ 0.78      │ 4        │
│ comprehensive│ Summarize everything   │ 0.72      │ 3        │
└─────────────┴─────────────────────────┴───────────┴──────────┘

Best Strategy: curated (0.85 avg)
Recommendation: Use "curated" strategy - it consistently outperforms others.
```

## Notes

- Strategies need at least 3 sessions with scores to calculate meaningful averages
- The system uses an explore/exploit balance to recommend when to try underused strategies
- Performance analysis improves over time as more data is collected
