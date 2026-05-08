package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

type myMessageStore struct{ db *sql.DB }

func (m *myMessageStore) Append(ctx context.Context, tenantID, sessionID string, msg *store.Message) (int64, error) {
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO messages (tenant_id, session_id, role, content, tool_call_id, tool_calls, tool_name, reasoning, timestamp, token_count, finish_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tenantID, sessionID, msg.Role, msg.Content,
		nullStr(msg.ToolCallID),
		nullStr(msg.ToolCalls),
		nullStr(msg.ToolName),
		nullStr(msg.Reasoning),
		msg.Timestamp,
		nullInt(msg.TokenCount),
		nullStr(msg.FinishReason))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (m *myMessageStore) List(ctx context.Context, tenantID, sessionID string, limit, offset int) ([]*store.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, tenant_id, session_id, role, COALESCE(content,''),
		       COALESCE(tool_call_id,''), COALESCE(tool_calls,''), COALESCE(tool_name,''),
		       COALESCE(reasoning,''), timestamp, COALESCE(token_count,0), COALESCE(finish_reason,'')
		FROM messages WHERE tenant_id = ? AND session_id = ?
		ORDER BY timestamp ASC LIMIT ? OFFSET ?`, tenantID, sessionID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*store.Message
	for rows.Next() {
		msg := &store.Message{}
		if err := rows.Scan(&msg.ID, &msg.TenantID, &msg.SessionID, &msg.Role, &msg.Content,
			&msg.ToolCallID, &msg.ToolCalls, &msg.ToolName,
			&msg.Reasoning, &msg.Timestamp, &msg.TokenCount, &msg.FinishReason); err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

// Search degrades to LIKE-based full-text search on MySQL (no FTS vectors).
func (m *myMessageStore) Search(ctx context.Context, tenantID, query string, limit int) ([]*store.SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	pattern := fmt.Sprintf("%%%s%%", escapeLike(query))
	rows, err := m.db.QueryContext(ctx, `
		SELECT session_id, id, COALESCE(content,''), COALESCE(content,''), 1.0
		FROM messages
		WHERE tenant_id = ? AND content LIKE ?
		ORDER BY timestamp DESC LIMIT ?`, tenantID, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*store.SearchResult
	for rows.Next() {
		r := &store.SearchResult{}
		if err := rows.Scan(&r.SessionID, &r.MessageID, &r.Content, &r.Snippet, &r.Rank); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (m *myMessageStore) CountBySession(ctx context.Context, tenantID, sessionID string) (int, error) {
	var count int
	err := m.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM messages WHERE tenant_id = ? AND session_id = ?`,
		tenantID, sessionID).Scan(&count)
	return count, err
}

// escapeLike escapes MySQL LIKE metacharacters and caps query length at 200 chars.
func escapeLike(q string) string {
	if len([]rune(q)) > 200 {
		runes := []rune(q)
		q = string(runes[:200])
	}
	var b strings.Builder
	for _, c := range q {
		if c == '%' || c == '_' || c == '\\' {
			b.WriteRune('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}

var _ store.MessageStore = (*myMessageStore)(nil)
