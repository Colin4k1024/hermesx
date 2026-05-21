package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/eino"
	"github.com/Colin4k1024/hermesx/internal/llm"
)

func TestServeAgentHTTP_IncludeAgenticBlocksOptIn(t *testing.T) {
	blocks := []eino.AgenticBlock{{
		Type: "server_tool_call",
		Data: map[string]any{"server_tool_call": map[string]any{"name": "web_search"}},
	}}
	h := NewChatHandler(stubStore{}, nil, nil)
	h.llmModel = "test-model"
	h.runAgent = func(_ context.Context, _ string, _ []llm.Message, _ *eino.StreamCallbacks) (*eino.ConversationResult, error) {
		return &eino.ConversationResult{FinalResponse: "done", AgenticBlocks: blocks}, nil
	}

	t.Run("omits blocks by default", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := newAgentChatRequest(t, `{"messages":[{"role":"user","content":"hi"}]}`)

		h.ServeAgentHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if _, ok := body["agentic_blocks"]; ok {
			t.Fatal("expected agentic_blocks to be omitted when include_agentic_blocks is false")
		}
	})

	t.Run("returns blocks when opted in", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := newAgentChatRequest(t, `{"messages":[{"role":"user","content":"hi"}],"include_agentic_blocks":true}`)

		h.ServeAgentHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body struct {
			AgenticBlocks []eino.AgenticBlock `json:"agentic_blocks"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(body.AgenticBlocks) != 1 {
			t.Fatalf("agentic_blocks len = %d, want 1", len(body.AgenticBlocks))
		}
		if body.AgenticBlocks[0].Type != "server_tool_call" {
			t.Fatalf("agentic_blocks[0].type = %q, want server_tool_call", body.AgenticBlocks[0].Type)
		}
	})
}

func TestServeAgentHTTP_StreamAgenticBlockEvent(t *testing.T) {
	block := eino.AgenticBlock{
		Type: "server_tool_call",
		Data: map[string]any{"server_tool_call": map[string]any{"name": "web_search"}},
	}
	h := NewChatHandler(stubStore{}, nil, nil)
	h.llmModel = "test-model"
	h.runAgent = func(_ context.Context, _ string, _ []llm.Message, callbacks *eino.StreamCallbacks) (*eino.ConversationResult, error) {
		if callbacks != nil {
			callbacks.OnStreamDelta("partial")
			callbacks.OnAgenticBlock(block)
		}
		return &eino.ConversationResult{FinalResponse: "done", AgenticBlocks: []eino.AgenticBlock{block}}, nil
	}

	t.Run("emits agentic block event when opted in", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := newAgentChatRequest(t, `{"messages":[{"role":"user","content":"hi"}],"stream":true,"include_agentic_blocks":true}`)

		h.ServeAgentHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "event: agentic_block") {
			t.Fatalf("expected SSE body to contain agentic_block event, got %q", body)
		}
		if !strings.Contains(body, `"type":"server_tool_call"`) {
			t.Fatalf("expected SSE body to contain serialized block payload, got %q", body)
		}
	})

	t.Run("hides agentic block event by default", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := newAgentChatRequest(t, `{"messages":[{"role":"user","content":"hi"}],"stream":true}`)

		h.ServeAgentHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, "event: agentic_block") {
			t.Fatalf("expected SSE body to hide agentic_block event by default, got %q", body)
		}
	})
}

func newAgentChatRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/chat", strings.NewReader(body))
	ctx := auth.WithContext(req.Context(), &auth.AuthContext{TenantID: "tenant-1", Identity: "user-1"})
	return req.WithContext(ctx)
}
