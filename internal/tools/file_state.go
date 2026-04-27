package tools

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// FileStateCoordinator tracks file modifications across parallel agents
// to prevent conflicting writes.
type FileStateCoordinator struct {
	mu     sync.RWMutex
	states map[string]*fileState
}

type fileState struct {
	LastModifiedBy string
	ModifiedAt     time.Time
	Hash           string
}

var globalFileState = &FileStateCoordinator{states: make(map[string]*fileState)}

// GlobalFileState returns the global coordinator.
func GlobalFileState() *FileStateCoordinator { return globalFileState }

// MarkModified records that an agent modified a file.
func (c *FileStateCoordinator) MarkModified(path, agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.states[path] = &fileState{
		LastModifiedBy: agentID,
		ModifiedAt:     time.Now(),
	}
}

// CheckConflict returns true if another agent recently modified the file.
func (c *FileStateCoordinator) CheckConflict(path, agentID string) (bool, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state, ok := c.states[path]
	if !ok {
		return false, ""
	}
	if state.LastModifiedBy != agentID && time.Since(state.ModifiedAt) < 5*time.Minute {
		return true, state.LastModifiedBy
	}
	return false, ""
}

// GetState returns file state info as JSON.
func (c *FileStateCoordinator) GetState(path string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state, ok := c.states[path]
	if !ok {
		return toJSON(map[string]any{"status": "no_state"})
	}
	b, _ := json.Marshal(map[string]any{
		"last_modified_by": state.LastModifiedBy,
		"modified_at":      state.ModifiedAt.Format(time.RFC3339),
	})
	return string(b)
}

func init() {
	if os.Getenv("HERMES_FILE_STATE") == "" {
		return
	}

	Register(&ToolEntry{
		Name:        "file_state",
		Toolset:     "file_state",
		Description: "Check file modification state across agents",
		Emoji:       "📋",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"check", "mark"}, "description": "Action to perform"},
				"path":   map[string]any{"type": "string", "description": "File path"},
			},
			"required": []string{"action", "path"},
		},
		Handler: handleFileState,
	})
}

func handleFileState(args map[string]any, ctx *ToolContext) string {
	action, _ := args["action"].(string)
	path, _ := args["path"].(string)

	switch action {
	case "check":
		conflict, owner := globalFileState.CheckConflict(path, ctx.SessionID)
		return toJSON(map[string]any{"conflict": conflict, "owner": owner, "state": globalFileState.GetState(path)})
	case "mark":
		globalFileState.MarkModified(path, ctx.SessionID)
		return toJSON(map[string]any{"status": "marked"})
	default:
		return toJSON(map[string]any{"error": "unknown action: " + action})
	}
}
