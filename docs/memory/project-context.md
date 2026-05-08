# Project Context: hermesx

**项目名**: hermesx  
**当前任务**: 2026-05-08-hermesx-v200-hardening  
**阶段**: in-progress  
**版本目标**: v2.0.0

## Tech Stack

- Go 1.25 + PostgreSQL 16 + Redis 7 + MinIO
- coreos/go-oidc/v3 (OIDC SSO — JWKS rotation, claim mapping, wired in saas.go)
- sony/gobreaker/v2 (per-model circuit breaker with Prometheus metrics)
- Redis Lua atomic ZSET (DualLayerLimiter)
- prometheus/client_golang (breaker state + request counters)
- Helm v3 (PDB/HPA)
- Docker multi-stage build → ghcr.io (CI auto-push)
- GitHub Actions CI (unit + integration + race + coverage + Docker push)

## 当前状态

- v1.2.0 CLOSED (Phase 1 + Phase 2 enterprise SaaS)
- v1.3.0 CLOSED (Phase 3 — OIDC wiring + breaker metrics + CI/CD)
- v1.4.0 CLOSED (v0.12 upstream absorption — hermes-agent v2026.4.30)
- v2.0.0 IN-PROGRESS (post-release hardening — LifecycleHooks wiring, SelfImprover wiring, security fixes)

## 已完成

- Phase 1: RLS write protection, audit immutability, GDPR cleanup, PDB/HPA, session ownership, IDOR fix, CORS fix, credential hygiene
- Phase 2: OIDCExtractor (JWKS + ClaimMapper), DualLayerLimiter (Redis Lua + local fallback), Dynamic PricingStore (30s cache + DB fallback), Admin Pricing CRUD API, store.ErrNotFound sentinel
- Phase 3: OIDC wired into auth chain (env var activation), breaker Prometheus metrics + ChatStream failure recording, CI coverage reporting + Docker ghcr.io push, security hardening (tenant claim validation, JWT error propagation, startup timeout, goroutine leak fix)
- v0.12 Absorption Sprint 1: Model Catalog hot-reload, CJK trigram search, Gateway platform registry refactor
- v0.12 Absorption Sprint 2: MultimodalRouter (image/audio/video dispatch with provider capability detection)
- v0.12 Absorption Sprint 3: Autonomous Memory Curator, Self-improvement Loop, Gateway Media Parity, Gateway Lifecycle Hooks
- v2.0.0 Rebrand: complete hermes→hermesx rebrand (247 files), ExecutionReceipt API + idempotency, OpenAPI 3.0.3 spec, Prometheus business metrics, RBAC auditor role, OTel+Jaeger, production docker-compose, backup/restore scripts

## 依赖

- Redis Cluster: DualLayerLimiter (hash tag {tenantID})
- PostgreSQL: RLS policies + pricing_rules table + pg_trgm extension
- OIDC IdP: wired and production-ready (set OIDC_ISSUER_URL to activate)
- GitHub Container Registry: automated image push on main

## 风险

- ChatStream breaker.Execute double-counts (accepted — low streaming volume)
- LifecycleHooks not yet wired into Gateway Runner (P1 — in-progress)
- SelfImprover not yet wired into Agent loop (P1 — in-progress)
- compress.go/curator.go LLM prompts not sanitized (P2 — in-progress)
- payload.URL path traversal unvalidated (P2 — in-progress)
- GHA actions not digest-pinned (P3 — deferred to security sweep)

## 下一步

1. [P1] LifecycleHooks → Gateway Runner wiring (in-progress)
2. [P1] SelfImprover → Agent loop wiring (in-progress)
3. [P2] payload.URL path traversal fix (in-progress)
4. [P2] compress.go / curator.go prompt sanitization (in-progress)
5. [P3] store/pg unit tests — pgxmock introduction
6. [P3] Curator O(n²) dedup optimization (if MaxMemories > 100)
7. 生产部署执行 (canary → 50% → full rollout)
