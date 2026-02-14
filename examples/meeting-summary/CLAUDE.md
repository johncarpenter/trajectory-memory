# Meeting Summary Workspace

This workspace is for creating executive summaries and action items from meeting transcripts.

## Trajectory Memory Integration

Before summarizing a meeting, search for past approaches:
```
trajectory_search({ "query": "meeting summary action items", "limit": 3, "min_score": 0.8 })
```

Start recording for substantial summarization work:
```
trajectory_start({ "task_prompt": "Summarize <meeting name>", "tags": ["meeting-summary"] })
```

## Meeting Summary Best Practices

From high-scoring trajectories:

### Reading Approach
1. **First pass:** Understand context, attendees, overall arc
2. **Second pass:** Extract specifics - decisions, action items, concerns
3. **Check for:** Owners, deadlines, dependencies, risks

### Output Structure (Executive Summary)
1. **Header:** Date, attendees, duration
2. **Executive Summary:** 2-3 sentences capturing the essence
3. **Key Decisions:** What was decided, with rationale
4. **Action Items:** Table with owner, deadline, status
5. **Risks/Concerns:** Flagged issues requiring attention
6. **Next Steps:** Immediate priorities and next meeting

### Action Item Format
Every action item should have:
- Clear, specific description
- Single owner (not "the team")
- Due date (specific day, not "soon")
- Status indicator

### What to Capture
- Decisions made (and why)
- Commitments made to stakeholders
- Risks identified
- Disagreements or tradeoffs discussed
- Dependencies on other teams/projects
- Timeline changes

### What to Skip
- Play-by-play of discussion
- Off-topic tangents
- Redundant points
- Obvious context

## Meeting Types

### Product/Planning Meetings
Focus on: Roadmap changes, resource decisions, timeline adjustments

### Customer Feedback Sessions
Focus on: Themes across customers, specific requests with attribution, commitments made

### Sprint Retrospectives
Focus on: Patterns in feedback, voted priorities, improvement actions with owners

### Incident Reviews
Focus on: Timeline, root cause, remediation actions, preventive measures

## Quality Checklist

Before finishing:
- [ ] Could an executive understand this in 2 minutes?
- [ ] Are all action items specific and assigned?
- [ ] Are deadlines explicit?
- [ ] Are risks clearly flagged?
- [ ] Is the "so what" clear?
