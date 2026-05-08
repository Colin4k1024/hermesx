# HermesX

[English](#english) | [中文](#中文)

---

<a id="english"></a>

## English

**HermesX** — Enterprise Agent Runtime & Multi-Tenant SaaS Control Plane.

A production-grade platform for deploying, isolating, and governing AI agents at enterprise scale. Built in Go for single-binary deployment, native concurrency, and zero-dependency distribution.

> Originally inspired by [hermes-agent](https://github.com/NousResearch/hermes-agent) by Nous Research. HermesX has since evolved into an independent enterprise platform with multi-tenant isolation, RBAC, audit trails, sandbox execution, and SaaS-grade observability — capabilities that go far beyond the original agent framework.

### Project Stats

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
| Version | v2.0.0 |

### Core Capabilities

#### Enterprise SaaS Platform

- **Multi-tenant isolation**: PostgreSQL Row-Level Security (RLS) with `SET LOCAL app.current_tenant` per transaction
- **Auth chain**: Static Token → API Key (SHA-256 hashed) → JWT/OIDC
- **5 roles**: `super_admin`, `admin`, `owner`, `user`, `auditor`
- **API Key scopes**: fine-grained `read`/`write`/`execute`/`admin`/`audit`/`gdpr` authorization
- **Dual-layer rate limiting**: atomic Redis Lua script (tenant + user sliding window) with local LRU fallback
- **Token usage metering**: async batch persistence with per-model cost calculation
- **Execution receipts**: auditable tool invocation with idempotency dedup and trace correlation
- **Audit trail**: immutable logs for all state-changing operations
- **GDPR compliance**: full-chain tenant data export + transactional deletion
- **Sandbox isolation**: per-tenant code execution with Docker network/resource limits
- **Admin API**: tenant management, sandbox policy CRUD, API key lifecycle, pricing rules

#### Observability

- **Prometheus metrics**: 11+ custom business metrics (HTTP, LLM, tools, rate limiting, sessions)
- **OpenTelemetry tracing**: HTTP → middleware → store → LLM full request chain
- **PGX tracer**: database query spans with parameter capture
- **Structured logging**: JSON via `slog` with tenant/request context

#### Agent Runtime

- **50 tools**: terminal, file ops, web search/crawl, browser, vision, image gen, TTS, code exec, subagent, session search, memory, todo, cron, MCP, and more
- **15 platform adapters**: Telegram, Discord, Slack, WhatsApp, Signal, Email, Matrix, Mattermost, DingTalk, Feishu, WeCom, SMS, Home Assistant, Webhook, API Server
- **7 terminal backends**: local, Docker, SSH, Modal, Daytona, Singularity, persistent shell
- **Dual API support**: OpenAI-compatible + Anthropic Messages API (with prompt caching)
- **LLM resilience**: FallbackRouter (primary→fallback switching) + RetryTransport (exponential backoff) + Circuit Breaker (per-model)
- **Skill system**: procedural memory with YAML/Markdown files, hub search/install, security scanning
- **Context compression**: automatic summarization when approaching token limits
- **Subagent delegation**: parallel task execution via goroutines (max 8 concurrent)
- **MCP integration**: Model Context Protocol client (stdio + SSE transport)

#### Infrastructure

- **Single binary**: zero runtime dependencies, cross-compile to any OS/arch
- **Multi-replica ready**: verified 3-replica + Nginx ip_hash load balancer
- **PG PITR backup**: pgBackRest with RPO < 5min, RTO < 1h
- **CI/CD**: GitHub Actions (unit + integration + race + coverage + Docker push)
- **Kubernetes ready**: Helm chart with PDB, HPA, conservative scale-down

### Installation

#### From Source

Requirements: Go 1.23+

```bash
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx
go build -o hermesx ./cmd/hermesx/

# Install globally
sudo cp hermesx /usr/local/bin/
```

#### Using Make

```bash
make build      # Build binary
make install    # Install to ~/.local/bin/
```

#### Docker

```bash
docker build -t hermesx .
docker run -it --rm \
  -v ~/.hermes:/home/hermes/.hermes \
  hermesx
```

### Quick Start

#### CLI Mode (Single Agent)

```bash
# Setup wizard
./hermesx setup

# Interactive CLI
./hermesx

# Single query
./hermesx chat "What tools do you have?"
```

#### SaaS Mode (Multi-Tenant)

```bash
# Start full stack
docker compose -f docker-compose.prod.yml up -d

# Run enterprise demo (11 steps)
./examples/enterprise-saas-demo/demo.sh
```

### Architecture

```
hermesx/
├── cmd/hermesx/             Entry point (Cobra CLI + SaaS server)
├── internal/
│   ├── agent/               Core agent loop, streaming, memory curator
│   ├── api/                 REST API server + handlers
│   │   └── admin/           Admin API (sandbox, keys, audit, pricing)
│   ├── auth/                Auth chain (API key, JWT, scopes, RBAC)
│   ├── cli/                 Interactive TUI, commands, setup wizard
│   ├── gateway/             Multi-platform messaging gateway
│   │   └── platforms/       15 platform adapters
│   ├── llm/                 LLM client, FallbackRouter, RetryTransport, CircuitBreaker
│   ├── metering/            Token usage recording, batch flush, cost calc
│   ├── middleware/          Rate limit, scope check, tenant injection, tracing
│   ├── observability/       OTel tracing, Prometheus metrics
│   ├── skills/              Skill loading, parsing, hub, MinIO sync
│   ├── store/               PostgreSQL store (RLS, 80+ migrations)
│   │   ├── pg/              PG implementations (sessions, memories, keys, etc.)
│   │   └── rediscache/      Redis (rate limit, sessions, context cache)
│   ├── tools/               50 tool implementations + sandbox
│   │   └── environments/    7 terminal backends + Docker sandbox
│   └── ...
├── deploy/                  Multi-replica, OTel collector, PITR
├── tests/integration/       Go integration tests (tenant/session/RLS)
├── examples/                Enterprise SaaS demo
├── scripts/                 Backup, restore, verification
├── skills/                  126 bundled skills
└── docs/                    Security model, RBAC matrix, deployment guide
```

### Deployment

#### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `REDIS_URL` | Yes | Redis connection URL |
| `LLM_API_KEY` | Yes | Primary LLM provider API key |
| `LLM_FALLBACK_API_KEY` | No | Fallback LLM provider API key |
| `MINIO_ENDPOINT` | No | S3-compatible storage for skills |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | No | OpenTelemetry collector |
| `HERMES_ADMIN_TOKEN` | Yes | Platform admin static token |

#### Infrastructure Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| PostgreSQL | 14+ | 16 (RLS support) |
| Redis | 6+ | 7 (Lua script) |
| Go | 1.23+ | 1.25 |

### Testing

```bash
# Unit tests
go test ./...
make test

# Integration tests (requires Docker)
make test-integration

# Race detection
go test -race ./internal/agent/... ./internal/tools/... ./internal/gateway/...
```

### Documentation

| Document | Description |
|----------|-------------|
| [SECURITY_MODEL.md](docs/SECURITY_MODEL.md) | Threat model, auth chain, RLS, sandbox |
| [RBAC_MATRIX.md](docs/RBAC_MATRIX.md) | 5 roles × 10 resources permission matrix |
| [ENTERPRISE_READINESS.md](docs/ENTERPRISE_READINESS.md) | 12 capabilities with evidence |
| [deployment.md](docs/deployment.md) | HA, scaling, backup, alerting |

### Acknowledgements

HermesX was originally forked from [hermes-agent](https://github.com/NousResearch/hermes-agent) by [Nous Research](https://nousresearch.com). We are grateful for their foundational work on the self-improving AI agent framework. HermesX has since diverged significantly to serve enterprise multi-tenant SaaS use cases.

### License

MIT

---

<a id="中文"></a>

## 中文

**HermesX** — 企业级 Agent 运行时 & 多租户 SaaS 控制平面。

面向企业规模的 AI Agent 部署、隔离和治理的生产级平台。使用 Go 构建，单二进制部署、原生并发、零依赖分发。

> 最初受 Nous Research 的 [hermes-agent](https://github.com/NousResearch/hermes-agent) 启发。HermesX 已演进为独立的企业平台，具备多租户隔离、RBAC、审计追踪、沙箱执行和 SaaS 级可观测性 — 远超原始 Agent 框架的能力边界。

### 项目数据

| 指标 | 数值 |
|------|------|
| Go 源文件 | 413 个 |
| 代码行数 | 78,000+ 行 |
| 注册工具 | 50 个（36 核心 + 14 扩展） |
| 平台适配器 | 15 个 |
| 终端后端 | 7 个 |
| 内置技能 | 126 个 |
| 测试文件 | 123 个 |
| 测试总数 | 1,585 个 |
| RLS 保护表 | 10 个 |
| API 端点 | 22+ 个 |
| 版本 | v2.0.0 |

### 核心能力

#### 企业 SaaS 平台

- **多租户隔离**：PostgreSQL 行级安全（RLS），每事务 `SET LOCAL app.current_tenant`
- **认证链**：静态 Token → API Key（SHA-256 哈希）→ JWT/OIDC
- **5 种角色**：`super_admin`、`admin`、`owner`、`user`、`auditor`
- **API Key 作用域**：`read`/`write`/`execute`/`admin`/`audit`/`gdpr` 细粒度授权
- **双层限流**：原子 Redis Lua 脚本（租户 + 用户滑动窗口），Redis 故障自动降级本地 LRU
- **Token 用量计量**：异步批量持久化 + 按模型成本计算
- **执行回执**：可审计的工具调用，含幂等去重和链路追踪关联
- **审计追踪**：所有状态变更操作的不可变日志
- **GDPR 合规**：全链路数据导出 + 事务性删除
- **沙箱隔离**：按租户的代码执行环境，Docker 网络/资源限制
- **Admin API**：租户管理、沙箱策略、密钥生命周期、定价规则

#### 可观测性

- **Prometheus 指标**：11+ 自定义业务指标（HTTP、LLM、工具、限流、会话）
- **OpenTelemetry 追踪**：HTTP → 中间件 → 存储 → LLM 全链路
- **PGX 追踪器**：数据库查询 Span
- **结构化日志**：`slog` JSON 格式，含租户/请求上下文

#### Agent 运行时

- **50 个工具**：终端、文件、搜索、浏览器、视觉、图像、TTS、代码执行、子 Agent、记忆、MCP 等
- **15 个平台**：Telegram、Discord、Slack、WhatsApp、Signal、邮件、Matrix、钉钉、飞书、企业微信等
- **7 个终端后端**：本地、Docker、SSH、Modal、Daytona、Singularity、持久 Shell
- **LLM 弹性**：FallbackRouter + RetryTransport（指数退避）+ 熔断器（按模型独立）
- **技能系统**：YAML/Markdown 文件 + Hub 搜索安装 + 安全扫描
- **上下文压缩**：接近 Token 上限时自动摘要
- **MCP 集成**：支持 stdio + SSE 传输

### 安装

```bash
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx
go build -o hermesx ./cmd/hermesx/
sudo cp hermesx /usr/local/bin/
```

### 快速开始

#### CLI 模式（单 Agent）

```bash
./hermesx setup    # 配置向导
./hermesx          # 交互式 CLI
./hermesx chat "你有什么工具？"
```

#### SaaS 模式（多租户）

```bash
docker compose -f docker-compose.prod.yml up -d
./examples/enterprise-saas-demo/demo.sh
```

### 部署

| 变量 | 必需 | 说明 |
|------|------|------|
| `DATABASE_URL` | 是 | PostgreSQL 连接字符串 |
| `REDIS_URL` | 是 | Redis 连接地址 |
| `LLM_API_KEY` | 是 | 主 LLM Provider API Key |
| `HERMES_ADMIN_TOKEN` | 是 | 平台管理员静态 Token |
| `MINIO_ENDPOINT` | 否 | S3 兼容存储 |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | 否 | OTel 收集器 |

### 文档

| 文档 | 说明 |
|------|------|
| [SECURITY_MODEL.md](docs/SECURITY_MODEL.md) | 威胁模型、认证链、RLS、沙箱 |
| [RBAC_MATRIX.md](docs/RBAC_MATRIX.md) | 5 角色 × 10 资源权限矩阵 |
| [ENTERPRISE_READINESS.md](docs/ENTERPRISE_READINESS.md) | 12 项能力及证据 |
| [deployment.md](docs/deployment.md) | 高可用、扩缩容、备份、告警 |

### 致谢

HermesX 最初 fork 自 [Nous Research](https://nousresearch.com) 的 [hermes-agent](https://github.com/NousResearch/hermes-agent)。感谢他们在自我进化 AI Agent 框架上的开创性工作。HermesX 已大幅偏离原始项目，专注于企业多租户 SaaS 场景。

### 许可证

MIT
