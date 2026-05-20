# HermesX

**Enterprise Agent Runtime & Multi-Tenant SaaS Control Plane**

A production-grade platform for deploying, isolating, and governing AI agents at enterprise scale. Built in Go for single-binary deployment, native concurrency, and zero-dependency distribution.

> Originally inspired by Nous Research's [hermes-agent](https://github.com/NousResearch/hermes-agent). HermesX has evolved into a standalone enterprise platform with multi-tenant isolation, RBAC, audit trails, sandbox execution, and SaaS-grade observability — far beyond the original agent framework's scope.

---

## Quick Links

| | |
|---|---|
| [SaaS Quickstart](saas-quickstart.md) | Get a tenant up and running in minutes |
| [API Reference](api-reference.md) | Full endpoint documentation |
| [Architecture Overview](architecture.md) | System design and component map |
| [Configuration](configuration.md) | All environment variables and config options |
| [Deployment](deployment.md) | Docker, Kubernetes, and bare-metal guides |
| [Security Model](SECURITY_MODEL.md) | Threat model, RLS, sandbox isolation |
| [RBAC Matrix](RBAC_MATRIX.md) | 5 roles × 10 resources permission matrix |
| [Skills Guide](skills-guide.md) | Skill system user guide |
| [Workflow Engine Guide](workflow-guide.en.md) | Fixed SOP workflow usage guide |
| [Scheduler Guide](scheduler-guide.en.md) | Distributed scheduler deployment & testing |

---

## Project Stats

| Metric | Value |
|--------|-------|
| Go source files | 413+ |
| Lines of code | 78,000+ |
| Registered tools | 52 (42 core + 10 RL training) |
| Platform adapters | 14 |
| Terminal backends | 8 |
| LLM providers | 7 |
| Bundled skill categories | 39 directories |
| Test files | 127 |
| Total tests | 1,828 |
| RLS-protected tables | 11 |
| API endpoints | 51+ |
| Version | v2.3.0 |

---

## Core Capabilities

### Enterprise SaaS Platform

- **Multi-tenant isolation** — PostgreSQL Row-Level Security (RLS), per-transaction `SET LOCAL app.current_tenant`, 10 RLS-protected tables
- **Auth chain** — Static Token → API Key (SHA-256 hashed) → JWT/OIDC, multi-level fallback
- **API Key scopes** — `read` / `write` / `execute` / `admin` / `audit` / `gdpr` — six-dimensional fine-grained authorization
- **5 roles** — `super_admin`, `admin`, `owner`, `user`, `auditor`, covering all operational paths
- **Dual-layer rate limiting** — atomic Redis Lua script (tenant + user sliding windows), auto-fallback to local LRU when Redis is unavailable
- **Token metering** — async batch persistence, DB-first + hardcoded dual-layer cost calculation, supports custom pricing rules
- **Execution receipts** — auditable tool call records with idempotent deduplication and OpenTelemetry trace correlation
- **Audit trail** — immutable logs for all state-changing operations, cross-tenant queries for `super_admin`
- **GDPR compliance** — full-chain data export (JSON) + transactional deletion + MinIO object storage cleanup
- **Sandbox isolation** — per-tenant code execution supporting local / Docker / K8s Job modes (`SANDBOX_MODE` env var), Docker network/resource limits, policy manageable via Admin API
- **Bootstrap protection** — bootstrap endpoint dual IP rate limiting (application layer + Nginx), cross-replica idempotent
- **Distributed cron scheduling** — gocron + Redis distributed lock for multi-pod scheduled task execution, PG poll-sync, idempotent dedup, SECURITY DEFINER cross-tenant cleanup, automatic result delivery to source platform

### Admin API

| Endpoint | Description |
|----------|-------------|
| `GET /admin/v1/bootstrap/status` | Bootstrap status (public) |
| `POST /admin/v1/bootstrap` | Initialize platform (ACP Token auth) |
| `GET/POST /admin/v1/tenants/{id}/sandbox-policy` | Sandbox policy CRUD |
| `DELETE /admin/v1/tenants/{id}/sandbox-policy` | Delete sandbox policy |
| `GET/POST /admin/v1/tenants/{id}/api-keys` | Tenant API Key management |
| `POST /admin/v1/tenants/{id}/api-keys/{kid}/rotate` | Rotate API Key |
| `DELETE /admin/v1/tenants/{id}/api-keys/{kid}` | Revoke API Key |
| `GET /admin/v1/pricing-rules` | Query pricing rules |
| `PUT/DELETE /admin/v1/pricing-rules/{model}` | Update/delete model pricing |
| `GET /admin/v1/audit-logs` | Cross-tenant audit logs |
| `GET /admin/v1/usage/tenants` | Tenant usage summary |
| `GET /admin/v1/usage` | Per-tenant usage aggregation (daily/monthly granularity, time range filter) |

### Tenant API (v1)

| Endpoint | Description |
|----------|-------------|
| `POST /v1/chat/completions` | OpenAI-compatible chat endpoint |
| `POST /v1/agent/chat` | Native agent streaming chat |
| `GET/POST/DELETE /v1/tenants` | Tenant management |
| `GET/POST/DELETE /v1/api-keys` | API Key self-service |
| `GET /v1/audit-logs` | Current tenant audit logs |
| `GET/DELETE /v1/execution-receipts/{id}` | Tool execution receipts |
| `GET /v1/usage` | Current tenant usage |
| `GET /v1/me` | Current identity info |
| `GET/DELETE /v1/memories/{id}` | Memory management |
| `GET /v1/sessions/{id}` | Session history |
| `GET/POST/PUT/DELETE /v1/skills/{id}` | Skill CRUD |
| `GET /v1/gdpr/export` | Data export (GDPR) |
| `DELETE /v1/gdpr/data` | Data deletion (GDPR) |
| `POST /v1/gdpr/cleanup-minio` | Clean up object storage |
| `GET/POST/PUT /v1/workflow-definitions` | Workflow definition management |
| `POST /v1/workflow-definitions/{id}/publish` | Publish workflow version |
| `POST/GET /v1/workflow-runs` | Start/query workflow instances |
| `GET /v1/workflow-runs/{id}` | Workflow instance details |
| `POST /v1/workflow-runs/{id}/cancel` | Cancel workflow instance |
| `POST /v1/workflow-runs/{id}/retry` | Retry paused instance |
| `GET /v1/workflow-tasks` | List pending human tasks |
| `POST /v1/workflow-tasks/{id}/complete` | Complete a human task |
| `GET /v1/openapi` | OpenAPI specification |
| `GET /health/live` / `GET /health/ready` | Health checks |
| `GET /metrics` | Prometheus metrics |

---

### Agent Runtime

#### Tools (52)

**Browser Automation (11)**

`browser_navigate` · `browser_snapshot` · `browser_click` · `browser_type` · `browser_scroll` · `browser_back` · `browser_press` · `browser_get_images` · `browser_vision` · `browser_console` · `browser_close`

Supports local Playwright and Browserbase cloud backends, with visual perception (screenshot + GPT-4V analysis).

**File Operations (5)**

`read_file` · `write_file` · `patch` · `search_files` · `file_state`

`file_state` supports snapshot and diff tracking; `patch` supports unified diff format.

**Terminal & Process (2)**

`terminal` · `process`

Full-platform PTY support (Unix/macOS/Windows), with dangerous command detection and automatic interception.

**Web (3)**

`web_search` · `web_extract` · `web_crawl`

URL safety detection (`url_safety`) prevents SSRF and malicious redirects.

**Vision & Media (3)**

`vision_analyze` · `image_generate` · `text_to_speech`

**Memory & Context (3)**

`memory` · `session_search` · `todo`

`memory` includes a Curator for automatic deduplication (O(n) exact dedup + content similarity scan).

**Skill Management (3)**

`skills_list` · `skill_view` · `skill_manage`

**Agent Collaboration (3)**

`delegate_task` · `mixture_of_agents` · `clarify`

`delegate_task` runs up to 8 concurrent sub-agent goroutines; `mixture_of_agents` supports multi-model ensemble voting.

**Code Execution (1)**

`execute_code` — sandboxed execution (Python/Bash) supporting `local` / `docker` / `k8s-job` backends (via `SANDBOX_MODE`), with resource limits, environment variable stripping, and output truncation. K8s Job mode requires no privileged containers and is compatible with GKE Autopilot / EKS Fargate.

**Platform Messaging (3)**

`send_message` · `discord_send` · `discord_search`

**Smart Home (4)**

`ha_list_entities` · `ha_get_state` · `ha_list_services` · `ha_call_service`

**Scheduled Tasks (1)**

`cronjob` — supports cron expressions, persistent scheduling, multi-tenant isolation.

**RL Training (10, extended)**

`rl_list_environments` · `rl_select_environment` · `rl_get_current_config` · `rl_edit_config` · `rl_start_training` · `rl_check_status` · `rl_stop_training` · `rl_get_results` · `rl_list_runs` · `rl_test_inference`

---

#### Platform Adapters (14)

| Platform | Description |
|----------|-------------|
| Telegram | Bot API with file & media support |
| Discord | Bot Gateway with channel search |
| Slack | Socket Mode / Webhook |
| WhatsApp | Cloud API |
| Signal | Signal CLI |
| Email | SMTP/IMAP |
| Matrix | Element protocol |
| Mattermost | REST API |
| DingTalk | DingTalk robot |
| Feishu | Feishu robot |
| WeCom | WeCom application |
| Weixin | WeChat Official Account |
| DMwork | Enterprise IM |
| API Server | HTTP Webhook mode |

#### LLM Providers (7)

| Provider | Description |
|----------|-------------|
| OpenAI | GPT-4o, o3, o4, etc. |
| Anthropic | Claude 4 Opus/Sonnet/Haiku with prompt caching |
| Google / Gemini | Gemini 2.5 Pro/Flash |
| OpenRouter | Unified multi-model routing |
| AWS Bedrock | Native credential chain, no key management |
| Nous Research | Inference API |
| Custom | Any OpenAI-compatible endpoint |

Supports reasoning models (Claude 3.7/Sonnet 4/Opus 4, o1/o3/o4, DeepSeek-r1, QwQ) with automatic model alias resolution (`opus`, `sonnet`, `flash`, `r1`, etc.).

#### Terminal Backends (8)

`local` (native PTY) · `docker` (container isolation) · `ssh` (remote machines) · `modal` (Modal cloud) · `daytona` (Daytona dev environments) · `singularity` (HPC containers) · `persistent_shell` (persistent shell sessions) · PTY Unix/Windows

#### LLM Resilience

- **FallbackRouter** — automatic failover to backup provider when primary LLM fails
- **RetryTransport** — exponential backoff retry with configurable attempts and delays
- **CircuitBreaker** — per-model independent circuit breaking to prevent cascading failures

---

### Infrastructure

- **Single binary** — zero runtime dependencies, cross-compile to any OS/arch with `CGO_ENABLED=0`
- **Multi-replica ready** — verified 3-replica + Nginx `ip_hash` load balancer, idempotent bootstrap
- **Kubernetes ready** — Helm chart with PDB, HPA, conservative scale-down
- **Backup & recovery** — PostgreSQL pgBackRest PITR (RPO < 5min, RTO < 1h) + Redis BGSAVE + S3 (RPO < 15min) + MinIO mc mirror (RPO < 1h), with automated backup scripts and disaster recovery verification (`scripts/dr-test.sh`)
- **CI/CD** — GitHub Actions (unit + integration + race detection + coverage + Docker push)
- **Observability** — Prometheus 11+ custom metrics, OpenTelemetry full-chain tracing (HTTP → middleware → storage → LLM), structured JSON logging (slog), pre-built Grafana dashboard (HTTP/LLM/circuit breaker/rate limits), Prometheus alert rules (5), OTel Collector config, one-click deploy via `docker-compose.observability.yml`

---

## Agent Intelligence

### Oris Evolution System

Behavioral gene (Gene) learning and replay mechanism, providing an independent evolution path beyond the standard SelfImprover.

**How It Works:**

1. **Task Classification** — `DetectTaskClass` automatically infers task type (debug, feature, analysis, writing, general, etc.) from the first user message and tool call history
2. **Strategy Distillation** — after conversation completion, an auxiliary LLM asynchronously distills successful behavioral patterns into a single gene (with confidence score)
3. **Pre-Turn Augmentation** — before the next similar task begins, high-confidence strategy summaries are automatically injected, guiding the agent to reuse validated experience
4. **Secure Isolation** — genes are stored with SHA-256-derived IDs from `tenantID + taskClass + insight`, strictly isolated between tenants (preventing B2 cross-tenant contamination), sanitized for prompt injection before injection (preventing B1 injection attacks)

**Storage backends:** SQLite (local single-node) or MySQL (multi-node), switchable via configuration.

**Configuration:**

```yaml
# ~/.hermes/config.yaml
evolution:
  enabled: true
  store_dsn: ""          # empty = SQLite default path
  min_confidence: 0.7    # genes below this threshold are not replayed
  max_genes_per_turn: 3  # max strategies injected per turn
```

---

### Batch Trajectory Generation

Run multiple prompts in parallel to batch-collect agent trajectory data for evaluation, fine-tuning, or RL dataset construction.

**Features:**

- Goroutine pool control (default 4 concurrent workers, configurable)
- Each prompt runs a full independent agent loop (including tool calls)
- Trajectories automatically persisted to `~/.hermes/batch_output` (JSON Lines format)
- Summary report generated (success rate, average turns, total token consumption)

**Configuration:**

```go
BatchConfig{
    Prompts:       []string{"task1", "task2", ...},
    Model:         "sonnet",
    MaxWorkers:    8,
    MaxIterations: 30,
    OutputDir:     "./trajectories",
    ToolSets:      []string{"file", "terminal"},
}
```

---

## Developer Tools

### ACP Server (Agent Communication Protocol)

A local agent integration protocol for editors, enabling VS Code, Zed, JetBrains and other IDE plugins to interact with the agent via HTTP.

**API Endpoints:**

| Endpoint | Description |
|----------|-------------|
| `GET /v1/health` | Service health check (public) |
| `POST /v1/chat` | Start agent conversation |
| `GET /v1/status` | Agent runtime status |
| `POST /v1/tool` | Direct tool invocation |
| `GET /v1/tools` | List available tools |
| `POST/GET /v1/sessions` | Session creation and listing |
| `GET/DELETE /v1/sessions/{id}` | Session query and deletion |
| `GET /v1/events?session_id=X` | SSE real-time event stream |

**SSE Event Push:** EventBroker maintains an independent subscription channel per session (buffered 32), pushing agent thinking, tool calls, and response events in real time.

**Auth:** Bearer Token (configured via `ACP_TOKEN` env var; empty = development mode, no auth).

---

### Dashboard

Embedded web management UI providing visual management for sessions, configuration, skills, and gateways.

**Features:**

- Session list and message history viewer
- Runtime configuration viewer
- Skill list and management
- Gateway connection status

**Deployment:** Dashboard starts as an optional standalone module with configurable port; SPA static assets are embedded in the binary.

**Auth:** Write operations require Bearer Token; empty value enters development no-auth mode.

---

## Fixed SOP Workflow Engine

Orchestrate agent tasks, HTTP service calls, and conditional branches into persistent, auditable DAG workflows with human-in-the-loop support and pause/retry semantics.

> Full usage guide: [Workflow Engine Guide](workflow-guide.en.md)

### Node Types

| Type | Description |
|------|-------------|
| `start` | Flow entry point, completes automatically |
| `agent_task` | Invokes a full agent loop (Eino ReAct Graph + safety pipeline) via `config.prompt` |
| `service_task` | HTTP call to external services with custom Method/Header/Body; JSON response auto-merged into step output |
| `human_task` | Pauses the flow awaiting human action; resume via `POST /v1/workflow-tasks/{id}/complete` with outcome + variable updates |
| `end` | Flow exit point, completes automatically |

### Graph JSON Example

```json
{
  "nodes": [
    {"id":"start","type":"start","name":"Start"},
    {"id":"ai_check","type":"agent_task","name":"AI Review",
     "config":{"prompt":"Review compliance: {{input.description}}"}},
    {"id":"approve","type":"human_task","name":"Manager Approval",
     "config":{"assignee":"manager"}},
    {"id":"notify","type":"service_task","name":"Send Notification",
     "config":{"url":"https://api.example.com/notify","method":"POST"}},
    {"id":"end_ok","type":"end","name":"Approved"},
    {"id":"end_reject","type":"end","name":"Rejected"}
  ],
  "edges": [
    {"from":"start","to":"ai_check"},
    {"from":"ai_check","to":"approve"},
    {"from":"approve","to":"notify","condition":{"outcome":"approved"}},
    {"from":"approve","to":"end_reject","condition":{"outcome":"rejected"}},
    {"from":"notify","to":"end_ok"}
  ]
}
```

### Conditional Routing

Edges support two-level filtering:

- **Outcome matching** — matches the upstream step's `outcome` field (e.g., `approved` / `rejected`)
- **Condition expressions** — supports `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `exists`, `contains`, `startsWith`, `endsWith`

Path syntax: `input.field`, `variables.key`, `steps.{nodeID}.output.field` (supports nested dot-notation access)

### Instance Lifecycle

```
running → waiting (awaiting human) → running → completed
                                   → paused (step failed, retryable)
                                   → cancelled
```

### Graph Validation Rules

| Rule | Description |
|------|-------------|
| Exactly 1 start | No multiple or missing entry points |
| At least 1 end | Must have a termination point |
| No self-loops, acyclic (DAG) | Topological sort detection ensures flow can terminate |
| Fully connected | All nodes must be reachable from start |

### Version Control

- Definitions support `draft` → `published` → `archived` state transitions
- Each publish snapshots GraphJSON; running instances are pinned to the version at launch time and never drift

### Agent Safety Execution (EinoAgentExecutor)

`agent_task` nodes execute through `EinoAgentExecutor` with mandatory safety pipeline:

- **Input interception** — prompt injection detection
- **Credential redaction** — 20+ credential patterns auto-masked (AWS Key, GitHub Token, etc.)
- **Iteration limit** — hard cap of 50 tool loop iterations
- **Stream redaction** — chunk-level buffering, intermediate chunks never leak raw credentials

### Storage Backends

PostgreSQL and MySQL dual implementation, tables: `workflow_definitions`, `workflow_versions`, `workflow_runs`, `workflow_step_runs`

### API Quick Example

```bash
# 1. Create definition
POST /v1/workflow-definitions
{"name":"Approval Flow","graph":{"nodes":[...],"edges":[...]}}

# 2. Publish
POST /v1/workflow-definitions/{id}/publish

# 3. Start instance
POST /v1/workflow-runs
{"definition_id":"{id}","input":{"amount":10000}}

# 4. List pending human tasks
GET /v1/workflow-tasks

# 5. Complete human task
POST /v1/workflow-tasks/{stepRunID}/complete
{"outcome":"approved","output":{"comment":"Approved"},"variables":{"approved":true}}

# 6. Retry failed step
POST /v1/workflow-runs/{id}/retry

# 7. Cancel instance
POST /v1/workflow-runs/{id}/cancel
```

---

## Security Enhancements

In addition to platform-level RBAC and RLS, the tool layer has multiple security detection mechanisms:

| Mechanism | File | Description |
|-----------|------|-------------|
| **Credential Guard** | `credential_guard.go` | Detects API keys, passwords, tokens and other sensitive patterns in tool inputs/outputs, blocking credential leakage |
| **Git Security** | `git_security.go` | Intercepts dangerous git commands (force push, history rewrite, etc.) |
| **URL Safety** | `url_safety.go` | SSRF protection, detects internal addresses and malicious redirects |
| **OSV Vulnerability Scan** | `osv_check.go` | Calls Google OSV API to detect known vulnerabilities in dependencies |
| **Patch Parser** | `patch_parser.go` | Structural validation of diff content for the patch tool |
| **Approval Flow** | `approval.go` | Human confirmation hook for dangerous operations, can integrate with external approval systems |
| **Prompt Sanitization** | `sanitize.go` (evolution) | Cleans prompt injection patterns before evolution gene injection |

---

## v2.3 New Capabilities (7 Architecture Enhancements)

### 1. Extended Thinking API

Deep reasoning for Claude models. Wires `ReasoningConfig` into Anthropic requests with configurable thinking budgets across 5 effort levels:

| Level | Token Budget | Use Case |
|-------|-------------|----------|
| `minimal` | 1,024 | Simple classification, format conversion |
| `low` | 2,048 | Standard Q&A, light reasoning |
| `medium` | 4,096 | Multi-step reasoning, code generation (default) |
| `high` | 10,000 | Complex architecture design, long-chain deduction |
| `xhigh` | 32,000 | Extreme complexity, research-grade reasoning |

When enabled, `max_tokens` is automatically adjusted to `budget_tokens + output_tokens` to prevent thinking and output from competing for space.

```yaml
reasoning: high   # .hermes/config.yaml
```

---

### 2. Model Aliases

Short, human-friendly names that resolve to fully-qualified model identifiers:

| Alias | Resolves To |
|-------|-------------|
| `opus` | `anthropic/claude-opus-4-20250514` |
| `sonnet` | `anthropic/claude-sonnet-4-20250514` |
| `haiku` | `anthropic/claude-haiku-4-20250414` |
| `gpt4o` | `openai/gpt-4o` |
| `o3` | `openai/o3` |
| `flash` | `google/gemini-2.5-flash` |
| `gemini` | `google/gemini-2.5-pro` |
| `r1` | `deepseek/deepseek-r1` |
| `llama` | `meta-llama/llama-4-maverick` |

Resolution is case-insensitive with automatic whitespace trimming. Unrecognized names pass through unchanged for custom endpoint compatibility.

---

### 3. Project-Scoped Config

Auto-discovers `.hermes/config.yaml` at the project root (git root or directory containing `.hermes/`):

**Security by Design:**

- Only safe fields can be overridden: `model`, `max_iterations`, `max_tokens`, `reasoning`, `toolsets`, `plugins`, `cache`, etc.
- Sensitive fields are automatically sanitized: `api_key`, `database`, `redis`, `objstore`, `provider`, `base_url` are cleared
- Project configs are safe to commit to version control — no credential leakage possible

**Priority (lowest to highest):**

```
Global defaults → ~/.hermes/config.yaml → {project}/.hermes/config.yaml → Env vars → CLI flags
```

---

### 4. Declarative Permission Policies

YAML-based tool access control with `allow` / `deny` / `ask` actions, integrated into the `CheckDangerousCommand` flow:

```yaml
# .hermes/permissions.yaml
default: ask
rules:
  - tool: terminal
    action: deny
    commands: ["rm -rf *", "DROP TABLE*"]
    reason: "Block destructive commands"
  - tool: file_write
    action: allow
    paths: ["src/**", "tests/**"]
  - tool: browser
    action: ask
    reason: "Browser actions require human confirmation"
```

**Layered Loading:**

1. User-level: `~/.hermes/permissions.yaml` (global baseline)
2. Project-level: `{project}/.hermes/permissions.yaml` (overrides user-level)

---

### 5. Structured Compaction

Preserves a **Tool Spine** — a structured summary of tool call outcomes — through context compression in long conversations:

```
### Tool Call History
1. terminal [ok]: go test ./... passed (127 tests)
2. file_write [ok]: success
3. grep [ok]: 15 results
4. terminal [FAIL]: exit code 1; package not found
```

Ensures the agent can trace "what was tried and what happened" even after heavy compression — preventing repeated failed operations or forgotten completed steps.

---

### 6. OAuth Device Flow (RFC 8628)

Standards-compliant OAuth 2.0 Device Authorization Grant for browser-based Anthropic login in headless environments:

```
1. CLI requests device code → Anthropic returns user_code + verification_uri
2. User opens URL in browser and enters verification code
3. CLI polls token endpoint (5s interval)
4. Receives access_token + refresh_token → persisted to ~/.hermes/anthropic.json (permissions 0600)
5. Auto-refreshes 30s before expiry
```

---

### 7. MCP Auto-Reconnect

Production-grade reliability for SSE-based MCP server connections:

| Parameter | Value |
|-----------|-------|
| Heartbeat ping interval | 30 seconds |
| Initial reconnect delay | 1 second |
| Backoff factor | 2.0x |
| Max delay | 30 seconds |
| Max retries | 10 |
| Jitter range | ±25% |

After reconnection, automatically re-executes `tools/list` to refresh tool definitions. Prometheus metric `mcp_server_reconnects_total` tracks reconnections per server.

---

## Installation

```bash
# From source (requires Go 1.23+)
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx
go build -o hermesx ./cmd/hermesx/

# Global install
sudo cp hermesx /usr/local/bin/
```

### CLI Mode

```bash
./hermesx setup   # configuration wizard
./hermesx         # interactive CLI
./hermesx chat "What tools do you have?"
```

### SaaS Mode

```bash
docker compose -f docker-compose.prod.yml up -d
./examples/enterprise-saas-demo/demo.sh   # 11-step enterprise demo
```

See the [SaaS Quickstart](saas-quickstart.md) for a full deployment walkthrough.
