# Project Context: hermesx

**项目名**: hermesx  
**当前任务**: 2026-06-01-hermes-agent-v0152-absorption + trusted-channel-login
**前任务**: 2026-05-27-backlog-batch-1（完成）
**阶段**: executing（v0.15.2 absorption 4 phases 完成 + trusted channel login 已合入 main）
**版本目标**: v2.5.0 — Eino Phase 2 + governance backlog closure + channel auth

## Tech Stack

- Go 1.25 + PostgreSQL 16 (+ MySQL 8.0 optional) + Redis 7 + RustFS (replaces MinIO, S3-compat)
- coreos/go-oidc/v3 (OIDC SSO — JWKS rotation, claim mapping, wired in saas.go)
- sony/gobreaker/v2 (per-model circuit breaker with Prometheus metrics)
- Redis Lua atomic ZSET (DualLayerLimiter)
- prometheus/client_golang (breaker state + request counters)
- Helm v3 (PDB/HPA)
- Docker multi-stage build → ghcr.io (CI auto-push)
- GitHub Actions CI (unit + integration + race + coverage + Docker push)
- **webui**: React 18 + React Router v6 + Ant Design 5 + Zustand + @tanstack/react-query v5 + Tailwind CSS 3 + Vite 6 multi-page

## 当前状态

- v1.2.0 CLOSED (Phase 1 + Phase 2 enterprise SaaS)
- v1.3.0 CLOSED (Phase 3 — OIDC wiring + breaker metrics + CI/CD)
- v1.4.0 CLOSED (v0.12 upstream absorption — hermes-agent v2026.4.30)
- v2.0.0 CLOSED (post-release hardening — commit 5ea9c44; LifecycleHooks + SelfImprover wired, URL sanitize + prompt sanitize fixed)
- v2.1.0 CLOSED (infra upgrade — ObjectStore interface + RustFS, pprof + OTel + Prometheus, MySQL adapter; K8s local deployment validated 2026-05-08)
- hermesx-webui RELEASED (v2.1.0-webui — Admin Console + User Portal; 4 CRITICAL + 4 HIGH 安全修复; Bootstrap 端点; 旧 HTML 下线; webui CI)
- v2.1.1 RELEASED (2026-05-09 — K8s MySQL 全栈部署验证 + 8 Bug 修复; 详见 session 004)
- v2.2.0-stabilization CLOSED (2026-05-14 — Bootstrap IP 限流、跨实例原子初始化、PG API key scopes 修复、会话标题 UX、发布/文档口径同步)
- IronClaw security-enhancement MERGED to main (2026-05-18 — safety/egress/secrets 三包构建完成，5236 行新增，未接入主链路)
- v2.3.0 security-integration CLOSED (2026-05-18 — 全部 9 Story 完成，5 阻塞项(B-1~B-5) + 6 MEDIUM 项修复，26/26 -race 通过，全链路 9 个 artifacts 落盘，6 项遗留入 v2.4.0 backlog)
- Eino ADK POC CLOSED (2026-05-19 — CloudWeGo Eino ReAct agent 验证通过；Adapter Layer + EinoAgent + SafetyPipeline + WorkflowExecutor；1813 tests/46 pkgs 全绿；Phase 2 全量替换准入达成)
- hermes-agent v0.15.2 absorption CLOSED (2026-06-01 — 4 phases: core+provider+gateway+safety&secrets; gofmt+govulncheck 修复)
- Trusted Channel Login MERGED (2026-06-02 — channel provider 管理、challenge 验证、网关身份识别、CSRF 中间件、MySQL/PG channel_auth store、Admin API 扩展；+3270 行/40 文件)

## 已完成

- Phase 1: RLS write protection, audit immutability, GDPR cleanup, PDB/HPA, session ownership, IDOR fix, CORS fix, credential hygiene
- Phase 2: OIDCExtractor (JWKS + ClaimMapper), DualLayerLimiter (Redis Lua + local fallback), Dynamic PricingStore (30s cache + DB fallback), Admin Pricing CRUD API, store.ErrNotFound sentinel
- Phase 3: OIDC wired into auth chain (env var activation), breaker Prometheus metrics + ChatStream failure recording, CI coverage reporting + Docker ghcr.io push, security hardening (tenant claim validation, JWT error propagation, startup timeout, goroutine leak fix)
- v0.12 Absorption Sprint 1: Model Catalog hot-reload, CJK trigram search, Gateway platform registry refactor
- v0.12 Absorption Sprint 2: MultimodalRouter (image/audio/video dispatch with provider capability detection)
- v0.12 Absorption Sprint 3: Autonomous Memory Curator, Self-improvement Loop, Gateway Media Parity, Gateway Lifecycle Hooks
- v2.0.0 Rebrand: complete hermes→hermesx rebrand (247 files), ExecutionReceipt API + idempotency, OpenAPI 3.0.3 spec, Prometheus business metrics, RBAC auditor role, OTel+Jaeger, production docker-compose, backup/restore scripts
- v2.0.0 Hardening: LifecycleHooks→Runner wired, SelfImprover→Agent loop wired, URL traversal fix, sanitizeForPrompt extracted + applied to compress.go/curator.go
- v2.1.0-webui: React Admin Console (租户/Key/审计/定价/沙箱) + User Portal (SSE Chat/Memories/Skills/Usage) + Bootstrap 引导页; subtle.ConstantTimeCompare + sync.Mutex TOCTOU + sessionStorage key 清除 + isAdmin roles 修复; Vary: Origin CORS; webui.yml 最小权限 CI
- v2.2.0 stabilization: `POST /admin/v1/bootstrap` 增加应用层与 Nginx IP 限流；PG/MySQL `bootstrap_state` 原子 claim 防跨实例重复初始化；PG API key scopes 读写对齐；新会话自动标题；release workflow 切到 Go 1.25
- hermes-agent v0.15.2 absorption: 4-phase upstream sync (core provider refactor, gateway platforms, safety & secrets enhancements)
- Trusted Channel Login: channel provider CRUD, HMAC/challenge verification, gateway channel identity extraction, CSRF middleware, OpenAPI channel auth endpoints

## 依赖

- Redis Cluster: DualLayerLimiter (hash tag {tenantID})
- PostgreSQL: RLS policies + pricing_rules table + pg_trgm extension
- OIDC IdP: wired and production-ready (set OIDC_ISSUER_URL to activate)
- GitHub Container Registry: automated image push on main

## 依赖（v2.3.0 新增）

- DB migrations 000001/000002（safety_policies + secret_patterns 表）必须在集成测试前执行
- `RequireScope("admin")` 中间件已存在（v1.4.0 RBAC）

## 依赖（channel auth 新增）

- DB migrations: channel_credentials + channel_sessions 表（PG/MySQL 双适配）
- Channel provider 注册: 飞书/企微/微信平台 challenge 验证回调

## 风险

- ChatStream breaker.Execute double-counts (accepted — low streaming volume)
- v2.3.0 #36+#37 原子 PR 引入回归风险（准入条件：新增失败 ≤ 5）
- Safety audit 模式上线后需要明确日志消费方和 enforce 升级标准
- OAuth 工具 redirect 目标域须预先注册到 tenant egress allowlist
- CLI app 仍保留 legacy `AIAgent`，全量 Eino 替换尚未覆盖 CLI REPL。
- MCP sampling 链路尚未接入 server-level SafetyInterceptor，本批只覆盖 API chat 与 workflow agent executor。

## 当前阶段产出（2026-05-19 SaaS Cron Scheduler — RELEASED）

- `docs/artifacts/2026-05-19-saas-cron-scheduler/prd.md` ✅
- `docs/artifacts/2026-05-19-saas-cron-scheduler/arch-design.md` ✅
- `docs/artifacts/2026-05-19-saas-cron-scheduler/delivery-plan.md` ✅
- `docs/artifacts/2026-05-19-saas-cron-scheduler/execute-log.md` ✅
- `docs/artifacts/2026-05-19-saas-cron-scheduler/test-plan.md` ✅
- `docs/artifacts/2026-05-19-saas-cron-scheduler/launch-acceptance.md` ✅ Go
- `docs/artifacts/2026-05-19-saas-cron-scheduler/deployment-context.md` ✅
- `docs/artifacts/2026-05-19-saas-cron-scheduler/release-plan.md` ✅ Go

## 下一步（v2.5.0 — Eino Phase 2 全量替换）

**P0（已完成 — SaaS Cron Scheduler v2.4.0）** ✅ RELEASED
- Story A-F: 全部完成（scheduler + executor + delivery + migrations + tests + backlog fixes）

**P1（关键路径 — Eino 全量替换）**
1. 全量 Agent 替换：AIAgent.RunConversation → EinoAgent.RunConversationSafe（API / workflow / gateway 已走 Eino；CLI REPL 仍待迁移）
2. 集成层 SafetyInterceptor 注入：HTTP handler / workflow 已完成；MCP sampling 初始化仍待接入
3. v2.3.0 遗留 R 类项：per-tenant EgressPolicy、redirect IP 验证、Canary 单实例、evolution watcher 已完成；共享 Transport 生产验证、Admin DI 完整重构、WASM sandbox 仍待排期

**P2（功能增强）**
4. 流式 chunk 级脱敏（StreamSafe → per-chunk redaction）
5. 真实 LLM 延迟基准对比测试
6. Workflow 引擎其他 node type 切换到 Eino Graph

**P3（远期）**
7. Phase 3: Workflow DAG → Eino Graph 编排迁移
8. Phase 4: Multi-Agent（Eino Host/Guest 模式）

**当前阶段产出**：
- `docs/artifacts/2026-05-18-v230-security-integration/prd.md` ✅
- `docs/artifacts/2026-05-18-v230-security-integration/arch-design.md` ✅
- `docs/artifacts/2026-05-18-v230-security-integration/delivery-plan.md` ✅
- `docs/artifacts/2026-05-18-v230-security-integration/execute-log.md` ✅
- `docs/artifacts/2026-05-18-v230-security-integration/test-plan.md` ✅
- `docs/artifacts/2026-05-18-v230-security-integration/launch-acceptance.md` ✅ READY
- `docs/artifacts/2026-05-18-v230-security-integration/deployment-context.md` ✅
- `docs/artifacts/2026-05-18-v230-security-integration/release-plan.md` ✅
