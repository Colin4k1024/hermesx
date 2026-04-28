package api

import (
	"encoding/json"
	"net/http"
)

// OpenAPISpec returns the embedded OpenAPI 3.0 specification.
func OpenAPISpec() http.HandlerFunc {
	spec := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "Hermes Agent API",
			"version":     "1.0.0",
			"description": "Multi-tenant AI agent platform API",
		},
		"paths": map[string]any{
			"/v1/health":       pathItem("GET", "Health check", "200"),
			"/v1/health/live":  pathItem("GET", "Kubernetes liveness probe", "200"),
			"/v1/health/ready": pathItem("GET", "Kubernetes readiness probe", "200"),
			"/v1/chat":         pathItem("POST", "Send a chat message", "200"),
			"/v1/status":       pathItem("GET", "Server status", "200"),
			"/v1/sessions":     pathItem("GET", "List sessions", "200"),
			"/v1/tenants":      pathItem("GET", "List tenants (admin)", "200"),
			"/v1/api-keys":     pathItem("GET", "List API keys", "200"),
			"/v1/audit-logs":   pathItem("GET", "List audit logs", "200"),
			"/v1/usage":        pathItem("GET", "Usage summary for billing", "200"),
			"/v1/metrics":      pathItem("GET", "Prometheus metrics", "200"),
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"BearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT or API Key or Static Token",
				},
			},
		},
		"security": []map[string]any{
			{"BearerAuth": []string{}},
		},
	}

	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(spec)
	}
}

func pathItem(method, summary, status string) map[string]any {
	return map[string]any{
		method: map[string]any{
			"summary": summary,
			"responses": map[string]any{
				status: map[string]any{"description": "Success"},
			},
		},
	}
}
