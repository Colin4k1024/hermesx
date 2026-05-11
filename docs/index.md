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
| Test files | 123 |
| Total tests | 1,585 |
| RLS-protected tables | 10 |
| API endpoints | 22+ |
| Version | v2.1.1 |

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

## Installation

```bash
# From source (requires Go 1.23+)
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx
go build -o hermesx ./cmd/hermesx/
```

See the [SaaS Quickstart](saas-quickstart.md) for a full deployment walkthrough.
