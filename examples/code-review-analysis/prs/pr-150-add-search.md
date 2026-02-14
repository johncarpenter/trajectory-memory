# PR #150: Add search functionality

**Author:** grace
**Branch:** feature/search
**Merged:** 2024-01-17
**Reviewers:** dave
**Files changed:** 6
**Additions:** 234, Deletions:** 18

## Description
Adds full-text search across products, orders, and customers. Uses PostgreSQL full-text search.

## Review Comments

**dave:** LGTM, nice use of PostgreSQL FTS!

## Files Changed

### search/handler.go (+78, -0)
```go
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query().Get("q")

    results, err := h.searchService.Search(query)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(results)
}
```

### search/service.go (+112, -0)
```go
func (s *SearchService) Search(query string) ([]SearchResult, error) {
    sql := fmt.Sprintf(`
        SELECT id, type, title, snippet
        FROM search_index
        WHERE search_vector @@ to_tsquery('%s')
        LIMIT 50
    `, query)

    rows, err := s.db.Query(sql)
    // ...
}
```

### db/migrations/005_add_search_index.sql (+44, -0)
Creates search index and triggers.

## Tests
- Basic search test added

## CI Status
All checks passed.
