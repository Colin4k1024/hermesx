# HermesX

**Enterprise Agent Runtime & Multi-Tenant SaaS Control Plane**

A production-grade platform for deploying, isolating, and governing AI agents at enterprise scale. Built in Go for single-binary deployment, native concurrency, and zero-dependency distribution.

---

## Quick Links

| | |
|---|---|
| [SaaS Quickstart](saas-quickstart.md) | Get a tenant up and running in minutes |
| [API Reference](api-reference.md) | Full endpoint documentation |
| [Architecture Overview](architecture.md) | System design and component map |
| [Configuration](configuration.md) | All environment variables and config options |
| [Deployment](deployment.md) | Docker, Kubernetes, and bare-metal guides |

---

## Project Stats

| Metric | Value |
|--------|-------|
| Go source files | 413 |
| Lines of code | 78,000+ |
| Registered tools | 50 (36 core + 14 extended) |
| Platform adapters | 15 |
| Terminal backends | 7 |
| Bundled skills | 126 |
| Test files | 127 |
| Total tests | 1,597 |
| RLS-protected tables | 10 |
| API endpoints | 33+ |
| Version | v2.2.0 |

---

## Core Capabilities

### Enterprise SaaS Platform

- **Multi-tenant isolation** — PostgreSQL Row-Level Security with per-transaction tenant context
- **Auth chain** — Static Token → API Key (SHA-256 hashed) → JWT/OIDC
- **5 roles** — `super_admin`, `admin`, `owner`, `user`, `auditor`
- **Dual-layer rate limiting** — atomic Redis Lua script with local LRU fallback
- **Audit trail** — immutable logs for all state-changing operations
- **GDPR compliance** — full-chain tenant data export + transactional deletion
- **Sandbox isolation** — per-tenant code execution with Docker network/resource limits

### Agent Runtime

- **50 tools** — terminal, file ops, web search/crawl, browser, vision, image gen, TTS, code exec, subagent, MCP, and more
- **15 platform adapters** — Telegram, Discord, Slack, WhatsApp, Signal, Email, Matrix, and more
- **Dual API support** — OpenAI-compatible + Anthropic Messages API (with prompt caching)
- **LLM resilience** — FallbackRouter + RetryTransport + Circuit Breaker (per-model)
- **Skill system** — procedural memory with YAML/Markdown files and hub search/install

### Infrastructure

- **Single binary** — zero runtime dependencies, cross-compile to any OS/arch
- **Multi-replica ready** — verified 3-replica + Nginx `ip_hash` load balancer
- **Kubernetes ready** — Helm chart with PDB, HPA, conservative scale-down
- **Observability** — Prometheus metrics, OpenTelemetry tracing, structured JSON logging

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

**Configuration:**

```yaml
# .hermes/config.yaml
reasoning: high
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

**Usage:**

```yaml
model: opus          # equivalent to anthropic/claude-opus-4-20250514
```

---

### 3. Project-Scoped Config

Auto-discovers `.hermes/config.yaml` at the project root (git root or directory containing `.hermes/`), enabling per-project behavior customization.

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
    reason: "Allow writes to source and test directories"
  - tool: browser
    action: ask
    reason: "Browser actions require human confirmation"
```

**Layered Loading:**

1. User-level: `~/.hermes/permissions.yaml` (global baseline)
2. Project-level: `{project}/.hermes/permissions.yaml` (overrides user-level)

Supports glob pattern matching for paths and commands. The `*` wildcard matches all tools.

---

### 5. Structured Compaction

Preserves a **Tool Spine** — a structured summary of tool call outcomes — through context compression in long conversations:

**How It Works:**

1. Extracts all tool call results from messages being compressed
2. Generates a triple for each call: `(tool_name, success/failure, one-line key result)`
3. Appends the Tool Spine to the compression summary

**Output Format:**

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

**Flow:**

```
1. CLI requests device code → Anthropic returns user_code + verification_uri
2. User opens URL in browser and enters verification code
3. CLI polls token endpoint (5s interval)
4. Receives access_token + refresh_token → persisted to ~/.hermes/anthropic.json
5. Auto-refreshes 30s before expiry
```

**Features:**

- Secure token persistence (file permissions `0600`)
- Automatic refresh — no repeated logins
- `ResolveAnthropicAPIKey` helper: prefers env var → falls back to OAuth token
- Custom OAuth endpoint support (for private deployments)

---

### 7. MCP Auto-Reconnect

Production-grade reliability for SSE-based MCP server connections:

**Health Monitoring:**

- Sends JSON-RPC `ping` every 30 seconds to proactively detect connection health
- Watches for SSE stream close events to detect disconnections immediately

**Reconnection Strategy (Exponential Backoff + Jitter):**

| Parameter | Value |
|-----------|-------|
| Initial delay | 1 second |
| Backoff factor | 2.0x |
| Max delay | 30 seconds |
| Max retries | 10 |
| Jitter range | ±25% |

**Post-Reconnect Recovery:**

- Automatically re-executes `tools/list` to refresh tool definitions
- Deregisters old tools → registers new ones, keeping the Registry consistent with the server
- Prometheus metric `mcp_server_reconnects_total` tracks reconnections per server

**Fault Tolerance:**

- Individual tool call failures trigger immediate reconnect + retry
- `tools/list_changed` notifications trigger hot tool list refresh
- When connection is permanently lost, placeholder tools return friendly error guidance

---

## Installation

```bash
# From source (requires Go 1.23+)
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx
go build -o hermesx ./cmd/hermesx/
```

See the [SaaS Quickstart](saas-quickstart.md) for a full deployment walkthrough.
