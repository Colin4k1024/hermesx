# Project Context: hermesx

**项目名**: hermesx  
**当前任务**: 2026-05-09-k8s-fullstack-deploy  
**前任务**: 2026-05-08-hermesx-webui（已完成）  
**阶段**: closed  
**版本目标**: v2.1.1 — K8s MySQL 全栈部署 + 8 Bug 修复

## Tech Stack

- Go 1.25 + PostgreSQL 16 (+ MySQL 8.0 optional) + Redis 7 + RustFS (replaces MinIO, S3-compat)
- coreos/go-oidc/v3 (OIDC SSO — JWKS rotation, claim mapping, wired in saas.go)
- sony/gobreaker/v2 (per-model circuit breaker with Prometheus metrics)
- Redis Lua atomic ZSET (DualLayerLimiter)
- prometheus/client_golang (breaker state + request counters)
- Helm v3 (PDB/HPA)
- Docker multi-stage build → ghcr.io (CI auto-push)
- GitHub Actions CI (unit + integration + race + coverage + Docker push)
- **webui**: Vue 3 + Pinia + Vue Router v4 + Naive UI + @tanstack/vue-query v5 + Tailwind CSS v4 + Vite 6 multi-page

## 当前状态

- v1.2.0 CLOSED (Phase 1 + Phase 2 enterprise SaaS)
- v1.3.0 CLOSED (Phase 3 — OIDC wiring + breaker metrics + CI/CD)
- v1.4.0 CLOSED (v0.12 upstream absorption — hermes-agent v2026.4.30)
- v2.0.0 CLOSED (post-release hardening — commit 5ea9c44; LifecycleHooks + SelfImprover wired, URL sanitize + prompt sanitize fixed)
- v2.1.0 CLOSED (infra upgrade — ObjectStore interface + RustFS, pprof + OTel + Prometheus, MySQL adapter; K8s local deployment validated 2026-05-08)
- hermesx-webui RELEASED (v2.1.0-webui — Admin Console + User Portal; 4 CRITICAL + 4 HIGH 安全修复; Bootstrap 端点; 旧 HTML 下线; webui CI)
- v2.1.1 RELEASED (2026-05-09 — K8s MySQL 全栈部署验证 + 8 Bug 修复; 详见 session 004)

## 已完成

- Phase 1: RLS write protection, audit immutability, GDPR cleanup, PDB/HPA, session ownership, IDOR fix, CORS fix, credential hygiene
- Phase 2: OIDCExtractor (JWKS + ClaimMapper), DualLayerLimiter (Redis Lua + local fallback), Dynamic PricingStore (30s cache + DB fallback), Admin Pricing CRUD API, store.ErrNotFound sentinel
- Phase 3: OIDC wired into auth chain (env var activation), breaker Prometheus metrics + ChatStream failure recording, CI coverage reporting + Docker ghcr.io push, security hardening (tenant claim validation, JWT error propagation, startup timeout, goroutine leak fix)
- v0.12 Absorption Sprint 1: Model Catalog hot-reload, CJK trigram search, Gateway platform registry refactor
- v0.12 Absorption Sprint 2: MultimodalRouter (image/audio/video dispatch with provider capability detection)
- v0.12 Absorption Sprint 3: Autonomous Memory Curator, Self-improvement Loop, Gateway Media Parity, Gateway Lifecycle Hooks
- v2.0.0 Rebrand: complete hermes→hermesx rebrand (247 files), ExecutionReceipt API + idempotency, OpenAPI 3.0.3 spec, Prometheus business metrics, RBAC auditor role, OTel+Jaeger, production docker-compose, backup/restore scripts
- v2.0.0 Hardening: LifecycleHooks→Runner wired, SelfImprover→Agent loop wired, URL traversal fix, sanitizeForPrompt extracted + applied to compress.go/curator.go
- v2.1.0-webui: Vue 3 Admin Console (租户/Key/审计/定价/沙箱) + User Portal (SSE Chat/Memories/Skills/Usage) + Bootstrap 引导页; subtle.ConstantTimeCompare + sync.Mutex TOCTOU + sessionStorage key 清除 + isAdmin roles 修复; Vary: Origin CORS; webui.yml 最小权限 CI

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

1. [Security P1] Bootstrap 端点 IP 速率限制（当前无 middleware 覆盖）
2. [UX P2] useSse.ts 401/403 auto-logout（当前 SSE 流中异常无自动重定向）
3. [Reliability P2] Bootstrap 跨实例 TOCTOU → DB unique constraint（api_keys.name + tenant_id）
4. [Infra] store/pg unit tests — pgxmock introduction
5. [Perf] Curator O(n²) dedup optimization
6. [Security] GHA actions digest-pin（deferred from v2.1.0）
