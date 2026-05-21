package pg

import (
	"strings"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/store"
)

var _ store.AgentCheckpointStore = (*pgAgentCheckpointStore)(nil)

func TestPGCheckpointStore_InterfaceCompliance(t *testing.T) {}

func TestPGCheckpointStore_UpsertSQL(t *testing.T) {
	sql := `
INSERT INTO agent_checkpoints (tenant_id, session_id, checkpoint_id, payload, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (tenant_id, session_id, checkpoint_id)
DO UPDATE SET payload=EXCLUDED.payload, updated_at=EXCLUDED.updated_at`

	if !strings.Contains(sql, "agent_checkpoints") {
		t.Fatal("checkpoint upsert must target agent_checkpoints table")
	}
	if !strings.Contains(sql, "ON CONFLICT (tenant_id, session_id, checkpoint_id)") {
		t.Fatal("checkpoint upsert must use tenant/session/checkpoint composite conflict key")
	}
	if !strings.Contains(sql, "payload=EXCLUDED.payload") {
		t.Fatal("checkpoint upsert must replace payload on conflict")
	}
}

func TestPGMessageStore_AgenticBlocksSQL(t *testing.T) {
	insertSQL := `
INSERT INTO messages (tenant_id, session_id, role, content, tool_call_id, tool_calls, tool_name, reasoning, timestamp, token_count, finish_reason, agentic_blocks)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id`
	listSQL := `
SELECT id, tenant_id, session_id, role, content, tool_call_id, tool_calls, tool_name,
       reasoning, timestamp, token_count, finish_reason, agentic_blocks
FROM messages WHERE tenant_id = $1 AND session_id = $2
ORDER BY timestamp ASC LIMIT $3 OFFSET $4`

	if !strings.Contains(insertSQL, "agentic_blocks") {
		t.Fatal("message INSERT must persist agentic_blocks")
	}
	if !strings.Contains(listSQL, "agentic_blocks") {
		t.Fatal("message SELECT must read agentic_blocks")
	}
}
