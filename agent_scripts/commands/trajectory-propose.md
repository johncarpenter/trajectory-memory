# Trajectory Propose

Propose CLAUDE.md improvements based on trajectory data and lessons learned.

## Context

This command analyzes recorded trajectory sessions to identify patterns, lessons learned, and common workflows. It then proposes concrete changes to CLAUDE.md that will improve future sessions by codifying successful approaches and avoiding past mistakes.

## Instructions

### 1. Gather Trajectory Data

First, list recent trajectories to understand what's been recorded:

```
Use mcp__trajectory-memory__trajectory_list tool with limit: 20
```

### 2. Search for Patterns

If the user specifies a topic, search for relevant trajectories:

```
Use mcp__trajectory-memory__trajectory_search tool with the topic query
```

Look for:
- High-scoring sessions (score >= 0.7) - these have reusable approaches
- Low-scoring sessions (score < 0.5) - these have lessons about what to avoid
- Common tags across sessions - these indicate frequent workflows
- Repeated summaries mentioning similar patterns

### 3. Read Current CLAUDE.md

```
Use Read tool to read the project's CLAUDE.md file
```

Identify:
- Existing optimization markers (`<!-- trajectory-optimize:tag -->`)
- Existing example markers (`<!-- trajectory-examples:tag -->`)
- Sections that could benefit from trajectory insights
- Missing documentation for common workflows

### 4. Analyze and Propose Changes

Based on trajectory data, propose changes in these categories:

**A. New Workflow Sections**
For workflows that appear in multiple trajectories but aren't documented:
```markdown
<!-- trajectory-optimize:workflow-name -->
## Workflow Name

Step-by-step instructions based on high-scoring trajectory approaches.

**Lessons Learned:**
- Pattern from trajectory summaries
- Common pitfalls to avoid
<!-- /trajectory-optimize:workflow-name -->
```

**B. Best Practices Updates**
Extract lessons learned from trajectory summaries and add to Best Practices:
```markdown
## Best Practices
- Existing practices...
- **New practice from trajectories** (learned from X sessions)
```

**C. Known Issues**
If trajectories reveal recurring problems (e.g., failing feeds, broken endpoints):
```markdown
## Known Issues
- Issue description (observed in N sessions)
```

**D. Example Markers**
For workflows with enough trajectory data, add example injection points:
```markdown
<!-- trajectory-examples:workflow-name -->
<!-- High-scoring examples will be curated here -->
<!-- /trajectory-examples:workflow-name -->
```

### 5. Present Proposal

Format the proposal clearly for user review:

```markdown
## Proposed CLAUDE.md Changes

Based on analysis of N trajectories (date range: X to Y)

### Change 1: [Category]
**Rationale**: Why this change is valuable based on trajectory data
**Source**: Which trajectories informed this (session IDs, scores)

[Proposed markdown content]

---

### Change 2: [Category]
...
```

### 6. Offer to Apply

Ask the user if they want to:
1. Apply all changes
2. Apply specific changes
3. Modify the proposal
4. Save the proposal for later review

If user approves, use Edit tool to update CLAUDE.md.

### 7. Record the Optimization

If changes are applied, use the trajectory optimization tools to track:

```
Use mcp__trajectory-memory__trajectory_optimize_save tool with:
- record_id: Generated ID for this optimization
- file_path: Path to CLAUDE.md
- tag: The optimization target tag
- previous_content: Original content
- content: New optimized content
```

## Example Usage

User: `/trajectory-propose`
-> Analyze all trajectories, propose general improvements

User: `/trajectory-propose daily-digest`
-> Focus on daily-digest related trajectories

User: `/trajectory-propose --min-score 0.8`
-> Only use high-scoring trajectories for proposals

User: `/trajectory-propose --apply`
-> Analyze and apply changes without confirmation (use cautiously)

## Proposal Categories

| Category | Trigger | Example |
|----------|---------|---------|
| New Workflow | 3+ trajectories with same tag | Document the daily-digest workflow |
| Best Practice | Lesson appears in 2+ summaries | "Check existing files before regenerating" |
| Known Issue | Same error in 2+ sessions | "Feed X returns HTTP 404" |
| Anti-pattern | Low-scoring sessions share pattern | "Don't attempt write without read" |
| Optimization Target | Workflow has variance in scores | Add marker for future optimization |

## Notes

- Proposals are based on actual execution data, not speculation
- High-scoring sessions (>= 0.8) are treated as gold standard approaches
- Low-scoring sessions inform what to avoid
- Changes should be minimal and focused - don't over-document
- Preserve existing CLAUDE.md structure and style
- If insufficient trajectory data exists, say so and suggest recording more sessions

## Minimum Data Requirements

For confident proposals:
- **New Workflow**: At least 3 related trajectories
- **Best Practice**: Pattern in at least 2 session summaries
- **Known Issue**: Same issue in at least 2 sessions
- **Anti-pattern**: At least 1 low-scoring session with clear cause

If data is sparse, propose adding optimization markers for future learning rather than making content changes.
