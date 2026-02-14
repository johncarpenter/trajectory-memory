# PR #152: Add structured logging to API

**Author:** henry
**Branch:** feature/structured-logging
**Merged:** 2024-01-18
**Reviewers:** alice, charlie
**Files changed:** 12
**Additions:** 189, Deletions:** 67

## Description
Replaces printf-style logging with structured JSON logging. Adds request IDs, timing, and better error context.

## Review Comments

**alice:** Great improvement! The request ID propagation through context is well done.

**charlie:** Should we redact sensitive fields like passwords or tokens from logs?
> **henry:** Good point! Added a sanitizer that redacts fields matching patterns like `password`, `token`, `secret`, `key`.

**alice:** The log level is always INFO. Should errors use ERROR level?
> **henry:** Added level detection based on status code: 5xx = ERROR, 4xx = WARN, else INFO

**charlie:** Consider adding sampling for high-volume endpoints to avoid log explosion
> **henry:** Added configurable sampling rate, defaulting to 100%. Can reduce for specific endpoints.

## Files Changed

### middleware/logging.go (+98, -34)
```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        requestID := uuid.New().String()

        ctx := context.WithValue(r.Context(), "requestID", requestID)
        w.Header().Set("X-Request-ID", requestID)

        // Wrap response writer to capture status
        wrapped := &responseWrapper{ResponseWriter: w, status: 200}

        next.ServeHTTP(wrapped, r.WithContext(ctx))

        duration := time.Since(start)
        level := levelFromStatus(wrapped.status)

        log.WithFields(log.Fields{
            "request_id": requestID,
            "method":     r.Method,
            "path":       r.URL.Path,
            "status":     wrapped.status,
            "duration_ms": duration.Milliseconds(),
            "user_agent": r.UserAgent(),
        }).Log(level, "request completed")
    })
}
```

### middleware/sanitizer.go (+45, -0)
Redacts sensitive fields from log output.

### config/logging.go (+28, -15)
Logging configuration with sampling rates.

## Tests
- 15 test cases for logging middleware
- Tests for sanitizer patterns

## CI Status
All checks passed.
