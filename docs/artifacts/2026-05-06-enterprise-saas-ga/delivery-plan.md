# Delivery Plan: Enterprise SaaS GA Hardening v1.2.0

**状态**: Draft  
**主责**: tech-lead  
**日期**: 2026-05-06  
**Slug**: enterprise-saas-ga  
**版本目标**: v1.2.0 GA

---

## 1. 版本目标

将 hermes-agent-go 从准生产级（6.2/10）推进到企业 GA（8.5+/10），修复所有 CRITICAL 安全缺口，补全商业完整性和运维可运营性。

**放行标准**：
- 5 个 CRITICAL 缺口全部修复（含 RLS WITH CHECK、审计不可篡改、默认凭证、GDPR MinIO、备份可验证）
- K8s PDB + HPA 生效（kind 集群验证）
- CI pipeline 全绿（go test + gofmt + race detection）
- 集成测试覆盖新增功能

---

## 2. Phase 划分与 Story Slices

### Phase 1 — CRITICAL 安全修复（P0）

**目标**：消除所有 CRITICAL 安全阻断缺口  
**前置依赖**：无  
**预估**：3-4 天

| Slice | US | 主责 | 验收标准 | 依赖 |
|-------|-----|------|---------|------|
| P1-S1 | US-01 | backend-engineer | 9 张表 WITH CHECK + FORCE RLS migration 通过；跨租户 INSERT 返回 policy violation | 无 |
| P1-S2 | US-02 | backend-engineer | REVOKE DELETE + SECURITY DEFINER GDPR 函数；应用角色 DELETE 返回 permission denied | P1-S1 |
| P1-S3 | US-03 | devops-engineer | docker-compose 无默认值；缺 env 启动失败；.env.example 占位符 | 无 |
| P1-S4 | US-04 | backend-engineer | GDPR deleteViaTx 后 MinIO prefix 为空；失败返回 207 + purge_audit_logs 记录 | P1-S2 |
| P1-S5 | US-05 | devops-engineer | pitr-drill.sh 完整执行；RPO delta 输出；Prometheus backup 指标 | 无 |
| P1-S6 | US-06 | devops-engineer | Helm PDB + HPA 模板；kind 集群 kubectl get pdb,hpa 有对象 | 无 |
| P1-S7 | US-14 | backend-engineer | session/message owner 校验；同租户跨用户 403 | 无 |

**P1 Handoff 终点**：所有 CRITICAL 修复 + 安全 Bug Fix 代码完成，集成测试通过，`go test ./...` 全绿。

---

### Phase 2 — 商业完整性（P1）

**目标**：补全 OAuth2/OIDC、动态计费、用户限流、断路器、账单  
**前置依赖**：Phase 1 完成（RLS WITH CHECK 影响写路径，须先稳定）  
**预估**：7-10 天

| Slice | US | 主责 | 验收标准 | 依赖 |
|-------|-----|------|---------|------|
| P2-S1 | US-07 | backend-engineer | OIDCExtractor + JWKS 动态刷新；mock IdP 测试通过 | P1 完成 |
| P2-S2 | US-08 | backend-engineer | pricing_rules 表 + Admin CRUD API；成本计算走 DB 优先 | P1 完成 |
| P2-S3 | US-09 | backend-engineer | 双层 Lua 限流（tenant + user/key）；X-RateLimit 头正确 | P1 完成 |
| P2-S4 | US-10 | backend-engineer | ProviderBreakerRegistry + ChatStream 统计修复；Prometheus gauge | P1 完成 |
| P2-S5 | US-11 | backend-engineer | billing summary + invoice JSON API；admin scope 鉴权 | P2-S2 |
| P2-S6 | US-13 | backend-engineer | OIDCExtractor ACR claim 校验；无 MFA token 访问 /admin 返回 403 | P2-S1 |

**P2 Handoff 终点**：所有 HIGH 优先级功能代码完成，接口契约锁定，单元/集成测试通过。

---

### Phase 3 — 能力完善与运维文档（P2）

**目标**：补全 MEDIUM 能力项 + 运维 Runbook + CI 覆盖率可见性  
**前置依赖**：Phase 2 接口锁定（Runbook 依赖接口）  
**预估**：4-6 天

| Slice | US | 主责 | 验收标准 | 依赖 |
|-------|-----|------|---------|------|
| P3-S1 | US-15 | backend-engineer | /admin/v1/tenants CRUD 路由 + PATCH + restore 端点 | P2 完成 |
| P3-S2 | US-16 | backend-engineer | createAPIKey 强制 ExpiresAt；List 响应 expires_soon 标志 | P2 完成 |
| P3-S3 | US-17 | backend-engineer | LoggingMiddleware 注入 trace_id/span_id；unit test 验证 | P2 完成 |
| P3-S4 | US-18 | devops-engineer | Helm Service sessionAffinity 条件化；nginx 注释更新 | P2 完成 |
| P3-S5 | US-19 | devops-engineer | CI -coverprofile + coverage artifact 上传 | P2 完成 |
| P3-S6 | US-20 | devops-engineer | NetworkPolicy 模板（enabled: false 默认） | P2 完成 |
| P3-S7 | US-12 | devops-engineer | 8 份 Runbook（incident-response 优先） | P2 接口锁定 |

**P3 Handoff 终点**：全部 20 个 US 代码/文档完成，CI 全绿，进入 review。

---

## 3. 角色分工

| 角色 | Phase 1 | Phase 2 | Phase 3 |
|------|---------|---------|---------|
| tech-lead | 方案决策、CRITICAL 优先级裁定、安全审查 | 接口契约收口、scope 决策 | 上线门禁、放行决策 |
| architect | RLS 设计审查、OIDC 架构指导 | breaker 注册表设计、pricing 接口审查 | NetworkPolicy 规则审查 |
| backend-engineer | P1-S1/S2/S4/S7 实现 | P2-S1~S6 实现 | P3-S1~S3 实现 |
| devops-engineer | P1-S3/S5/S6 实现 | 协助 kind 验证 | P3-S4~S7 实现 |
| qa-engineer | P1 集成测试 | P2 集成测试 + OIDC mock 验证 | 全量回归 + 覆盖率报告 |
| security-reviewer | P1 安全审查（RLS、审计、凭证） | OIDC 安全配置审查 | 最终安全扫描 |

---

## 4. 风险与缓解

| 风险 | 影响 | 概率 | 缓解措施 | Owner |
|------|------|------|---------|-------|
| RLS WITH CHECK 破坏现有写入路径 | HIGH | MEDIUM | 先测试库逐表验证；SET LOCAL 确保事务内生效 | backend-engineer |
| OIDC 需外部 IdP 配合测试 | HIGH | HIGH | mock IdP (httptest.Server) + coreos/go-oidc | backend-engineer |
| pitr-drill.sh Docker 环境限制 | MEDIUM | HIGH | docker exec 模式替代 volume inspect | devops-engineer |
| 双层限流 Lua 脚本复杂度 | MEDIUM | MEDIUM | 充分表驱动测试 + Redis 集群模式验证 | backend-engineer |
| Helm PDB minAvailable 阻塞 drain | LOW | HIGH | 条件化渲染 replicaCount >= 2 | devops-engineer |

---

## 5. 节点检查

| 检查点 | 时间 | 角色 | 门禁 |
|--------|------|------|------|
| Phase 1 安全审查 | P1 完成后 | security-reviewer | 无 CRITICAL/HIGH 遗留 |
| Phase 2 接口冻结 | P2-S5 完成后 | architect + tech-lead | 接口契约 JSON 通过对比审查 |
| Phase 3 全量回归 | P3 完成后 | qa-engineer | go test 全绿 + race detection PASS |
| Release Readiness | P3 + 回归后 | tech-lead | CI 全绿 + 安全扫描 PASS |

---

## 6. 降级到 v1.3.0 的功能

| 功能 | 原因 |
|------|------|
| API Key 30 天提醒通知 | 通知基础设施未建立 |
| CI 80% 覆盖率硬阻断 | 当前覆盖率 34%，需先提升基线 |
| Cookie 粘滞（nginx sticky） | Nginx 开源版不支持 |
| NetworkPolicy 生产验证 | kind CNI 不支持 NetworkPolicy enforce |
| per-tenant OIDC IdP 映射 | 单 IdP 限制 GA 文档说明 |
| invoice 不可变快照 | invoices 表状态机工作量大 |

---

## 7. Karpathy Guidelines 收口

### 核心假设
1. 所有写路径已在事务中 — 需逐包验证（store/pg 层）
2. GoBreaker v2 支持泛型 — 已确认 go.mod 中版本兼容
3. MinIO ListObjects + RemoveObject 可覆盖全部租户对象 — 需验证 soul/skills 前缀一致性
4. kind 集群 metrics-server 安装后 HPA 可正常工作 — 已有成熟脚本

### 更简单备选路径
- OIDC: 可用 `coreos/go-oidc/v3` 替代 `lestrrat-go/jwx`（更少依赖）
- 断路器: 可保持 model 级 breaker 不改粒度（但违背 US-10 要求）
- 账单: 可直接 SQL SUM 不做缓存（GA 数据量有限）

### 当前不做项
- WebUI 前端
- 多云/多地域部署
- SOC2 认证流程
- RL Training 接入
- PDF 发票

### 为什么本轮范围已足够
v1.2.0 修复所有 CRITICAL + HIGH 后，安全合规评分从 6.2 提升至 8.5+，满足企业采购审查的最低门槛。MEDIUM 项中 US-13~US-20 均为增量改进，不阻塞 GA 发布。

---

## 8. Implementation Readiness 结论

**状态**: `handoff-ready`

**前置证据**：
- ✅ PRD 完成（20 个 US，范围/成功标准明确）
- ✅ 需求挑战会完成（9 项 tech-lead 决策已记录）
- ✅ 接口约定已在 requirement-challenge.md 中锁定
- ✅ 技术依赖已满足（PG16、Redis7、MinIO SDK、gobreaker v2 均已在 go.mod）
- ✅ 待确认项已全部收口

**可执行条件**：本 delivery-plan 经 tech-lead 确认后，即可进入 `/team-execute` Phase 1。

---

## 9. Phase 2 — Refined Execution Plan (Post-Challenge)

**更新日期**: 2026-05-07  
**更新原因**: Phase 1 已完成收口，Phase 2 进入执行规划阶段。经需求挑战会识别隐藏依赖和接口变更风险后，对原 Phase 2 计划进行如下调整。

### 9.1 需求挑战会结论

| # | 假设 | 质疑人 | 结论 | 处置 |
|---|------|--------|------|------|
| C1 | RateLimiter 接口可透明扩展为双层 | architect | **错误** — `Allow(key, limit)` 签名不支持原子双键检查 | 新建 `DualLayerLimiter` 接口，保留旧接口向后兼容，ADR-002 记录 |
| C2 | OIDC `sub` claim 可直接映射 AuthContext.Identity | architect | **部分正确** — 需增加显式 `UserID` 字段以区分 API key ID 和真实用户 | P2-S0 扩展 AuthContext |
| C3 | 30s singleflight 缓存足够保证计费一致性 | architect | **风险可接受** — billing 路径直读 DB 绕过缓存；实时定价允许 30s 漂移 | P2-S5 invoice 生成时强制 DB 查询 |
| C4 | P2-S3 与 P2-S1 无依赖 | project-manager | **错误** — 用户级限流需要 UserID，仅 OIDC 和 JWT 提供真实用户标识 | P2-S3 排在 P2-S1 之后 |
| C5 | Provider-level breaker 可无缝替换 model-level | architect | **需预修复** — ChatStream 未调用 breaker.Execute，流式错误不计入断路 | 作为 P2-S4 前置 bugfix |
| C6 | 7-10 天可完成全部 6 个 slice | project-manager | **乐观** — 含接口变更和新依赖，实际 10-12 天；MVP 可在 7-8 天内交付 | 设 MVP 切割线，day 8 做 go/no-go |

### 9.2 AuthContext 扩展（P2-S0 前置 Slice）

新增字段：
```go
type AuthContext struct {
    Identity   string   // API key ID 或 OIDC sub (保持向后兼容)
    UserID     string   // 真实用户标识 (OIDC sub / JWT sub)；API key 为空
    TenantID   string
    Roles      []string
    Scopes     []string
    AuthMethod string
    ACRLevel   string   // OIDC acr claim; 空 = 未提供
}
```

影响面：
- Rate limiter key 构造：`rl:{tenantID}:user:{UserID}` (有 UserID 时) 或 `rl:{tenantID}:key:{Identity}` (API key)
- Billing 记录：使用 UserID 关联真实用户
- ACR 校验：P2-S6 读取 ACRLevel 字段
- 向后兼容：现有代码使用 Identity/TenantID/Roles 不受影响

### 9.3 Refined Story Slices（执行顺序）

| 序号 | Slice | US | 验收标准 | 前置依赖 | 预估 |
|------|-------|----|---------|----------|------|
| 0 | **P2-S0: AuthContext 扩展** | — | `UserID`/`ACRLevel` 字段添加；JWT/APIKey extractor 正确填充；现有测试全绿 | Phase 1 ✅ | 0.5d |
| 1 | **P2-S2: Dynamic Pricing** | US-08 | PricingStore CRUD + 30s 缓存 + Admin API；合并两处硬编码定价 | P2-S0 | 1.5d |
| 2 | **P2-S1: OIDC SSO** | US-07 | OIDCExtractor + JWKS rotation + mock IdP test；填充 UserID/ACRLevel | P2-S0 | 2.5d |
| 3 | **P2-S3: Dual Rate Limiting** | US-09 | DualLayerLimiter + 单 Lua 脚本原子检查；X-RateLimit 头正确；local fallback 降级 | P2-S1 (UserID) | 2d |
| — | ════ **MVP 切割线** ════ | | S0+S1+S2+S3 完成 = 企业核心可用 | | ~6.5d |
| 4 | **P2-S4: Provider Breaker** | US-10 | ProviderBreakerRegistry + ChatStream Execute 修复 + Prometheus gauge | P2-S0 | 1.5d |
| 5 | **P2-S5: Monthly Billing** | US-11 | BillingStore summary/invoice API；invoice 路径强制 DB 读取定价 | P2-S2 | 1.5d |
| 6 | **P2-S6: Admin ACR** | US-13 | /admin/* 路由强制 ACRLevel == "mfa"；无 MFA token 返回 403 | P2-S1 | 0.5d |

**总预估**：10-11 天（含 review 和 buffer）  
**MVP 交付**：6.5-7 天（S0+S1+S2+S3）  
**Go/No-Go 检查点**：Day 8 — 若 MVP 未完成则砍 S5/S6

### 9.4 依赖图

```
P2-S0 (AuthContext)
  ├── P2-S2 (Pricing)
  │     └── P2-S5 (Billing)
  ├── P2-S1 (OIDC)
  │     ├── P2-S3 (Rate Limit)
  │     └── P2-S6 (ACR)
  └── P2-S4 (Breaker)
```

### 9.5 接口变更清单

| 变更 | 类型 | ADR | 影响范围 |
|------|------|-----|---------|
| `AuthContext` 新增 `UserID`, `ACRLevel` | 向后兼容扩展 | 不需要 | 所有 extractor 需填充新字段 |
| 新建 `DualLayerLimiter` 接口 | 新增接口，旧接口保留 | ADR-002 | middleware/ratelimit.go, redis_ratelimiter.go |
| `ProviderBreakerRegistry` 替代 inline breaker | 内部重构 | 不需要 | llm/breaker.go, fallback_router.go |
| 合并 `metering/cost.go` + `agent/pricing.go` → `PricingStore` | 内部重构 | 不需要 | 两处 cost 函数调用方迁移 |
| `Store` 接口新增 `PricingRules()`, `Billing()` | 向后兼容扩展 | 不需要 | pg store 新增实现文件 |

### 9.6 风险更新（Phase 2 特定）

| 风险 | 影响 | 概率 | 缓解措施 | Owner |
|------|------|------|---------|-------|
| OIDC IdP `acr` claim 格式不标准 | HIGH | MEDIUM | ClaimMapper 配置化；P2-S6 若无标准 acr 则降级为"仅校验 OIDC 登录" | backend-engineer |
| 双层 Lua 脚本 Redis 集群兼容性 | MEDIUM | MEDIUM | 使用 `{tenantID}` hash tag 保证同 slot | backend-engineer |
| singleflight 缓存可能掩盖定价更新 | LOW | LOW | billing 路径强制 DB 读取；实时 API 允许 30s 漂移 | backend-engineer |
| ChatStream breaker 修复需改流式接口 | MEDIUM | MEDIUM | 不改 Transport 接口，用 wrapper 在消费端计数 | backend-engineer |
| Prometheus label 基数爆炸（user-level） | LOW | HIGH | rate limit metric 不按 user 打标；只按 tenant + layer 分 | backend-engineer |

### 9.7 Karpathy Guidelines 收口（Phase 2）

**核心假设**：
1. 单 IdP 配置足以覆盖 GA 需求 — 多 IdP 延后到 v1.3.0
2. DualLayerLimiter 新接口不破坏现有 RateLimiter 消费方 — 保留旧接口
3. ChatStream breaker 可用消费端 wrapper 修复 — 不改 Transport 签名

**更简单备选路径**：
- OIDC: 若 mock IdP 复杂度超预期，可退到 JWT + 静态 JWKS（已有 jwt.go）
- Rate Limit: 若 Lua 复杂度超预期，可退到两次 Allow 调用 + 文档说明非原子性
- Billing: 若时间不足，invoice API 可简化为原始 SQL 聚合（无 cache/format）

**当前不做项**：
- 多 IdP per-tenant 配置
- PDF invoice 生成
- Rate limit burst 模式
- Provider breaker dashboard UI
- per-user pricing tier

**为什么 MVP 切割线足够**：
S0+S1+S2+S3 覆盖了企业客户三大核心需求：SSO 登录 (OIDC)、透明计价 (Dynamic Pricing)、公平使用保障 (User Rate Limiting)。S4/S5/S6 为 resilience 和 billing 增强，不阻塞企业采购审查。

### 9.8 Phase 2 Implementation Readiness

**状态**: `handoff-ready`

**前置证据**：
- ✅ Phase 1 已完成收口 (closeout-summary.md)
- ✅ 需求挑战会完成 (6 项假设质疑 + 结论)
- ✅ 依赖已确认 (coreos/go-oidc/v3 待引入，gobreaker/redis/pg 已有)
- ✅ 接口变更清单已锁定 (5 项变更，1 项需要 ADR)
- ✅ pricing_rules 表已存在 (migration v69)
- ✅ AuthContext 扩展方案已确认 (向后兼容)

**阻塞项**：
- ADR-002 (DualLayerLimiter) 需在 P2-S3 执行前完成

**可执行条件**：ADR-002 完成后，即可进入 `/team-execute` Phase 2。
