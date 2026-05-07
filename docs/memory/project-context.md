# Project Context: hermes-agent-go

**项目名**: hermes-agent-go  
**当前任务**: 2026-05-07-v012-absorption  
**阶段**: closed  
**版本目标**: v1.4.0

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
- 全部 enterprise SaaS GA + upstream absorption 交付完成

## 已完成

- Phase 1: RLS write protection, audit immutability, GDPR cleanup, PDB/HPA, session ownership, IDOR fix, CORS fix, credential hygiene
- Phase 2: OIDCExtractor (JWKS + ClaimMapper), DualLayerLimiter (Redis Lua + local fallback), Dynamic PricingStore (30s cache + DB fallback), Admin Pricing CRUD API, store.ErrNotFound sentinel
- Phase 3: OIDC wired into auth chain (env var activation), breaker Prometheus metrics + ChatStream failure recording, CI coverage reporting + Docker ghcr.io push, security hardening (tenant claim validation, JWT error propagation, startup timeout, goroutine leak fix)
- v0.12 Absorption Sprint 1: Model Catalog hot-reload, CJK trigram search, Gateway platform registry refactor
- v0.12 Absorption Sprint 2: MultimodalRouter (image/audio/video dispatch with provider capability detection)
- v0.12 Absorption Sprint 3: Autonomous Memory Curator, Self-improvement Loop, Gateway Media Parity, Gateway Lifecycle Hooks

## 依赖

- Redis Cluster: DualLayerLimiter (hash tag {tenantID})
- PostgreSQL: RLS policies + pricing_rules table + pg_trgm extension
- OIDC IdP: wired and production-ready (set OIDC_ISSUER_URL to activate)
- GitHub Container Registry: automated image push on main

## 风险

- ChatStream breaker.Execute double-counts (accepted — low streaming volume)
- LifecycleHooks not yet wired into Gateway Runner (standalone correct, additive work)
- SelfImprover not yet wired into Agent loop (standalone correct, additive work)
- compress.go/curator.go LLM prompts not sanitized (server-controlled data only)
- GHA actions not digest-pinned (deferred to next security sweep)

## 下一步

1. LifecycleHooks integration into Gateway Runner
2. SelfImprover wiring into Agent conversation loop
3. compress.go / curator.go prompt sanitization consistency
4. payload.URL traversal check extension
5. Curator O(n²) dedup optimization (if MaxMemories > 100 needed)
6. 生产部署执行 (canary → 50% → full rollout)
