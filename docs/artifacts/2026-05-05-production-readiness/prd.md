# PRD: Hermes Agent Go — Production Readiness

**状态**: Draft  
**主责**: tech-lead  
**日期**: 2026-05-05  
**Slug**: production-readiness

---

## 1. 背景

### 1.1 业务问题

Hermes Agent Go 已完成 SaaS 多租户核心架构（v1.0.0），通过 38 个集成测试验证了 7 层安全隔离机制。但当前系统仍处于验证原型阶段，距离承接真实企业客户流量存在明确差距。

### 1.2 触发原因

- 集成测试全部通过后的自然演进需求
- 需要向领导层展示 production-ready 路线图
- 商业化时间窗口压力

### 1.3 当前约束

| 维度 | 现状 | 差距 |
|------|------|------|
| 认证 | API Key SHA256 only | 无 OAuth2/OIDC/SSO |
| LLM 集成 | Mock LLM 测试 | 真实 LLM 流式/重试/降级未验证 |
| 可观测性 | Prometheus handler 存在但无 tracing | OpenTelemetry 依赖已引入但未接线 |
| CI/CD | 基础 CI 存在（build/test/lint） | 无集成测试、无安全扫描、无部署 |
| 限流 | rate_limit_rpm 字段存在 | 无真实限流验证、无熔断 |
| 扩展 | 单实例通过测试 | 多副本/负载均衡未验证 |
| 运维 API | SandboxPolicy 只能 SQL 设置 | 无 Admin API |
| 灾备 | 无 | 无备份/恢复/RTO/RPO 定义 |
| 合规 | GDPR 链路代码存在 | 无审计证据、无 SOC2 映射 |
| 计费 | 基础 usage endpoint | 无精确 token 计量、无计费集成 |

### 1.4 已有基础设施

- `go.mod` 已引入: `prometheus/client_golang`, `go.opentelemetry.io/otel`, `golang-jwt/jwt/v5`
- `.github/workflows/ci.yml` 已有: build, vet, test, race detection, lint, docker build
- `internal/api/metrics.go`: Prometheus handler 已注册
- `internal/api/usage.go`: 基础 usage 统计（待重构）
- 48K+ 行 Go 代码，18 个测试包，60 个 DB migrations

---

## 2. 目标与成功标准

### 2.1 业务目标

将 Hermes Agent Go 从"安全架构已验证"推进到"可接入首批企业客户"的 production-ready 状态。

### 2.2 用户价值

- 企业客户可通过 SSO 无缝接入
- 平台可稳定承载多租户真实负载
- 运维团队可独立管理租户策略
- 安全团队有合规证据支撑审计

### 2.3 成功标准

| 标准 | 度量 |
|------|------|
| 认证覆盖 | OAuth2/OIDC + API Key 双模式可用 |
| LLM 可靠性 | 真实 LLM 测试通过率 ≥ 99%，含重试/降级 |
| 可观测性 | Trace → Metric → Log 三柱完整联通 |
| CI 门禁 | 集成测试 + 安全扫描在 PR 合并前强制 |
| 限流准确性 | 并发压测下限流误差 < 5% |
| 水平扩展 | 3 副本 + LB 下隔离测试全部通过 |
| Admin API | SandboxPolicy/Tenant CRUD 全覆盖 |
| RTO/RPO | RTO < 1h, RPO < 5min（定义并验证） |
| 合规就绪 | 审计日志完整 + GDPR 端到端可验证 |
| 计费精度 | Token 级计量误差 < 1% |

---

## 3. 用户故事

### US-1: 企业 SSO 接入
**As** 企业 IT 管理员  
**I want** 通过公司 OIDC Provider 登录 Hermes  
**So that** 员工无需管理额外凭证，离职时自动吊销权限  
**AC**: OIDC discovery → token exchange → tenant mapping → session 建立

### US-2: LLM 调用可靠性
**As** 开发者  
**I want** 在 LLM 服务短暂不可用时系统自动重试和降级  
**So that** 我的 agent 不会因为偶发超时而完全失败  
**AC**: 指数退避重试 3 次 → 主模型不可用时降级到备选模型 → 返回降级标记

### US-3: 运维自助管理
**As** 平台运维  
**I want** 通过 Admin API 管理租户 SandboxPolicy  
**So that** 不需要直连数据库修改配置  
**AC**: CRUD endpoints + 权限校验 + 审计日志

### US-4: 故障快速定位
**As** SRE  
**I want** 从一条 trace_id 追踪请求全链路  
**So that** 出问题时能在 5 分钟内定位到具体租户和代码路径  
**AC**: Request → API → Store → LLM 全链路 trace_id 贯穿

### US-5: 合规审计
**As** 安全审计员  
**I want** 查看所有数据操作的审计日志  
**So that** 能证明平台满足 GDPR 合规要求  
**AC**: 审计日志包含 who/what/when/where，保留 90 天，不可篡改

### US-6: 用量计费
**As** 财务  
**I want** 精确获取每个租户的 token 消耗  
**So that** 能按量计费或设置用量告警  
**AC**: 按 session 粒度记录 input/output tokens，支持按日/月聚合查询

---

## 4. 范围

### 4.1 In Scope

| Phase | 内容 | 优先级 |
|-------|------|--------|
| P1 | OAuth2/OIDC 认证 + CI/CD 集成测试 + OpenTelemetry 接线 | Critical |
| P2 | 真实 LLM 集成（重试/降级/流式）+ 分布式限流 + 水平扩展验证 | High |
| P3 | Admin API（SandboxPolicy CRUD）+ 备份恢复 + 计费计量 | High |
| P4 | 合规加固（审计完善 + 渗透测试准备 + 文档化） | Medium |

### 4.2 Out of Scope

- 前端 UI 变更（当前无前端改动计划）
- 多区域/多云部署（后续 Phase）
- 自建 LLM 模型服务
- SOC2 正式认证（仅准备控制点映射）
- Kubernetes 集群运维自动化（使用现有 Docker/Compose 部署）

---

## 5. 风险与依赖

### 5.1 关键依赖

| 依赖 | 说明 | 风险等级 |
|------|------|----------|
| OIDC Provider | 需要选型（Keycloak/Auth0/自建） | Medium — 选型影响后续集成复杂度 |
| LLM Provider 稳定性 | OpenAI/Anthropic API SLA | Low — 多模型降级可缓解 |
| PostgreSQL HA | 备份恢复依赖 PG 本身能力 | Low — 成熟方案 |

### 5.2 风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| OIDC 集成复杂度超预期 | P1 延期 | 先支持标准协议，不做定制化 |
| 真实 LLM 测试成本 | 预算消耗 | 用低成本模型做集成测试 |
| 水平扩展暴露隐藏 bug | 并发问题 | 渐进式加副本 + race detection |
| 限流在分布式下精度下降 | 超卖或误杀 | Redis sliding window 算法 |

### 5.3 待确认项

| # | 问题 | 决策方 | 影响 |
|---|------|--------|------|
| 1 | OIDC Provider 选型：Keycloak vs Auth0 vs 自建 | tech-lead | P1 启动前必须确认 |
| 2 | LLM 降级策略：返回错误 vs 降级模型 vs 缓存响应 | tech-lead + product | 影响用户体验 |
| 3 | 计费粒度：Token 级 vs Session 级 vs 请求级 | product + finance | 影响存储设计 |
| 4 | RTO/RPO 目标：1h/5min vs 15min/1min | tech-lead + ops | 影响架构复杂度和成本 |
| 5 | 是否需要多模型路由（OpenAI + Anthropic + Bedrock） | product | 影响 LLM 集成范围 |

---

## 6. 关键假设

1. 首批企业客户量级 < 50 租户，日均请求 < 10 万
2. 可以使用 Redis 作为分布式限流和缓存的中间件（已有 Redis 依赖）
3. PostgreSQL 逻辑复制或 PITR 满足 RPO 需求
4. 不需要自建 OIDC Provider，可以直接集成标准协议
5. CI/CD 使用 GitHub Actions（已有基础 pipeline）

---

## 7. 非目标

- 不追求 100% 自动化运维（允许 runbook + 手动介入）
- 不追求多区域部署能力（单区域 + 跨 AZ 足够）
- 不在本阶段做性能极限优化（满足 < 50 租户即可）
- 不做正式安全认证（SOC2/ISO27001 留到商业化阶段）

---

## 8. 企业治理待确认项

| 维度 | 状态 | 说明 |
|------|------|------|
| 应用等级 | 待确认 | 建议 T3（需限流/熔断/扩缩容/多版本） |
| 技术架构等级 | 待确认 | 单区域多副本 + Redis + PG HA |
| 数据/合规风险 | 已识别 | 含用户对话数据，GDPR 适用 |
| 集团组件约束 | 不适用 | 独立产品，非集团内部系统 |

---

## 9. 参与角色清单

| 角色 | 职责 |
|------|------|
| tech-lead | 整体方案决策、选型仲裁、交付收口 |
| architect | OIDC/可观测性/扩展架构设计 |
| backend-engineer | 核心功能实现 |
| devops-engineer | CI/CD、部署、备份恢复 |
| qa-engineer | 集成测试、压测、安全扫描 |

---

## 10. 需求挑战会候选分组

### Group A: 认证与安全
- OIDC Provider 选型
- API Key 与 OIDC 共存策略
- Token 刷新机制
- 密钥轮换方案

### Group B: 可靠性与扩展
- LLM 重试/降级策略
- 分布式限流算法选型
- 水平扩展下的 Session 亲和性
- RTO/RPO 目标确认

### Group C: 运维与商业化
- Admin API 权限模型
- 计费粒度与存储方案
- 审计日志存储与查询
- 监控告警阈值定义

---

## 11. 领域技能包启用建议

| 技能 | 原因 |
|------|------|
| `golang-patterns` | Go 实现最佳实践 |
| `api-design` | Admin API 契约设计 |
| `deployment-patterns` | CI/CD + 多副本部署 |
| `security-review` | OIDC 集成安全审查 |
| `postgres-patterns` | 备份/HA/性能优化 |

---

## 12. 交付计划概览

```
Week 1-2: P1 — 认证 + CI/CD + 可观测性
  ├── OAuth2/OIDC 中间件 + Provider 对接
  ├── CI Pipeline 补充集成测试 + 安全扫描
  └── OpenTelemetry SDK 接线 + Jaeger/Grafana 联调

Week 3-4: P2 — LLM 集成 + 限流 + 扩展
  ├── 真实 LLM 集成测试（重试/流式/降级）
  ├── Redis sliding window 限流
  └── 多副本部署 + 隔离测试重跑

Week 5-6: P3 — Admin API + 备份 + 计费
  ├── SandboxPolicy/Tenant CRUD API
  ├── PG PITR + 恢复验证
  └── Token 级计量 + 聚合查询

Week 7-8: P4 — 合规 + 收尾
  ├── 审计日志完善 + 查询 API
  ├── GDPR 端到端验证
  └── 安全扫描报告 + 渗透测试准备
```

---

**已创建**: `docs/artifacts/2026-05-05-production-readiness/prd.md`
