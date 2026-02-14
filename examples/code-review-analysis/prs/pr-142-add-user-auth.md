# PR #142: Add user authentication

**Author:** alice
**Branch:** feature/user-auth
**Merged:** 2024-01-10
**Reviewers:** bob, charlie
**Files changed:** 8
**Additions:** 342, Deletions:** 45

## Description
Implements JWT-based authentication for the API. Adds login/logout endpoints, middleware for protected routes, and user session management.

## Review Comments

**bob:** The token expiration is hardcoded to 24 hours. Should this be configurable?
> **alice:** Good catch, moved to env var `JWT_EXPIRY_HOURS`

**charlie:** Missing rate limiting on the login endpoint - potential brute force vulnerability
> **alice:** Added rate limiting middleware, 5 attempts per minute per IP

**bob:** The password hashing uses bcrypt which is good, but the cost factor is only 10. Consider increasing to 12 for production.
> **alice:** Updated to 12, added as configurable `BCRYPT_COST`

## Files Changed

### auth/handler.go (+156, -0)
```go
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
    var req LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    user, err := h.userRepo.FindByEmail(req.Email)
    if err != nil {
        http.Error(w, "Invalid credentials", http.StatusUnauthorized)
        return
    }

    if !h.checkPassword(user.PasswordHash, req.Password) {
        http.Error(w, "Invalid credentials", http.StatusUnauthorized)
        return
    }

    token, err := h.generateToken(user.ID)
    // ...
}
```

### auth/middleware.go (+89, -0)
```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := extractToken(r)
        if token == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        claims, err := validateToken(token)
        if err != nil {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }

        ctx := context.WithValue(r.Context(), "userID", claims.UserID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### auth/token.go (+67, -0)
Token generation and validation utilities.

### config/auth.go (+30, -0)
Configuration for auth settings (JWT secret, expiry, bcrypt cost).

## Tests Added
- `auth/handler_test.go` - 12 test cases
- `auth/middleware_test.go` - 8 test cases

## CI Status
All checks passed.
