# Code Review Analysis Workspace

This workspace is for analyzing pull request patterns, identifying issues, and improving code quality processes.

## Trajectory Memory Integration

Before analyzing PRs, search for past approaches:
```
trajectory_search({ "query": "code review analysis patterns", "limit": 3, "min_score": 0.8 })
```

Start recording for substantive analysis:
```
trajectory_start({ "task_prompt": "Analyze recent PRs for <focus>", "tags": ["code-review", "analysis"] })
```

## Analysis Best Practices

From high-scoring trajectories:

### Preparation
1. Read team guidelines (PR standards, security policies)
2. Get PR list with metadata (reviewers, size, status)
3. Identify which PRs to analyze in depth

### Analysis Approach
1. **Read each PR** - not just titles, actual changes and comments
2. **Look for patterns** - across PRs, not just individual issues
3. **Verify suspicions** - use grep/search to confirm potential issues
4. **Check both code AND process** - how was it reviewed?

### What to Look For

**Security Issues:**
- SQL injection (string interpolation in queries)
- Missing authentication/authorization
- Sensitive data in logs
- Hardcoded secrets
- Missing input validation

**Code Quality:**
- Missing tests
- Error handling gaps
- Large PRs that are hard to review
- Inconsistent patterns
- TODO/FIXME without tickets

**Process Issues:**
- PRs with only 1 reviewer
- "LGTM" without substantive feedback
- Tests deferred to "follow-up"
- Guidelines not followed

### Output Structure
1. **Executive Summary** - Key findings in 2-3 sentences
2. **Critical Issues** - Things needing immediate action
3. **Positive Patterns** - What's working well
4. **Concerning Patterns** - Systemic issues
5. **Recommendations** - Specific, actionable improvements
6. **Metrics** - Quantify the findings

### Verification Techniques
```bash
# Find potential SQL injection
grep -r "fmt.Sprintf.*SELECT" --include="*.go"

# Find missing error handling
grep -r "_, err :=" --include="*.go" | grep -v "if err"

# Check test coverage
find . -name "*_test.go" -type f | wc -l
```

## Security Checklist

When reviewing for security:
- [ ] Input validation on all user inputs
- [ ] Parameterized queries (no string interpolation)
- [ ] Authentication on sensitive endpoints
- [ ] Authorization checks (user can access resource?)
- [ ] Sensitive data not logged
- [ ] Secrets not hardcoded
- [ ] Rate limiting on auth endpoints

## Quality Checklist

Before finishing analysis:
- [ ] Read actual PR content, not just metadata
- [ ] Identified patterns across multiple PRs
- [ ] Verified issues with grep/search where possible
- [ ] Noted both problems AND good practices
- [ ] Recommendations are specific and actionable
- [ ] Critical issues clearly prioritized
