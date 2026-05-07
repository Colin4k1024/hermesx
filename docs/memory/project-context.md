# Project Context: hermes-agent-go

**项目名**: hermes-agent-go  
**当前任务**: 2026-05-07-enterprise-saas-phase3  
**阶段**: closed  
**版本目标**: v1.3.0

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

- v1.2.0 CLOSED (Phase 1 + Phase 2)
- v1.3.0-rc CLOSED (Phase 3 — OIDC wiring + breaker metrics + CI/CD)
- 全部 enterprise SaaS GA 交付完成，production-ready

## 已完成

- Phase 1: RLS write protection, audit immutability, GDPR cleanup, PDB/HPA, session ownership, IDOR fix, CORS fix, credential hygiene
- Phase 2: OIDCExtractor (JWKS + ClaimMapper), DualLayerLimiter (Redis Lua + local fallback), Dynamic PricingStore (30s cache + DB fallback), Admin Pricing CRUD API, store.ErrNotFound sentinel
- Phase 3: OIDC wired into auth chain (env var activation), breaker Prometheus metrics + ChatStream failure recording, CI coverage reporting + Docker ghcr.io push, security hardening (tenant claim validation, JWT error propagation, startup timeout, goroutine leak fix)

## 依赖

- Redis Cluster: DualLayerLimiter (hash tag {tenantID})
- PostgreSQL: RLS policies + pricing_rules table
- OIDC IdP: wired and production-ready (set OIDC_ISSUER_URL to activate)
- GitHub Container Registry: automated image push on main

## 风险

- ChatStream breaker.Execute double-counts (accepted — low streaming volume)
- Half-open state not throttled for ChatStream (accepted — streaming probes negligible)
- GHA actions not digest-pinned (deferred to next security sweep)
- ACRLevel field populated but enforcement middleware not yet implemented

## 下一步

1. 生产部署执行 (canary → 50% → full rollout)
2. OIDC IdP 联调 (配置 OIDC_ISSUER_URL + OIDC_CLIENT_ID)
3. Grafana dashboard for breaker metrics (hermes_breaker_state, hermes_breaker_requests_total)
4. Backlog: digest-pin GHA actions, CI coverage threshold, ACR enforcement middleware
