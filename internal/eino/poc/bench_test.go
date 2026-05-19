package poc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

func BenchmarkReactAgent_SingleToolLoop(b *testing.B) {
	toolCallArgs, _ := json.Marshal(map[string]any{"query": "test"})

	transport := &mockTransport{
		responses: []llm.ChatResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "c1", Type: "function", Function: llm.FunctionCall{Name: "search", Arguments: string(toolCallArgs)}},
				},
				FinishReason: "tool_calls",
			},
			{Content: "Result based on search.", FinishReason: "stop"},
		},
	}

	toolEntries := []*tools.ToolEntry{
		{
			Name:        "search",
			Description: "Search for information",
			Schema: map[string]any{
				"name":        "search",
				"description": "Search for information",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string", "description": "search query"},
					},
					"required": []string{"query"},
				},
			},
			Handler: func(_ context.Context, _ map[string]any, _ *tools.ToolContext) string {
				return "search result: some information"
			},
		},
	}

	ctx := context.Background()
	agent, err := NewReactAgent(ctx, ReactAgentConfig{
		Transport: transport,
		ModelName: "bench-model",
		ToolSet:   toolEntries,
		MaxStep:   10,
	})
	if err != nil {
		b.Fatalf("NewReactAgent: %v", err)
	}

	tctx := &tools.ToolContext{TaskID: "bench", SessionID: "bench-session"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		transport.callCount = 0
		_, err := agent.Generate(ctx, "search for test", tctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReactAgent_MultiToolLoop(b *testing.B) {
	args1, _ := json.Marshal(map[string]any{"query": "weather"})
	args2, _ := json.Marshal(map[string]any{"command": "date"})

	transport := &mockTransport{
		responses: []llm.ChatResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "c1", Type: "function", Function: llm.FunctionCall{Name: "search", Arguments: string(args1)}},
				},
				FinishReason: "tool_calls",
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "c2", Type: "function", Function: llm.FunctionCall{Name: "terminal", Arguments: string(args2)}},
				},
				FinishReason: "tool_calls",
			},
			{Content: "The weather is sunny and today is Monday.", FinishReason: "stop"},
		},
	}

	toolEntries := []*tools.ToolEntry{
		{
			Name:        "search",
			Description: "Search for information",
			Schema: map[string]any{
				"name": "search", "description": "Search",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string", "description": "query"},
					},
					"required": []string{"query"},
				},
			},
			Handler: func(_ context.Context, _ map[string]any, _ *tools.ToolContext) string {
				return "sunny, 22C"
			},
		},
		{
			Name:        "terminal",
			Description: "Run command",
			Schema: map[string]any{
				"name": "terminal", "description": "Run command",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{"type": "string", "description": "cmd"},
					},
					"required": []string{"command"},
				},
			},
			Handler: func(_ context.Context, _ map[string]any, _ *tools.ToolContext) string {
				return "Mon May 19 2025"
			},
		},
	}

	ctx := context.Background()
	agent, err := NewReactAgent(ctx, ReactAgentConfig{
		Transport: transport,
		ModelName: "bench-model",
		ToolSet:   toolEntries,
		MaxStep:   10,
	})
	if err != nil {
		b.Fatalf("NewReactAgent: %v", err)
	}

	tctx := &tools.ToolContext{TaskID: "bench", SessionID: "bench-session"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		transport.callCount = 0
		_, err := agent.Generate(ctx, "what's the weather and date?", tctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReactAgent_NoToolCall(b *testing.B) {
	transport := &mockTransport{
		responses: []llm.ChatResponse{
			{Content: "Hello! I'm here to help.", FinishReason: "stop"},
		},
	}

	toolEntries := []*tools.ToolEntry{
		{
			Name:        "search",
			Description: "Search",
			Schema: map[string]any{
				"name": "search", "description": "Search",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
				},
			},
			Handler: func(_ context.Context, _ map[string]any, _ *tools.ToolContext) string {
				return ""
			},
		},
	}

	ctx := context.Background()
	agent, err := NewReactAgent(ctx, ReactAgentConfig{
		Transport: transport,
		ModelName: "bench-model",
		ToolSet:   toolEntries,
	})
	if err != nil {
		b.Fatalf("NewReactAgent: %v", err)
	}

	tctx := &tools.ToolContext{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		transport.callCount = 0
		_, err := agent.Generate(ctx, "hi", tctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// latencyTransport wraps mockTransport and injects realistic network latency.
type latencyTransport struct {
	inner   *mockTransport
	latency time.Duration
}

func (lt *latencyTransport) Name() string { return "latency-mock" }

func (lt *latencyTransport) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	time.Sleep(lt.latency)
	return lt.inner.Chat(ctx, req)
}

func (lt *latencyTransport) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamDelta, <-chan error) {
	time.Sleep(lt.latency)
	return lt.inner.ChatStream(ctx, req)
}

// BenchmarkReactAgent_WithLatency simulates real-world LLM response times
// to measure framework overhead relative to network latency.
func BenchmarkReactAgent_WithLatency_50ms(b *testing.B) {
	benchWithLatency(b, 50*time.Millisecond)
}

func BenchmarkReactAgent_WithLatency_200ms(b *testing.B) {
	benchWithLatency(b, 200*time.Millisecond)
}

func benchWithLatency(b *testing.B, latency time.Duration) {
	b.Helper()
	toolCallArgs, _ := json.Marshal(map[string]any{"query": "test"})

	inner := &mockTransport{
		responses: []llm.ChatResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "c1", Type: "function", Function: llm.FunctionCall{Name: "search", Arguments: string(toolCallArgs)}},
				},
				FinishReason: "tool_calls",
			},
			{Content: "Done.", FinishReason: "stop"},
		},
	}

	transport := &latencyTransport{inner: inner, latency: latency}

	toolEntries := []*tools.ToolEntry{
		{
			Name:        "search",
			Description: "Search",
			Schema: map[string]any{
				"name": "search", "description": "Search",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
					"required": []string{"query"},
				},
			},
			Handler: func(_ context.Context, _ map[string]any, _ *tools.ToolContext) string {
				return "result"
			},
		},
	}

	ctx := context.Background()
	agent, err := NewReactAgent(ctx, ReactAgentConfig{
		Transport: transport,
		ModelName: "bench-latency",
		ToolSet:   toolEntries,
		MaxStep:   10,
	})
	if err != nil {
		b.Fatalf("NewReactAgent: %v", err)
	}

	tctx := &tools.ToolContext{SessionID: "bench"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		inner.callCount = 0
		_, err := agent.Generate(ctx, "search test", tctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReactAgent_Concurrent measures throughput under parallel load.
func BenchmarkReactAgent_Concurrent10(b *testing.B) {
	toolCallArgs, _ := json.Marshal(map[string]any{"query": "test"})

	toolEntries := []*tools.ToolEntry{
		{
			Name:        "search",
			Description: "Search",
			Schema: map[string]any{
				"name": "search", "description": "Search",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
					"required": []string{"query"},
				},
			},
			Handler: func(_ context.Context, _ map[string]any, _ *tools.ToolContext) string {
				return "result"
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.SetParallelism(10)
	b.RunParallel(func(pb *testing.PB) {
		transport := &mockTransport{
			responses: []llm.ChatResponse{
				{
					ToolCalls: []llm.ToolCall{
						{ID: "c1", Type: "function", Function: llm.FunctionCall{Name: "search", Arguments: string(toolCallArgs)}},
					},
					FinishReason: "tool_calls",
				},
				{Content: "Done.", FinishReason: "stop"},
			},
		}

		ctx := context.Background()
		agent, err := NewReactAgent(ctx, ReactAgentConfig{
			Transport: transport,
			ModelName: "bench-concurrent",
			ToolSet:   toolEntries,
			MaxStep:   10,
		})
		if err != nil {
			b.Fatalf("NewReactAgent: %v", err)
		}

		tctx := &tools.ToolContext{SessionID: "bench-concurrent"}

		for pb.Next() {
			transport.callCount = 0
			_, err := agent.Generate(ctx, "search test", tctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
