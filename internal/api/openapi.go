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
			"title":       "HermesX Agent API",
			"version":     "2.2.0",
			"description": "Enterprise multi-tenant AI agent platform with RBAC, RLS, and execution audit trail.",
			"contact":     map[string]any{"name": "HermesX Team"},
		},
		"servers": []map[string]any{
			{"url": "/", "description": "Current server"},
		},
		"paths": map[string]any{
			// Infrastructure — no auth required, no /v1 prefix.
			"/health/live":  pathItem("get", "Kubernetes liveness probe", "200"),
			"/health/ready": pathItem("get", "Kubernetes readiness probe (checks DB connectivity)", "200"),
			"/metrics":      pathItem("get", "Prometheus metrics endpoint", "200"),

			// User-facing API (Bearer auth required).
			"/v1/chat/completions": chatPath(),
			"/v1/agent/chat":       agentChatPath(),
			"/v1/sessions":         sessionsPath(),
			"/v1/sessions/{id}":    sessionByIDPath(),
			"/v1/memories":         memoriesPath(),
			"/v1/memories/{key}":   memoryByKeyPath(),

			"/v1/tenants":                 tenantsPath(),
			"/v1/tenants/{id}":            tenantByIDPath(),
			"/v1/api-keys":                apiKeysPath(),
			"/v1/api-keys/{id}":           apiKeyByIDPath(),
			"/v1/audit-logs":              auditLogsPath(),
			"/v1/execution-receipts":      executionReceiptsPath(),
			"/v1/execution-receipts/{id}": executionReceiptByIDPath(),

			"/v1/usage":   pathItem("get", "Usage summary for billing (input/output/cache tokens)", "200"),
			"/v1/me":      pathItem("get", "Current identity, tenant, roles, and scopes", "200"),
			"/v1/openapi": pathItem("get", "This OpenAPI specification", "200"),

			"/v1/gdpr/export":          pathItem("get", "GDPR data export for current tenant", "200"),
			"/v1/gdpr/data":            pathItem("delete", "GDPR data deletion for current tenant", "200"),
			"/v1/gdpr/cleanup-minio":   gdprCleanupPath(),

			"/v1/skills":        pathItem("get", "List tenant skills", "200"),
			"/v1/skills/{name}": pathItem("get", "Get skill content by name", "200"),

			// Bootstrap — unauthenticated, IP rate-limited.
			"/admin/v1/bootstrap":        bootstrapCreatePath(),
			"/admin/v1/bootstrap/status": bootstrapStatusPath(),

			// Admin API (admin scope required).
			"/admin/v1/tenants/{id}/sandbox-policy":           sandboxPolicyPath(),
			"/admin/v1/tenants/{id}/api-keys":                 adminAPIKeysPath(),
			"/admin/v1/tenants/{id}/api-keys/{kid}":           adminAPIKeyByIDPath(),
			"/admin/v1/tenants/{id}/api-keys/{kid}/rotate":    adminAPIKeyRotatePath(),
			"/admin/v1/pricing-rules":                         pricingRulesPath(),
			"/admin/v1/pricing-rules/{model}":                 pricingRuleByModelPath(),
			"/admin/v1/audit-logs":                            adminAuditLogsPath(),
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"BearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "API Key (hk_*) | JWT | Static Token",
					"description":  "Auth chain: Static Token → API Key → OIDC/JWT. Tenant derived from credential, never headers.",
				},
			},
			"schemas": schemas(),
		},
		"security": []map[string]any{
			{"BearerAuth": []string{}},
		},
		"tags": []map[string]any{
			{"name": "Health", "description": "Liveness and readiness probes"},
			{"name": "Chat", "description": "OpenAI-compatible chat completions"},
			{"name": "Sessions", "description": "Conversation session management"},
			{"name": "Memory", "description": "Per-user key-value memory"},
			{"name": "Bootstrap", "description": "One-time platform admin key creation"},
			{"name": "Admin", "description": "Tenant, API key, and pricing management (admin scope required)"},
			{"name": "Audit", "description": "Audit logs and execution receipts (auditor role required)"},
			{"name": "GDPR", "description": "Data export and deletion"},
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


func chatPath() map[string]any {
	return map[string]any{
		"post": map[string]any{
			"tags":    []string{"Chat"},
			"summary": "OpenAI-compatible chat completions with agent tool loop",
			"requestBody": map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/ChatRequest"},
					},
				},
			},
			"responses": map[string]any{
				"200": map[string]any{"description": "Chat response (streaming or JSON)"},
				"401": map[string]any{"description": "Unauthorized"},
				"429": map[string]any{"description": "Rate limit exceeded"},
			},
		},
	}
}

func sessionsPath() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"tags":    []string{"Sessions"},
			"summary": "List sessions for current tenant",
			"parameters": []map[string]any{
				{"name": "limit", "in": "query", "schema": map[string]any{"type": "integer", "default": 50}},
				{"name": "offset", "in": "query", "schema": map[string]any{"type": "integer", "default": 0}},
				{"name": "user_id", "in": "query", "schema": map[string]any{"type": "string"}},
			},
			"responses": map[string]any{
				"200": map[string]any{"description": "Session list with pagination"},
			},
		},
	}
}

func sessionByIDPath() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"tags":    []string{"Sessions"},
			"summary": "Get session by ID (tenant-scoped)",
			"responses": map[string]any{
				"200": map[string]any{"description": "Session details with messages"},
				"404": map[string]any{"description": "Session not found or belongs to another tenant"},
			},
		},
		"delete": map[string]any{
			"tags":    []string{"Sessions"},
			"summary": "Delete session (tenant-scoped)",
			"responses": map[string]any{
				"204": map[string]any{"description": "Deleted"},
				"404": map[string]any{"description": "Not found"},
			},
		},
	}
}

func memoriesPath() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"tags":    []string{"Memory"},
			"summary": "List memories for user (tenant-scoped)",
			"parameters": []map[string]any{
				{"name": "X-Hermes-User-Id", "in": "header", "required": true, "schema": map[string]any{"type": "string"}},
			},
			"responses": map[string]any{
				"200": map[string]any{"description": "Memory entries"},
			},
		},
		"post": map[string]any{
			"tags":    []string{"Memory"},
			"summary": "Upsert a memory key-value pair",
			"responses": map[string]any{
				"200": map[string]any{"description": "Upserted"},
			},
		},
	}
}

func memoryByKeyPath() map[string]any {
	return map[string]any{
		"delete": map[string]any{
			"tags":    []string{"Memory"},
			"summary": "Delete a memory key",
			"responses": map[string]any{
				"204": map[string]any{"description": "Deleted"},
			},
		},
	}
}

func tenantsPath() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "List all tenants (admin only)",
			"responses": map[string]any{
				"200": map[string]any{"description": "Tenant list"},
				"403": map[string]any{"description": "Forbidden — admin role required"},
			},
		},
		"post": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "Create a new tenant (admin only)",
			"responses": map[string]any{
				"201": map[string]any{"description": "Created"},
				"403": map[string]any{"description": "Forbidden"},
			},
		},
	}
}

func tenantByIDPath() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "Get tenant by ID",
			"responses": map[string]any{
				"200": map[string]any{"description": "Tenant details"},
			},
		},
		"delete": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "Soft-delete a tenant",
			"responses": map[string]any{
				"204": map[string]any{"description": "Deleted"},
			},
		},
	}
}

func apiKeysPath() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "List API keys for tenant",
			"responses": map[string]any{
				"200": map[string]any{"description": "API key list (hashes never exposed)"},
			},
		},
		"post": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "Create API key (raw key returned only on creation)",
			"requestBody": map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/CreateAPIKeyRequest"},
					},
				},
			},
			"responses": map[string]any{
				"201": map[string]any{"description": "Key created — raw key in response"},
				"403": map[string]any{"description": "Non-admin cannot specify tenant_id"},
			},
		},
	}
}

func apiKeyByIDPath() map[string]any {
	return map[string]any{
		"delete": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "Revoke an API key",
			"responses": map[string]any{
				"204": map[string]any{"description": "Revoked"},
				"404": map[string]any{"description": "Key not found"},
			},
		},
	}
}

func auditLogsPath() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"tags":    []string{"Audit"},
			"summary": "List audit logs for tenant (auditor role required)",
			"parameters": []map[string]any{
				{"name": "action", "in": "query", "schema": map[string]any{"type": "string"}},
				{"name": "from", "in": "query", "schema": map[string]any{"type": "string", "format": "date-time"}},
				{"name": "to", "in": "query", "schema": map[string]any{"type": "string", "format": "date-time"}},
				{"name": "limit", "in": "query", "schema": map[string]any{"type": "integer", "default": 50}},
				{"name": "offset", "in": "query", "schema": map[string]any{"type": "integer", "default": 0}},
			},
			"responses": map[string]any{
				"200": map[string]any{"description": "Audit log entries with pagination"},
				"403": map[string]any{"description": "Forbidden — auditor role required"},
			},
		},
	}
}

func executionReceiptsPath() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"tags":    []string{"Audit"},
			"summary": "List execution receipts for tenant (auditor role required)",
			"parameters": []map[string]any{
				{"name": "session_id", "in": "query", "schema": map[string]any{"type": "string"}},
				{"name": "tool_name", "in": "query", "schema": map[string]any{"type": "string"}},
				{"name": "status", "in": "query", "schema": map[string]any{"type": "string", "enum": []string{"success", "error", "timeout"}}},
				{"name": "limit", "in": "query", "schema": map[string]any{"type": "integer", "default": 50}},
				{"name": "offset", "in": "query", "schema": map[string]any{"type": "integer", "default": 0}},
			},
			"responses": map[string]any{
				"200": map[string]any{"description": "Execution receipt list with pagination"},
				"403": map[string]any{"description": "Forbidden — auditor role required"},
			},
		},
	}
}

func executionReceiptByIDPath() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"tags":    []string{"Audit"},
			"summary": "Get execution receipt by ID",
			"responses": map[string]any{
				"200": map[string]any{"description": "Receipt details"},
				"404": map[string]any{"description": "Not found"},
			},
		},
	}
}

func agentChatPath() map[string]any {
	return map[string]any{
		"post": map[string]any{
			"tags":    []string{"Chat"},
			"summary": "Agent chat (alias for /v1/chat/completions with session persistence)",
			"requestBody": map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/ChatRequest"},
					},
				},
			},
			"responses": map[string]any{
				"200": map[string]any{"description": "Chat response (streaming or JSON)"},
				"400": map[string]any{"description": "No user message in request"},
				"401": map[string]any{"description": "Unauthorized"},
				"403": map[string]any{"description": "Session belongs to another user"},
			},
		},
	}
}

func gdprCleanupPath() map[string]any {
	return map[string]any{
		"post": map[string]any{
			"tags":    []string{"GDPR"},
			"summary": "Purge orphaned MinIO skill objects for current tenant",
			"responses": map[string]any{
				"200": map[string]any{"description": "Cleanup complete"},
				"403": map[string]any{"description": "Forbidden"},
			},
		},
	}
}

func bootstrapCreatePath() map[string]any {
	return map[string]any{
		"post": map[string]any{
			"tags":    []string{"Bootstrap"},
			"summary": "Create the platform's first admin API key (idempotent, IP rate-limited to 5 RPM)",
			"requestBody": map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/BootstrapRequest"},
					},
				},
			},
			"responses": map[string]any{
				"200": map[string]any{"description": "Admin key created — raw key in response"},
				"403": map[string]any{"description": "Bootstrap already completed"},
				"429": map[string]any{"description": "Rate limit exceeded"},
			},
			"security": []map[string]any{},
		},
	}
}

func bootstrapStatusPath() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"tags":    []string{"Bootstrap"},
			"summary": "Check whether platform bootstrap has been completed",
			"responses": map[string]any{
				"200": map[string]any{"description": "Bootstrap status (completed: bool)"},
			},
			"security": []map[string]any{},
		},
	}
}

func sandboxPolicyPath() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "Get sandbox policy for a tenant",
			"responses": map[string]any{
				"200": map[string]any{"description": "Sandbox policy"},
				"404": map[string]any{"description": "Policy not set"},
			},
		},
		"post": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "Set sandbox policy for a tenant",
			"requestBody": map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/SandboxPolicy"},
					},
				},
			},
			"responses": map[string]any{
				"200": map[string]any{"description": "Policy updated"},
			},
		},
		"delete": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "Delete sandbox policy for a tenant (reverts to default)",
			"responses": map[string]any{
				"204": map[string]any{"description": "Deleted"},
			},
		},
	}
}

func adminAPIKeysPath() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "List API keys for a specific tenant (admin scope required)",
			"responses": map[string]any{
				"200": map[string]any{"description": "API key list"},
			},
		},
		"post": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "Create API key for a specific tenant",
			"requestBody": map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/CreateAPIKeyRequest"},
					},
				},
			},
			"responses": map[string]any{
				"201": map[string]any{"description": "Key created — raw key in response"},
			},
		},
	}
}

func adminAPIKeyByIDPath() map[string]any {
	return map[string]any{
		"delete": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "Revoke an API key for a specific tenant",
			"responses": map[string]any{
				"204": map[string]any{"description": "Revoked"},
				"404": map[string]any{"description": "Key not found"},
			},
		},
	}
}

func adminAPIKeyRotatePath() map[string]any {
	return map[string]any{
		"post": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "Rotate (revoke and re-create) an API key",
			"responses": map[string]any{
				"200": map[string]any{"description": "New key — raw key in response"},
			},
		},
	}
}

func pricingRulesPath() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "List all pricing rules",
			"responses": map[string]any{
				"200": map[string]any{"description": "Pricing rule list"},
			},
		},
	}
}

func pricingRuleByModelPath() map[string]any {
	return map[string]any{
		"put": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "Upsert pricing rule for a model",
			"responses": map[string]any{
				"200": map[string]any{"description": "Rule upserted"},
			},
		},
		"delete": map[string]any{
			"tags":    []string{"Admin"},
			"summary": "Delete pricing rule for a model",
			"responses": map[string]any{
				"204": map[string]any{"description": "Deleted"},
			},
		},
	}
}

func adminAuditLogsPath() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"tags":    []string{"Audit"},
			"summary": "List audit logs across all tenants (admin scope required)",
			"parameters": []map[string]any{
				{"name": "tenant_id", "in": "query", "schema": map[string]any{"type": "string"}},
				{"name": "action", "in": "query", "schema": map[string]any{"type": "string"}},
				{"name": "from", "in": "query", "schema": map[string]any{"type": "string", "format": "date-time"}},
				{"name": "to", "in": "query", "schema": map[string]any{"type": "string", "format": "date-time"}},
				{"name": "limit", "in": "query", "schema": map[string]any{"type": "integer", "default": 50}},
				{"name": "offset", "in": "query", "schema": map[string]any{"type": "integer", "default": 0}},
			},
			"responses": map[string]any{
				"200": map[string]any{"description": "Audit log entries with pagination"},
				"403": map[string]any{"description": "Forbidden — admin scope required"},
			},
		},
	}
}

func schemas() map[string]any {
	return map[string]any{
		"ChatRequest": map[string]any{
			"type":     "object",
			"required": []string{"messages"},
			"properties": map[string]any{
				"model":    map[string]any{"type": "string", "description": "Model identifier"},
				"messages": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Message"}},
				"stream":   map[string]any{"type": "boolean", "default": true},
			},
		},
		"Message": map[string]any{
			"type":     "object",
			"required": []string{"role", "content"},
			"properties": map[string]any{
				"role":    map[string]any{"type": "string", "enum": []string{"system", "user", "assistant", "tool"}},
				"content": map[string]any{"type": "string"},
			},
		},
		"CreateAPIKeyRequest": map[string]any{
			"type":     "object",
			"required": []string{"name"},
			"properties": map[string]any{
				"name":      map[string]any{"type": "string"},
				"roles":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "default": []string{"user"}},
				"scopes":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Fine-grained scopes; empty = legacy role-only access"},
				"tenant_id": map[string]any{"type": "string", "description": "Only admin callers may specify a foreign tenant_id"},
			},
		},
		"BootstrapRequest": map[string]any{
			"type":     "object",
			"required": []string{"name", "tenant_id"},
			"properties": map[string]any{
				"name":      map[string]any{"type": "string", "description": "Display name for the bootstrap admin key"},
				"tenant_id": map[string]any{"type": "string", "format": "uuid", "description": "Platform root tenant ID"},
			},
		},
		"ExecutionReceipt": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":             map[string]any{"type": "string", "format": "uuid"},
				"tenant_id":      map[string]any{"type": "string", "format": "uuid"},
				"session_id":     map[string]any{"type": "string"},
				"user_id":        map[string]any{"type": "string"},
				"tool_name":      map[string]any{"type": "string"},
				"input":          map[string]any{"type": "string"},
				"output":         map[string]any{"type": "string"},
				"status":         map[string]any{"type": "string", "enum": []string{"success", "error", "timeout"}},
				"duration_ms":    map[string]any{"type": "integer"},
				"idempotency_id": map[string]any{"type": "string"},
				"trace_id":       map[string]any{"type": "string"},
				"created_at":     map[string]any{"type": "string", "format": "date-time"},
			},
		},
		"Tenant": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":             map[string]any{"type": "string", "format": "uuid"},
				"name":           map[string]any{"type": "string"},
				"plan":           map[string]any{"type": "string", "enum": []string{"free", "pro", "enterprise"}},
				"rate_limit_rpm": map[string]any{"type": "integer"},
				"max_sessions":   map[string]any{"type": "integer"},
				"sandbox_policy": map[string]any{"$ref": "#/components/schemas/SandboxPolicy"},
			},
		},
		"SandboxPolicy": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"enabled":             map[string]any{"type": "boolean"},
				"max_timeout_seconds": map[string]any{"type": "integer"},
				"allowed_tools":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"allow_docker":        map[string]any{"type": "boolean"},
				"restrict_network":    map[string]any{"type": "boolean"},
				"max_stdout_kb":       map[string]any{"type": "integer"},
			},
		},
	}
}
