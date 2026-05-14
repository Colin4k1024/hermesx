# Project Context: hermesx

**项目名**: hermesx  
**当前任务**: v2.2.0-stabilization
**前任务**: 2026-05-09-k8s-fullstack-deploy（已完成）
**阶段**: active
**版本目标**: v2.2.0 — 发布口径同步 + Bootstrap 安全加固 + API/WebUI 契约修复

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
- v2.2.0-stabilization ACTIVE (2026-05-14 — Bootstrap IP 限流、跨实例原子初始化、PG API key scopes 修复、会话标题 UX、发布/文档口径同步)

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

## 风险

- ChatStream breaker.Execute double-counts (accepted — low streaming volume)
- GHA actions not digest-pinned (P3 — deferred to security sweep)
- v2.1.0 RustFS SDK 兼容性未验证（集成测试前未知）
- v2.1.0 MySQL 全量实现工作量大（~31h 估时，按子接口拆 PR）
- v2.1.0 pprof admin 端点需严格访问控制（生产默认 disabled）

## 下一步（v2.2.0 候选）

1. [Verify] v2.2.0-stabilization 完整验证：Go test/vet + WebUI typecheck/build + Docker/K8s smoke
2. [Infra] store/pg unit tests — pgxmock introduction
3. [Perf] Curator O(n²) dedup optimization
4. [Security] GHA actions digest-pin（deferred from v2.1.0）
5. [UX] Admin Dashboard tenant-level usage aggregation
