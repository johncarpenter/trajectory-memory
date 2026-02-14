# Example E: Code Review Analysis

This example demonstrates trajectory-memory for pattern identification across code reviews and pull requests.

## The Task

Analyze recent PRs to identify patterns, issues, and areas for improvement.

## Example Prompts

**Primary task:**
```
Review the recent PRs in this repo and identify common patterns, issues, and areas for improvement
```

**Variations:**
- "Analyze recent PRs for security issues and code quality patterns"
- "Review our PR process based on recent merges - what's working and what needs improvement?"
- "What security vulnerabilities might have been missed in recent reviews?"

## What Makes a Good Trajectory?

**High-scoring approaches (0.8-1.0):**
- Reads actual PR content, not just metadata
- Uses grep/search to verify suspicions
- Identifies patterns across multiple PRs
- Categorizes findings (critical, concerning, positive)
- Provides specific, actionable recommendations
- Notes both issues AND good practices

**Medium-scoring approaches (0.5-0.7):**
- Describes what was done in each PR
- Some pattern identification
- Generic recommendations
- Doesn't verify issues in code

**Low-scoring approaches (0.0-0.4):**
- Just lists PR titles
- No actual analysis
- Doesn't read PR content
- No patterns or recommendations

## Sample PR Data

The `prs/` directory contains 6 sample PR summaries:

| PR | Content | Issues Present |
|----|---------|----------------|
| #142 | User authentication | Good - caught issues in review |
| #145 | Payment race condition fix | Good - substantive discussion |
| #148 | Dashboard UI update | Large PR, deferred tests |
| #150 | Search functionality | **SQL injection vulnerability** |
| #152 | Structured logging | Good - security considerations |
| #155 | CSV export | No tests, minimal review |

## Running the Example

1. **Seed the database:**
   ```bash
   trajectory-memory import examples/code-review-analysis/seed.jsonl
   ```

2. **View seeded sessions:**
   ```bash
   trajectory-memory search "code review"
   trajectory-memory show 01JCODERVW001
   ```

3. **Run in Claude Code:**
   - Start Claude Code in this directory
   - Try: "Analyze the recent PRs for security issues"
   - Watch for: Does it actually read the PRs? Does it grep to verify?

4. **Compare outputs:**
   - `output-examples/high-score-output.md` - Found SQL injection, categorized findings
   - `output-examples/low-score-output.md` - Just listed PRs

## Key Learnings from Trajectories

What the agent learns from high-scoring sessions:

1. **Read the actual code** - not just PR descriptions
2. **Use grep to verify** - "I suspect SQL injection" → grep for `fmt.Sprintf.*SELECT`
3. **Look for patterns** - Which PRs had good review? Which didn't?
4. **Categorize severity** - Critical vs concerning vs positive
5. **Check process too** - How many reviewers? Was feedback substantive?
6. **Compare to standards** - Do PRs follow team guidelines?

## Metrics

Track improvement with:
- Did analysis find the planted SQL injection?
- Were patterns identified (not just individual issues)?
- Are recommendations specific and actionable?
- Did analysis include process observations?
