package mysql

import (
	"strings"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/store"
)

var _ store.AgentCheckpointStore = (*myAgentCheckpointStore)(nil)

func TestMySQLCheckpointStore_InterfaceCompliance(t *testing.T) {}

func TestMySQLCheckpointStore_UpsertSQL(t *testing.T) {
	sql := `
INSERT INTO agent_checkpoints (tenant_id, session_id, checkpoint_id, payload, updated_at)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE payload=VALUES(payload), updated_at=VALUES(updated_at)`

	if !strings.Contains(sql, "agent_checkpoints") {
		t.Fatal("checkpoint upsert must target agent_checkpoints table")
	}
	if !strings.Contains(sql, "ON DUPLICATE KEY UPDATE") {
		t.Fatal("checkpoint upsert must use ON DUPLICATE KEY UPDATE")
	}
	if !strings.Contains(sql, "payload=VALUES(payload)") {
		t.Fatal("checkpoint upsert must replace payload on conflict")
	}
}

func TestMySQLMessageStore_AgenticBlocksSQL(t *testing.T) {
	insertSQL := `
INSERT INTO messages (tenant_id, session_id, role, content, tool_call_id, tool_calls, tool_name, reasoning, timestamp, token_count, finish_reason, agentic_blocks)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	listSQL := `
SELECT id, tenant_id, session_id, role, COALESCE(content,''),
       COALESCE(tool_call_id,''), COALESCE(tool_calls,''), COALESCE(tool_name,''),
       COALESCE(reasoning,''), timestamp, COALESCE(token_count,0), COALESCE(finish_reason,''),
       COALESCE(agentic_blocks,'')
FROM messages WHERE tenant_id = ? AND session_id = ?
ORDER BY timestamp ASC LIMIT ? OFFSET ?`

	if !strings.Contains(insertSQL, "agentic_blocks") {
		t.Fatal("message INSERT must persist agentic_blocks")
	}
	if !strings.Contains(listSQL, "COALESCE(agentic_blocks,'')") {
		t.Fatal("message SELECT must coalesce agentic_blocks for legacy NULL rows")
	}
}
