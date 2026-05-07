package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ToolExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hermesx_tool_execution_duration_seconds",
			Help:    "Tool execution latency in seconds.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"tool_name", "status", "tenant_id"},
	)

	ToolExecutionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hermesx_tool_executions_total",
			Help: "Total tool executions by name, status, and tenant.",
		},
		[]string{"tool_name", "status", "tenant_id"},
	)

	ChatCompletionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hermesx_chat_completions_total",
			Help: "Total chat completion requests by tenant and status.",
		},
		[]string{"tenant_id", "status"},
	)

	ChatCompletionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hermesx_chat_completion_duration_seconds",
			Help:    "End-to-end chat completion latency including tool calls.",
			Buckets: []float64{0.5, 1, 2.5, 5, 10, 30, 60, 120},
		},
		[]string{"tenant_id"},
	)

	StoreOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hermesx_store_operation_duration_seconds",
			Help:    "Database store operation latency.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"operation", "entity"},
	)
)
