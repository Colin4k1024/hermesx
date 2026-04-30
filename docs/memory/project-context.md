# Project Context: hermes-agent-go
| 字段 | 值 |
|------|-----|
| 项目名 | hermes-agent-go |
| 当前任务 | `2026-04-30-hermes-webui` |
| 阶段 | `handoff-ready` |
| 更新时间 | 2026-04-30 |

---

## Tech Stack

### 后端
- Go 1.25.0
- PostgreSQL (primary store, pgx/v5, migration v27)
- SQLite (local dev store, noop for SaaS features)
- Redis (session lock, rate limit fallback)
- MinIO (per-tenant skills + soul storage)
- OpenTelemetry (otel SDK + OTLP gRPC exporter)
- Prometheus (client_golang v1.23.2)
- hashicorp/golang-lru/v2 v2.0.7 (rate limiter)
- golang-jwt/jwt/v5 (JWT RS256)
- Helm chart (Kubernetes)
- Docker Compose (local dev)

### 前端 (新增，webui/ 子目录)
- Vue 3 + TypeScript + Vite
- Naive UI (组件库)
- Pinia (状态管理)
- Vue Router (hash mode)
- Nginx (生产静态服务 + API 反代)

## 当前任务

**hermes-webui** — 为 hermes-agent-go 构建可独立部署的 Vue 3 SPA WebUI。

- 支持多租户多用户接入（API Key + User ID 认证）
- 聊天、记忆管理、技能管理、Admin（租户/API Key 管理）
- 部署：`webui/` monorepo 子目录，Docker 镜像，`HERMES_BACKEND_URL` env

**Plan 产出物**:
- `docs/artifacts/2026-04-30-hermes-webui/prd.md`
- `docs/artifacts/2026-04-30-hermes-webui/arch-design.md`
- `docs/artifacts/2026-04-30-hermes-webui/delivery-plan.md`
- `docs/artifacts/2026-04-30-hermes-webui/ui-implementation-plan.md`

**后端前置条件已应用**:
- `internal/api/server.go:181`: WriteTimeout 60s → 150s（修复 LLM 超时竞争）

## 依赖

- 现有后端 REST API 全量可用（无新接口）
- `CORS_ORIGINS` env 需配置为允许 WebUI 域名
- `HERMES_BACKEND_URL` env 指向 Go 后端（生产 Docker 部署）
- Playwright api-isolation 13/13 必须继续通过（后端零破坏）

## 风险

- R1: API Key 浏览器暴露 — 已接受，内部工具场景，sessionStorage + Key 可轮换
- R2: 无 SSE 流式输出 — 已接受，v1 loading spinner，SSE 进 backlog
- R3: X-Hermes-User-Id 自声明 — 已接受，租户隔离由 API Key 强制
- R4: /v1/audit-logs 返回 500 — 各 tab 独立，不阻塞其他 Admin 功能

## 下一步

1. **frontend-engineer 开始实现**: 从 Sprint S0 (`webui/` 脚手架) 开始
2. S0 完成后 S1/S2/S3 可并行
3. S4 收口: UI 评审 + Docker 集成 + Playwright 回归
