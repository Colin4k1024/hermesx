package tools

import (
	"context"
	"testing"
)

func TestToolRegistry_Lookup(t *testing.T) {
	reg := Registry()

	// Register a minimal tool on the global registry.
	reg.Register(&ToolEntry{
		Name:        "lookup-test-tool-xyz",
		Description: "test",
		Toolset:     "test",
		Emoji:       "🔧",
		Handler: func(_ context.Context, _ map[string]any, _ *ToolContext) string {
			return ""
		},
	})
	defer reg.Deregister("lookup-test-tool-xyz")

	// Lookup existing tool.
	entry := reg.Lookup("lookup-test-tool-xyz")
	if entry == nil {
		t.Fatal("Lookup should return registered tool")
	}
	if entry.Name != "lookup-test-tool-xyz" {
		t.Errorf("Name = %q, want lookup-test-tool-xyz", entry.Name)
	}

	// Lookup non-existing tool.
	if got := reg.Lookup("nonexistent-tool-xyz"); got != nil {
		t.Errorf("Lookup nonexistent should return nil, got %+v", got)
	}
}

func TestToolRegistry_GetEmoji(t *testing.T) {
	reg := Registry()

	reg.Register(&ToolEntry{
		Name:  "emoji-tool-xyz",
		Emoji: "🚀",
		Handler: func(_ context.Context, _ map[string]any, _ *ToolContext) string {
			return ""
		},
	})
	defer reg.Deregister("emoji-tool-xyz")

	// Known tool with emoji.
	if got := reg.GetEmoji("emoji-tool-xyz", "⬛"); got != "🚀" {
		t.Errorf("GetEmoji = %q, want 🚀", got)
	}

	// Unknown tool - returns default.
	if got := reg.GetEmoji("unknown-tool-xyz", "⬛"); got != "⬛" {
		t.Errorf("GetEmoji unknown = %q, want ⬛", got)
	}
}

func TestToolRegistry_GetToolToToolsetMap(t *testing.T) {
	reg := Registry()

	reg.Register(&ToolEntry{
		Name:    "toolmap-a-xyz",
		Toolset: "set-1",
		Handler: func(_ context.Context, _ map[string]any, _ *ToolContext) string {
			return ""
		},
	})
	reg.Register(&ToolEntry{
		Name:    "toolmap-b-xyz",
		Toolset: "set-2",
		Handler: func(_ context.Context, _ map[string]any, _ *ToolContext) string {
			return ""
		},
	})
	defer reg.Deregister("toolmap-a-xyz")
	defer reg.Deregister("toolmap-b-xyz")

	m := reg.GetToolToToolsetMap()
	if m["toolmap-a-xyz"] != "set-1" {
		t.Errorf("toolmap-a toolset = %q, want set-1", m["toolmap-a-xyz"])
	}
	if m["toolmap-b-xyz"] != "set-2" {
		t.Errorf("toolmap-b toolset = %q, want set-2", m["toolmap-b-xyz"])
	}
}
