# Market Research Project

This is a research workspace for market analysis and competitive intelligence.

## Trajectory Memory Integration

This project uses trajectory-memory to learn from past research approaches.

### Before Starting Research

Always search for relevant past trajectories:
```
trajectory_search({ "query": "<topic> research", "limit": 3, "min_score": 0.7 })
```

Review high-scoring approaches and adopt their methodology.

### During Research

When starting substantive research, begin recording:
```
trajectory_start({
  "task_prompt": "Research <specific topic>",
  "tags": ["research", "market-analysis"]
})
```

### Research Best Practices (from high-scoring trajectories)

1. **Define scope first** - What specific questions need answering?
2. **Use authoritative sources** - Official docs, surveys, analyst reports
3. **Multiple perspectives** - Search for different viewpoints
4. **Structure output** - Executive summary, details, recommendations
5. **Cite sources** - Track where information came from
6. **Identify gaps** - What couldn't be determined?

### After Research

Stop recording and generate summary:
```
trajectory_stop({ "auto_summarize": true })
```

Then score the session based on:
- Comprehensiveness of research
- Quality of sources used
- Clarity of output structure
- Actionability of insights

## Output Format

Research deliverables should include:
- Executive summary (2-3 sentences)
- Key findings with supporting data
- Comparison tables where applicable
- Market trends or implications
- Recommendations
- Source attribution
