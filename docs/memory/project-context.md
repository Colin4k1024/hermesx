# Project Context: hermesx

**项目名**: hermesx  
**当前任务**: 2026-05-18-v230-security-integration
**前任务**: 2026-05-18-security-enhancement-ironclaw（已合并到 main）
**阶段**: closed（v2.3.0 Security Integration Sprint CLOSED — 9 Story 完成，B-1~B-5 修复，26/26 -race，全链路 artifacts 落盘，6 项 R 类遗留已入 v2.4.0 backlog）
**版本目标**: v2.3.0 — Security Integration Sprint (safety/egress/secrets 三子系统接入主链路)

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

## 依赖

- Redis Cluster: DualLayerLimiter (hash tag {tenantID})
- PostgreSQL: RLS policies + pricing_rules table + pg_trgm extension
- OIDC IdP: wired and production-ready (set OIDC_ISSUER_URL to activate)
- GitHub Container Registry: automated image push on main

## 依赖（v2.3.0 新增）

- DB migrations 000001/000002（safety_policies + secret_patterns 表）必须在集成测试前执行
- `RequireScope("admin")` 中间件已存在（v1.4.0 RBAC）

## 风险

- ChatStream breaker.Execute double-counts (accepted — low streaming volume)
- v2.3.0 #36+#37 原子 PR 引入回归风险（准入条件：新增失败 ≤ 5）
- Safety audit 模式上线后需要明确日志消费方和 enforce 升级标准
- OAuth 工具 redirect 目标域须预先注册到 tenant egress allowlist

## 下一步（v2.3.0 执行顺序）

**P1（关键路径）**
1. Story A: SafetyInterceptor → agent.go RunConversation（#38，8h，可并行）
2. Story B: SecureTransport + CheckRedirect 原子 PR（#36+#37，16h，关键路径）
3. Story E: Canary token TTL 清理（#41，6h，可与 A 并行）

**P2（Story B 完成后）**
4. Story C: 高风险 10 工具迁移 SecretResolver（#39，10h）
5. Story D: Admin API 三 handler 统一注册（#40，10h，可与 C 并行）

**P3（条件进入，P1 回归 ≤ 5）**
6. Story F: #42/#43/#44/#45（17h）

**当前阶段产出**：
- `docs/artifacts/2026-05-18-v230-security-integration/prd.md` ✅
- `docs/artifacts/2026-05-18-v230-security-integration/arch-design.md` ✅
- `docs/artifacts/2026-05-18-v230-security-integration/delivery-plan.md` ✅
- `docs/artifacts/2026-05-18-v230-security-integration/execute-log.md` ✅
- `docs/artifacts/2026-05-18-v230-security-integration/test-plan.md` ✅
- `docs/artifacts/2026-05-18-v230-security-integration/launch-acceptance.md` ✅ READY
- `docs/artifacts/2026-05-18-v230-security-integration/deployment-context.md` ✅
- `docs/artifacts/2026-05-18-v230-security-integration/release-plan.md` ✅
