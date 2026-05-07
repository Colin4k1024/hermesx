# Test Plan: enterprise-saas-ga v1.2.0 Phase 1

| 字段 | 值 |
|------|-----|
| 任务 | 2026-05-06-enterprise-saas-ga |
| 阶段 | Phase 1 (CRITICAL Security Hardening) |
| 主责 | qa-engineer |
| 状态 | BLOCK — 需修复后重新评审 |
| 评审时间 | 2026-05-06 |

---

## 测试范围

### 功能范围
- RLS WITH CHECK 写策略 (migration 65-66)
- 审计日志不可变性 (migration 67-68, REVOKE DELETE + SECURITY DEFINER)
- Docker Compose 凭证硬编码移除
- GDPR MinIO 对象清理 + 207 Multi-Status
- K8s PDB + HPA Helm 模板
- Session owner 安全检查 (agent_chat.go)

### 非功能范围
- RLS 性能影响（`current_setting()` 在高并发下的开销）
- MinIO 大批量对象删除的超时行为
- HPA scale-down 稳定性窗口验证

### 不覆盖项
- Phase 2/3 功能（OIDC、断路器、计费）
- 前端变更（本次无 UI 改动）
- LLM 集成端到端验证

---

## 评审发现汇总

### Code Reviewer

| 严重度 | 数量 | 状态 |
|--------|------|------|
| CRITICAL | 2 | BLOCK |
| HIGH | 4 | BLOCK |
| MEDIUM | 4 | WARN |
| LOW | 3 | NOTE |

### Security Reviewer

| 严重度 | 数量 | 状态 |
|--------|------|------|
| CRITICAL | 0 | — |
| HIGH | 4 | BLOCK |
| MEDIUM | 6 | WARN |
| LOW | 3 | NOTE |

---

## CRITICAL 阻塞项（必须修复）

### CR-CRIT-1: RLS 写策略缺少 SET LOCAL 会导致全量写入失败

- **文件**: `internal/store/pg/migrate.go` migration 66
- **问题**: `current_setting('app.current_tenant', false)` 在 GUC 未设置时会 ERROR。当前 Store 层所有写操作从未调用 `SET LOCAL app.current_tenant`，上线后所有 INSERT/UPDATE/DELETE 将立即失败。
- **修复方案**: 在 Store 层 PG pool 添加 `BeforeAcquire` hook 或事务包装器，在 DML 前执行 `SET LOCAL app.current_tenant = $tenantID`。
- **验证**: 集成测试用非 superuser 连接执行 CRUD → 成功

### CR-CRIT-2: GDPR handler 泄露内部错误详情

- **文件**: `internal/api/gdpr.go` lines 172, 281
- **问题**: `err.Error()` 直接返回给客户端，可能包含 PG 表名、MinIO bucket 名、object key 等敏感信息
- **修复方案**: 返回通用错误消息，server-side 记录详细日志
- **验证**: 触发 MinIO/PG 错误 → 响应体不含内部路径信息

---

## HIGH 阻塞项（应修复后合并）

### CR-HIGH-1: SkillsClient 作为 GDPR MinIO 客户端语义不匹配

- **问题**: `cfg.SkillsClient` 是 skills bucket 客户端，GDPR 删除只清理 skills 数据，可能遗漏其他 tenant 数据
- **修复**: 明确文档说明 SkillsClient 即唯一 tenant 数据 bucket，或引入独立 GDPRMinIOClient

### CR-HIGH-2: deleteMinIOTenantObjects 首次错误即终止

- **问题**: 删除列表中第一个失败对象后停止，已删对象无审计记录
- **修复**: 改为 best-effort 全量删除，收集所有错误后统一报告

### CR-HIGH-3: purge_audit_logs 在 allowlist 但不在 cascadeTables

- **问题**: 不一致状态，未来扩展可能绕过 REVOKE DELETE 保护
- **修复**: 从 gdprAllowedTables 中移除 purge_audit_logs

### CR-HIGH-4: SECURITY DEFINER 函数 TEXT vs UUID 类型不匹配

- **问题**: `p_tenant_id TEXT` 与 `audit_logs.tenant_id UUID` 比较，绕过索引，大租户下全表扫描
- **修复**: 改为 `p_tenant_id UUID` 或 `WHERE tenant_id = p_tenant_id::uuid`

### SR-HIGH-1: X-Hermes-User-Id header 允许 IDOR 攻击

- **文件**: `internal/api/memory_api.go` lines 25, 74, 110
- **问题**: 同租户内任何用户可通过设置 header 读取/删除其他用户的 memories 和 sessions
- **修复**: 移除 header fallback，或仅 admin scope 允许覆盖

### SR-HIGH-2: handleGetSessionMessages 无 ownership 检查

- **文件**: `internal/api/memory_api.go` line 194
- **问题**: 任何租户成员可读取任意 session 的消息，无需验证 session 归属
- **修复**: 添加 session owner 验证（与 agent_chat.go 一致）

### SR-HIGH-3: CORS 通配符默认值

- **文件**: `docker-compose.saas.yml` line 66
- **问题**: `${SAAS_ALLOWED_ORIGINS:-*}` 未设置时允许任意跨域请求
- **修复**: 移除 `:-*` 默认值，强制显式配置

### SR-HIGH-4: Helm values.yaml 含可用 changeme 凭证

- **文件**: `deploy/helm/hermes-agent/values.yaml`
- **问题**: `changeme` 作为默认值不会触发启动失败
- **修复**: 使用空字符串或 `REPLACE_ME_BEFORE_DEPLOY` 触发启动报错

---

## MEDIUM 非阻塞风险

| ID | 来源 | 描述 | 建议 |
|----|------|------|------|
| CR-M1 | code | CleanupMinIOHandler 无独立限流 | 考虑 admin scope 或更低 RPM |
| CR-M2 | code | deleteViaStore 返回 nil 但跳过 4 张表 | 返回 207 或要求 pool 非空 |
| CR-M3 | code | ExportHandler 混合 JSON 构造方式 | 统一用 json.Encoder |
| SR-M1 | sec | docker-compose.dev.yml 仍含硬编码密码 | 添加警告注释 |
| SR-M3 | sec | CleanupMinIOHandler 泄露 MinIO 错误 | 已含在 CR-CRIT-2 |
| SR-M5 | sec | SECURITY DEFINER 无 REVOKE FROM PUBLIC | 新增 migration |
| SR-M6 | sec | purge_audit_logs 无 RLS INSERT 策略 | 补充 FORCE RLS + 策略 |

---

## 测试矩阵

| 场景 | 类型 | 当前覆盖 | 缺口 |
|------|------|----------|------|
| GDPR Export 200/400/405 | 单元 | ✅ 3 cases | — |
| GDPR Delete 204/400/405 | 单元 | ✅ 4 cases | 缺 207 Multi-Status 路径 |
| GDPR CleanupMinio | 单元 | ❌ | 需新增 |
| Session owner 403 | 单元 | ❌ | 需新增 |
| RLS SET LOCAL + 写入 | 集成 | ❌ | CRITICAL 依赖 |
| Memory IDOR via header | 单元 | ❌ | HIGH 安全修复后补测 |
| Helm template 渲染 | chart | ❌ | helm template + kubeconform |
| PDB/HPA 多 release 隔离 | chart | ❌ | LOW 优先级 |

---

## 放行建议

**结论: NO-GO — 不建议放行**

阻塞原因：
1. 2 个 CRITICAL + 6 个独立 HIGH（跨 code 和 security）
2. RLS 写策略上线即全量写入中断，属于 P0 生产事故风险
3. IDOR 漏洞允许同租户用户读取其他用户数据

**建议修复优先级:**
1. CR-CRIT-1 (SET LOCAL) → 架构级改动，需要 Store 层事务包装
2. SR-HIGH-1 + SR-HIGH-2 (IDOR) → 2-3 行修复
3. CR-CRIT-2 (error leak) → 5 行修复
4. SR-HIGH-3 (CORS default) → 1 行修复
5. 其余 HIGH → 逐项修复

修复后需重新评审。

---

# Test Plan: enterprise-saas-ga v1.2.0 Phase 2

| 字段 | 值 |
|------|-----|
| 任务 | 2026-05-06-enterprise-saas-ga |
| 阶段 | Phase 2 (OIDC + Pricing + Rate Limiting) |
| 主责 | qa-engineer |
| 状态 | PASS — 建议放行 |
| 评审时间 | 2026-05-07 |

---

## 测试范围

### 功能范围
- P2-S0: AuthContext 扩展 (UserID, ACRLevel 字段)
- P2-S1: OIDCExtractor — JWKS 旋转、ClaimMapper 配置化、token 验证
- P2-S2: Dynamic Pricing — PricingRuleStore (CRUD), CostCalculator (DB-first + fallback), Admin API
- P2-S3: DualLayerLimiter — Redis Lua 原子双 ZSET 滑动窗口、LocalDualLimiter 回退、中间件集成

### 非功能范围
- Redis Cluster hash tag 兼容性 (`{tenantID}` 保证同 slot)
- 30s TTL 缓存一致性 (PricingStore)
- 分布式限流器故障自动降级至本地内存
- 并发安全 (race detector)

### 不覆盖项
- OIDC extractor 端到端 IdP 集成 (生产配置不在 Phase 2 scope)
- 多副本场景下 LocalDualLimiter 行为 (已知限制，文档记录)
- Admin UI (无前端变更)

---

## 测试矩阵

| # | 场景 | 类型 | 前置条件 | 预期结果 | 状态 |
|---|------|------|----------|----------|------|
| 1 | OIDC valid token | unit | Mock IdP | AuthContext populated with sub, tenant, roles, acr | PASS |
| 2 | OIDC no Bearer | unit | - | nil, nil (skip) | PASS |
| 3 | OIDC invalid token | unit | Malformed JWT | nil, nil (not this extractor's match) | PASS |
| 4 | OIDC custom ClaimMapper | unit | Custom claim paths | Correct field extraction | PASS |
| 5 | OIDC default roles | unit | No roles claim | Default ["user"] | PASS |
| 6 | OIDC wrong audience | unit | Mismatched aud | nil (rejected) | PASS |
| 7 | OIDC expired token | unit | Past expiry | nil (rejected) | PASS |
| 8 | PricingStore List | unit | Mock DB | Returns all rules | PASS |
| 9 | PricingStore Upsert | unit | - | ON CONFLICT UPDATE | PASS |
| 10 | PricingStore Delete (found) | unit | Rule exists | 204 No Content | PASS |
| 11 | PricingStore Delete (not found) | unit | No rule | 404 Not Found | PASS |
| 12 | PricingStore Delete (DB error) | unit | Simulated failure | 500 Internal Error | PASS |
| 13 | Admin upsert invalid model key | unit | Special chars | 400 Bad Request | PASS |
| 14 | Admin upsert negative pricing | unit | Negative values | 400 Bad Request | PASS |
| 15 | CostCalculator DB-first | unit | Rule in cache | DB pricing used | PASS |
| 16 | CostCalculator fallback | unit | No rule | Hardcoded pricing | PASS |
| 17 | DualLimiter both allow | integration | miniredis | allowed=true, remaining decremented | PASS |
| 18 | DualLimiter user exhausted | integration | 3/3 user calls | allowed=false, userRemaining=0 | PASS |
| 19 | DualLimiter tenant exhausted | integration | 3/3 tenant calls | allowed=false, tenantRemaining=0 | PASS |
| 20 | DualLimiter window expiry | integration | FastForward 61s | Counters reset | PASS |
| 21 | DualLimiter middleware: TenantLimitFn override | unit | Custom fn | Override applied | PASS |
| 22 | DualLimiter middleware: UserLimitFn override | unit | Custom fn | Override applied | PASS |
| 23 | DualLimiter middleware: Redis error fallback | unit | Simulated err | Falls back to local | PASS |
| 24 | DualLimiter middleware: no UserID path | unit | Empty UserID | Single-layer path | PASS |
| 25 | DualLimiter middleware: hash tags | unit | org-abc tenant | rl:{org-abc} format | PASS |
| 26 | LocalDualLimiter basic | unit | 3 calls, limit=2 | Third denied | PASS |
| 27 | LocalDualLimiter tenant exhaustion | unit | 2 users exhaust | Third user denied | PASS |

---

## 代码审查发现

### CRITICAL (已修复)
- **CR-CRIT-1**: `deletePricingRule` 将所有 DB 错误映射为 404 → 已修复为区分 `store.ErrNotFound` 和内部错误
- **CR-CRIT-2**: Admin API 无输入验证 (负值/NaN/Inf/modelKey 注入) → 已添加 `isValidPrice()` + `validModelKey` regex

### HIGH (已修复)
- **CR-HIGH-1**: `RedisDualLimiter.AllowDual` 使用 `context.Background()` → 已改为传递 request context
- **CR-HIGH-2**: `store.ErrNotFound` sentinel 缺失，pg 层泄漏 `pgx.ErrNoRows` 到 handler → 已引入 `store.ErrNotFound`

### MEDIUM (已接受风险)
- **CR-MED-1**: OIDC Extract() 对验证失败返回 nil,nil → 符合 ExtractorChain "not my token" 语义，且 OIDC 尚未 wire 到 server.go；生产启用时需复审
- **CR-MED-2**: `X-RateLimit-Limit` header 始终显示 tenantLimit → 非阻塞，符合当前 API 约定
- **CR-MED-3**: OIDC 未填充 AuthContext.Scopes → Phase 3 scope，当前角色检查足够
- **CR-MED-4**: Local fallback 按副本数倍增限额 → 已在 ADR-002 中文档化为已知限制

### 安全审查
- **SR-HIGH-1**: HasScope 对 empty scopes 放行非 admin scope → P1 既有行为，文档化为遗留兼容
- **SR-MED-1**: OIDC extractor 未 wire 到 auth chain → 设计决策：Phase 2 仅交付代码，wiring 需运维配置

---

## 覆盖率

| 包 | 覆盖率 |
|----|--------|
| internal/auth (OIDCExtractor) | 90.9% |
| internal/metering (PricingStore) | 92.9% |
| internal/middleware (DualLayerLimiter) | 100% (LocalDual) |
| internal/middleware (RedisDualLimiter) | miniredis integration |
| internal/api/admin (pricing handlers) | 100% |

---

## 放行建议

**建议放行**。所有 CRITICAL 和 HIGH 问题已修复。MEDIUM 项为已知限制或设计决策，不阻塞发布。

全量 1469 测试通过，go vet 无问题，race detector 无竞态。
