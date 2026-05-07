# Delivery Plan — Enterprise SaaS Phase 3

**版本目标**: v1.3.0  
**范围说明**: OIDC activation, breaker observability, CI/CD hardening  
**放行标准**: 全量测试通过 + OIDC integration test green + Prometheus metrics exportable + CI pipeline coverage > 70%  
**主责角色**: tech-lead  
**日期**: 2026-05-07

---

## Requirement Challenge Session Log

| # | 核心假设 | 质疑人 | 质疑内容 | 结论 |
|---|---------|--------|---------|------|
| 1 | OIDC wiring 是一个完整 story | product-manager | 代码已写完，wiring 只有 ~20 行，真正风险在集成验证 | 保留 P1，scope 含 integration test + env docs |
| 2 | 需要全局 breaker registry | architect | 现有 per-model breaker 已有 slog，真正需求是 Prometheus metrics 而非 admin API | 降级为 metrics export + /healthz breaker state，registry 延后 |
| 3 | CI/CD 一次性全面改造 | project-manager | 现有 CI 已覆盖 unit/integration/race/security，全面改造 scope 过大 | 本期只做 coverage + registry push + caching，SBOM/E2E 延后 |

### 未决项

- OIDC IdP 测试环境：是否使用 mock IdP (如 Dex) 或真实 IdP sandbox
- Docker registry 选型：GitHub Container Registry (ghcr.io) vs 自建

---

## 工作拆解

### P3-S1: OIDC Wiring to Auth Chain (P1)

**目标**: 将已实现的 OIDCExtractor 激活到生产 auth chain

**验收标准**:
- `OIDC_ISSUER_URL` + `OIDC_CLIENT_ID` 设置后，OIDC tokens 可通过 auth middleware
- 未设置时行为不变 (backward compatible)
- Integration test 使用 mock IdP 验证完整 token flow
- `.env.example` 和 deployment docs 更新

**影响文件**:
- `cmd/hermes/saas.go` (env parsing + chain wiring, ~25 lines)
- `.env.example` (新增 OIDC_* 变量)
- `tests/integration/oidc_test.go` (新建)
- `deploy/helm/hermes-agent/values.yaml` (OIDC env 注释)

**依赖**: 无外部依赖 (mock IdP 用 httptest)  
**Owner**: backend-engineer  
**预估**: 0.5d

---

### P3-S2: Circuit Breaker Metrics Export (P2)

**目标**: 将 per-model 断路器状态暴露为 Prometheus metrics 和 /healthz 子项

**验收标准**:
- `hermes_breaker_state{model="..."}` gauge (0=closed, 1=half-open, 2=open)
- `hermes_breaker_requests_total{model="...",result="success|failure"}` counter
- `/healthz` endpoint 包含 breaker summary
- **修复 ChatStream 未包裹 breaker.Execute() 的 bug** — stream 失败需计入 trip 阈值
- 不引入全局 registry struct (YAGNI)

**影响文件**:
- `internal/llm/breaker.go` (添加 metrics registration)
- `internal/llm/breaker_metrics.go` (新建, Prometheus collectors)
- `internal/api/healthz.go` (扩展 health check)
- `internal/llm/breaker_metrics_test.go` (新建)

**依赖**: prometheus/client_golang (已在 go.mod)  
**Owner**: backend-engineer  
**预估**: 0.5d

---

### P3-S3: CI/CD — Coverage & Registry Push (P2)

**目标**: CI pipeline 增加覆盖率报告和 Docker 镜像推送

**验收标准**:
- `go test -coverprofile` 输出上传到 PR comment 或 artifact
- Coverage badge 可展示 (Codecov 或 GitHub native)
- main 分支 push 后自动构建并推送 Docker image 到 ghcr.io
- Go module download 缓存启用，CI 时间减少 >30%

**影响文件**:
- `.github/workflows/ci.yml` (coverage step, cache, image push)
- `.github/workflows/release.yml` (image push on tag)
- `Makefile` (test-cover target)

**依赖**: GitHub Container Registry 权限配置  
**Owner**: devops-engineer  
**预估**: 0.5d

---

## 风险与缓解

| 风险 | 影响 | 缓解措施 | Owner |
|------|------|---------|-------|
| Mock IdP 与真实 IdP 行为差异 | OIDC 集成测试假阳性 | 测试覆盖 clock skew, missing claims, expired token 场景 | backend-engineer |
| Prometheus metrics cardinality 爆炸 | 监控成本 | model label 固定为配置中的 model name，不用动态值 | backend-engineer |
| ghcr.io 权限配置错误 | Image push 失败 | 先用 dry-run 验证 workflow permissions | devops-engineer |
| OIDC 激活后 static token 失效 | 现有租户断连 | ExtractorChain 保持顺序: static → OIDC → API key | backend-engineer |

---

## 执行顺序与依赖

```
P3-S1 (OIDC) ──┐
               ├──> /team-review ──> /team-release
P3-S2 (Breaker) ┘
      │
P3-S3 (CI/CD) ─────> 独立验证 (不阻塞 S1/S2)
```

- S1 和 S2 无相互依赖，可并行
- S3 独立于业务代码，可与 S1/S2 并行
- Review 等待 S1+S2 完成后统一进行

---

## 不做项 (Out of Scope)

| 项目 | 原因 | 目标阶段 |
|------|------|---------|
| Breaker admin reset API | YAGNI — 无运维证据需要手动干预 | Phase 4+ |
| E2E tests in CI | Playwright 环境复杂，本期优先 coverage | Phase 4 |
| SBOM generation | 供应链安全加固，非当前阻塞项 | Phase 4 |
| HasScope empty scopes fix | 需 OIDC 生产运行后评估影响 | Phase 4 |
| RLS SELECT policies | 需读隔离需求确认 | Phase 4 |

---

## Karpathy Guidelines 复核

- **假设已显式列出**: OIDC IdP 可通过 env 激活、breaker metrics 不需要 registry、CI 现有覆盖已足够
- **更简单备选路径**: OIDC 只加 20 行 wiring (已选)，breaker 不建 registry 只导出 metrics (已选)
- **当前不做项**: 5 项明确延后
- **为什么本轮范围已经足够**: 3 个 half-day stories 完成后，系统具备 OIDC 生产就绪 + 可观测断路器 + 自动化镜像发布，是 GA 后最关键的运维能力补全

---

## 角色分工

| 角色 | 职责 |
|------|------|
| tech-lead | 计划收口、review 仲裁、release 放行 |
| backend-engineer | S1 (OIDC wiring) + S2 (breaker metrics) 实现 |
| devops-engineer | S3 (CI/CD) 实现 + deployment docs 更新 |
| qa-engineer | Integration test 验证 + launch acceptance |

---

## 检查节点

| 节点 | 条件 | 角色 |
|------|------|------|
| 方案评审 | delivery-plan 锁定 | tech-lead |
| 开发完成 | S1+S2+S3 自测通过 | backend-engineer, devops-engineer |
| 测试完成 | 全量 pass + integration test green | qa-engineer |
| 发布准备 | deployment-context 更新 + image push 验证 | devops-engineer |
