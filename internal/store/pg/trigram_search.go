package pg

import (
	"context"
	"fmt"
	"time"
	"unicode"

	"github.com/hermes-agent/hermes-agent-go/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SessionSearchResult is a lightweight result from session title search.
type SessionSearchResult struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Platform  string    `json:"platform"`
	UserID    string    `json:"user_id"`
	StartedAt time.Time `json:"started_at"`
}

// TrigramSearcher provides CJK-aware full-text search using pg_trgm.
type TrigramSearcher struct {
	pool *pgxpool.Pool
}

// NewTrigramSearcher creates a TrigramSearcher backed by the given pool.
func NewTrigramSearcher(pool *pgxpool.Pool) *TrigramSearcher {
	return &TrigramSearcher{pool: pool}
}

// SearchMessages performs hybrid search: tsvector for Latin text, trigram for CJK.
func (ts *TrigramSearcher) SearchMessages(ctx context.Context, tenantID, query string, limit int) ([]*store.SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if query == "" {
		return nil, nil
	}

	var sql string
	if containsCJK(query) {
		sql = `
			SELECT session_id, id, content,
			       left(content, 200) AS snippet,
			       similarity(content, $2) AS rank
			FROM messages
			WHERE tenant_id = $1 AND content % $2
			ORDER BY rank DESC LIMIT $3`
	} else {
		sql = `
			SELECT session_id, id, content,
			       ts_headline('english', content, plainto_tsquery('english', $2)) AS snippet,
			       ts_rank(to_tsvector('english', content), plainto_tsquery('english', $2)) AS rank
			FROM messages
			WHERE tenant_id = $1 AND to_tsvector('english', content) @@ plainto_tsquery('english', $2)
			ORDER BY rank DESC LIMIT $3`
	}

	rows, err := ts.pool.Query(ctx, sql, tenantID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("trigram search messages: %w", err)
	}
	defer rows.Close()

	var results []*store.SearchResult
	for rows.Next() {
		r := &store.SearchResult{}
		if err := rows.Scan(&r.SessionID, &r.MessageID, &r.Content, &r.Snippet, &r.Rank); err != nil {
			return nil, fmt.Errorf("trigram search scan: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// SearchMemories searches memory content using trigram similarity.
func (ts *TrigramSearcher) SearchMemories(ctx context.Context, tenantID, userID, query string, limit int) ([]store.MemoryEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	if query == "" {
		return nil, nil
	}

	var sql string
	if containsCJK(query) {
		sql = `
			SELECT tenant_id, user_id, key, content, updated_at
			FROM memories
			WHERE tenant_id = $1 AND user_id = $2 AND content % $3
			ORDER BY similarity(content, $3) DESC LIMIT $4`
	} else {
		sql = `
			SELECT tenant_id, user_id, key, content, updated_at
			FROM memories
			WHERE tenant_id = $1 AND user_id = $2 AND content ILIKE '%' || $3 || '%'
			ORDER BY updated_at DESC LIMIT $4`
	}

	rows, err := ts.pool.Query(ctx, sql, tenantID, userID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("trigram search memories: %w", err)
	}
	defer rows.Close()

	var entries []store.MemoryEntry
	for rows.Next() {
		var e store.MemoryEntry
		if err := rows.Scan(&e.TenantID, &e.UserID, &e.Key, &e.Content, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("trigram search memories scan: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// SearchSessions searches session titles using trigram similarity.
func (ts *TrigramSearcher) SearchSessions(ctx context.Context, tenantID, query string, limit int) ([]SessionSearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if query == "" {
		return nil, nil
	}

	var sql string
	if containsCJK(query) {
		sql = `
			SELECT id, title, platform, user_id, started_at
			FROM sessions
			WHERE tenant_id = $1 AND title IS NOT NULL AND title % $2
			ORDER BY similarity(title, $2) DESC LIMIT $3`
	} else {
		sql = `
			SELECT id, title, platform, user_id, started_at
			FROM sessions
			WHERE tenant_id = $1 AND title IS NOT NULL AND title ILIKE '%' || $2 || '%'
			ORDER BY started_at DESC LIMIT $3`
	}

	rows, err := ts.pool.Query(ctx, sql, tenantID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("trigram search sessions: %w", err)
	}
	defer rows.Close()

	var results []SessionSearchResult
	for rows.Next() {
		var s SessionSearchResult
		if err := rows.Scan(&s.ID, &s.Title, &s.Platform, &s.UserID, &s.StartedAt); err != nil {
			return nil, fmt.Errorf("trigram search sessions scan: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// containsCJK returns true if the string contains CJK Unicode characters.
func containsCJK(s string) bool {
	for _, r := range s {
		if isCJK(r) {
			return true
		}
	}
	return false
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hangul, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r)
}
