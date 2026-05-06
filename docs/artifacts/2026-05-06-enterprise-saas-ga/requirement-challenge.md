# 需求挑战会结论 — Enterprise SaaS GA Hardening

**日期**: 2026-05-06  
**参与**: 分组 A（安全合规）/ 分组 B（商业能力）/ 分组 C（运维可靠性）  
**主持**: tech-lead  
**状态**: CLOSED — 全部待确认项已收口

---

## Tech-Lead 决策记录

在进入详细挑战结论之前，记录所有升级到 tech-lead 的决策：

| 编号 | 问题 | 决策 | 原因 |
|------|------|------|------|
| D-01 | audit_logs DELETE 权限与 GDPR 合规 | **选 A**：剥夺应用角色 DELETE 权限，GDPR 删除走 `SECURITY DEFINER` 函数 | 不可篡改保证必须在数据库层强制，触发器可被 table owner 绕过不够 |
| D-02 | MFA/TOTP 实现位置 | **IDP 层 ACR claim 校验**（不在应用层自建 TOTP） | 应用层 TOTP 需 2-3 周，且引入 secret 存储风险；OIDC ACR claim 校验 1-2 天 |
| D-03 | `app.current_tenant` 设置机制 | **`SET LOCAL`**，所有写路径必须包在事务中 | session 级 `SET` 在 pgBouncer transaction mode 下有污染风险 |
| D-04 | docker-compose.saas.yml 定位 | **纳入生产基线**，必须清除默认值，加启动校验 | compose 文件存在被复制到生产的真实风险，不能只加注释 |
| D-05 | OIDC 多 IdP 支持（GA 阶段） | **单 IdP 限制**，明确写入 release notes | per-tenant IdP 映射工作量大，GA 先做单 IdP，v1.3.0 再扩展 |
| D-06 | 用户级限流 user 语义（api_key 场景） | **绑定 key ID**（`rl:{tenantID}:key:{keyID}`），文档说明语义 | 不引入 api_key.user_id 字段避免 schema breaking change |
| D-07 | 账单 API 鉴权 scope | **复用 `admin` scope**，GA 阶段不新增 `billing:read` | scope 体系扩展留 v1.3.0，避免 GA 阶段权限模型碎片化 |
| D-08 | invoice 不可变性 | **实时计算**，响应体标注 `"type":"realtime"` | `invoices` 表和状态机工作量 +3 天，v1.2.0 不做 |
| D-09 | ChatStream breaker 统计缺失 | **US-10 附带必须修复**，不作为独立 bug fix | 进程级 provider breaker 重构时必须一并修复，否则 breaker 统计永远不准 |

---

## 分组 A 挑战结论 — 安全合规

### US-01 RLS WITH CHECK 补全

**挑战发现**：
1. 现有 9 张表 RLS 策略只有 `USING` 子句，无 `WITH CHECK`，UPDATE 可将 `tenant_id` 改写为其他租户
2. `current_setting('app.current_tenant', true)` 的 `true` 参数在 GUC 未设置时返回 NULL，INSERT WITH CHECK 行为需明确（PG 实际拒绝 NULL 结果，但不够显式）
3. `audit_logs` 策略存在 `OR tenant_id IS NULL` 例外，写操作不能沿用此例外
4. `usage_records` 和 `purge_audit_logs` 两张表无 RLS，存在横向访问风险
5. Migration 中无 `CREATE ROLE app_role` + `FORCE ROW LEVEL SECURITY`，table owner 连接绕过所有 RLS

**挑战结论**：修改

**落地约束**：
- 所有 9 张表改用 `current_setting('app.current_tenant', false)`（GUC 未设置则报错，强制应用层主动 SET）
- 按 `FOR SELECT / FOR INSERT / FOR UPDATE / FOR DELETE` 分别定义策略，`audit_logs` 的 `OR tenant_id IS NULL` 仅保留在 `FOR SELECT`
- Migration 中补 `ALTER TABLE ... FORCE ROW LEVEL SECURITY`（对 table owner 也生效）
- `usage_records` 补 RLS 或通过 GRANT 限制为只写不读（metrics writer 角色）
- 所有写操作路径（store 层）必须包在事务中，确保 `SET LOCAL app.current_tenant` 生效范围正确

**新增前置 Migration 要求**：
```sql
-- 必须在 WITH CHECK 补全之前执行
ALTER TABLE sessions FORCE ROW LEVEL SECURITY;
ALTER TABLE messages FORCE ROW LEVEL SECURITY;
-- ... 所有 9 张表
```

---

### US-02 审计日志不可篡改

**挑战发现**：
1. 触发器无法防止 table owner 执行 `DISABLE TRIGGER + DELETE`，需要同时剥夺应用角色的 DELETE 权限
2. GDPR 删除路径（`deleteViaTx`）直接 `DELETE FROM audit_logs`，与不可篡改存在语义冲突
3. `purge_audit_logs` 表本身无 DELETE 保护，攻击者可清空证据链
4. `audit_logs` 的 `pgAuditLogStore.Append` 使用 pool 连接，`SET LOCAL` 不生效，需确认 RLS 策略中 `app.current_tenant` 如何与 audit 路径协同

**挑战结论**：修改

**落地约束（tech-lead D-01 决策执行）**：
- 应用角色 `REVOKE DELETE ON audit_logs FROM hermes_app`
- GDPR 删除 audit_logs 通过 `SECURITY DEFINER` 存储函数执行，函数内用有特权的角色绕过限制
- `purge_audit_logs` 同时 `REVOKE DELETE, UPDATE`，只允许 INSERT
- 触发器函数用 `SECURITY DEFINER` + `SET search_path = pg_catalog, public`（防 search_path 注入）
- 触发器在 GDPR 删除时填充 `purge_audit_logs`（记录 deleted_by、tenant_id、record_count、reason="GDPR_DELETE"）

**验收**：
```sql
-- 应用角色尝试删除返回 permission denied
SET ROLE hermes_app;
DELETE FROM audit_logs WHERE id = 1;  -- 预期: ERROR: permission denied
```

---

### US-03 移除生产默认凭证

**挑战发现**：
- `docker-compose.saas.yml` 完全硬编码（非 `${VAR:-default}` 格式），风险最高
- `docker-compose.quickstart.yml` 使用 `${VAR:-dev-xxx}` 格式，稍好但仍有默认值
- `deploy/pitr/docker-compose.pitr.yml` 同样存在硬编码

**挑战结论**：接受，扩展范围（D-04 决策：纳入生产基线）

**落地约束**：
- `docker-compose.saas.yml` 所有密码 env var 改为 `${VAR}` 无默认值格式（compose 启动时若未设置直接报错）
- `cmd/hermes/main.go` 或 `config.go` 启动时增加弱密码检测：若 `DATABASE_URL` 包含 `:hermes@` 且非 `localhost`/`127.0.0.1`，拒绝启动并输出明确错误
- `.env.example` 所有密码替换为 `CHANGEME_STRONG_PASSWORD_HERE` 占位符，加注释说明生成命令（`openssl rand -hex 32`）
- `HERMES_ACP_TOKEN` 增加长度最低要求校验（≥ 32 字符），防止弱 token 进入生产
- 提供 `make dev-env` 命令自动生成随机值到 `.env.local`（不提交 git）

---

### US-04 GDPR MinIO 删除链路补全

**挑战发现**：
1. `deleteViaTx`（生产路径）完全没有 MinIO 删除调用，仅删 PG 数据
2. `TenantCleanupJob.purgeTenant` 有 MinIO 清理但是异步软删除清理，与 GDPR 即时删除语义不同
3. Soul 文件存储路径需确认是否都在 `tenantID/` 前缀下（`prompt.go` 中有本地路径读取逻辑）
4. `deleteViaStore` fallback 路径有已知警告（`user_profiles` 等表无法删除），不满足 GDPR 要求

**挑战结论**：修改，优先级提升

**落地约束**：
- `deleteViaTx` PG 事务 commit 后，异步调用 MinIO 删除（`ListObjects(ctx, tenantID+"/")` + 逐对象删除）
- MinIO 删除失败**不 rollback PG 事务**，但必须写入 `purge_audit_logs`（记录 `minio_cleanup_status=failed` + error）并在 HTTP 响应中返回 `207 Multi-Status`（不返回 204）
- 提供幂等重试端点：`POST /v1/gdpr/cleanup-minio`（管理员调用，清理 MinIO 遗留对象）
- `deleteViaStore` fallback 路径在 pool 为 nil 时改为返回 `503`（不允许在无 pool 的情况下假装成功）
- 验证 soul 文件前缀：搜索代码确认所有 MinIO 写入路径，确保 ListObjects 前缀可以覆盖全部

---

### US-05 pitr-drill.sh 实现

**挑战发现**：
1. 脚本基本结构正确（5 步骤），但存在 4 个真实 bug：
   - 数据卷清空命令不可靠（inspect fallback 可产生 false positive）
   - `sleep 3` 不足以等待 pgbackrest 归档完成
   - 缺少 `recovery.signal` 文件验证
   - 缺少 RPO delta 时间差计算输出

**挑战结论**：修改

**落地约束**：
- 数据卷清空改为 `docker exec` 方式，移除不可靠的 volume inspect fallback
- `sleep 3` 改为轮询 `pgbackrest info` 状态，最多等待 30s
- restore 后验证 `recovery.signal` 文件存在
- 增加 RPO delta 输出（INSERT 时间戳 vs 恢复后最新行时间戳的差值）
- 增加 `--dry-run` 模式（只做备份不做 restore，供 CI 日常验证）

---

### US-13 MFA/TOTP

**挑战发现**：
- 代码库完全无 TOTP 实现
- 应用层自建 TOTP 需要 2-3 工程师周，且引入 secret 存储、备用码、时间漂移等独立安全决策点
- 更轻量方案：OIDC Provider 层强制 MFA，应用层只需检查 JWT `acr` claim

**挑战结论**：升级 → tech-lead 决策 → **IDP 层 ACR claim 方案（D-02）**

**落地约束**：
- `OIDCExtractor` 验证 JWT 时，对 `/admin/*` 路由检查 `acr` claim 包含 `"mfa"` 或 `amr` 包含 `"otp"`
- 非 OIDC 认证路径（API key）的 admin 路由豁免 MFA 要求（API key 自身已是高熵凭证）
- 提供 OIDC Provider 配置文档（如何为 Keycloak/Authentik 配置 MFA 策略，provider 无关步骤）
- 验收：携带无 MFA ACR 的 OIDC token 访问 `/admin/*` 返回 403

---

## 分组 B 挑战结论 — 商业能力

### US-07 通用 OIDC SSO

**挑战发现**：
1. 现有 `JWTExtractor` 是静态 RSA 公钥方案，无 JWKS 轮换能力，**不能直接复用**
2. alg 不匹配时当前返回 `(nil, nil)` 而非 error，在 OIDC 场景下可能静默降级到 API key 认证
3. `tenant_id` / `roles` claim 名称不同 IdP 不一样，需要可配置映射
4. `ExtractorChain` 顺序需要明确（OIDC 放在 API key 之前）

**挑战结论**：修改（不复用现有 JWTExtractor，新建独立 OIDCExtractor）

**落地约束**：
- 引入 JWKS 动态 fetch（`github.com/lestrrat-go/jwx/v3` 或 `coreos/go-oidc/v3`），TTL 5min，background refresh
- alg 不匹配时返回 `error`（非 nil），防止静默降级
- claim 映射通过环境变量配置：`HERMES_OIDC_TENANT_CLAIM`（默认 `tenant_id`）、`HERMES_OIDC_ROLES_CLAIM`（默认 `roles`）
- GA 阶段单 IdP 限制明确写入 release notes（D-05 决策）
- `OIDCExtractor` 实现 `auth.CredentialExtractor` 接口，注入 `ExtractorChain` 在 API key extractor 之前

**接口约定**：
```go
type OIDCExtractor struct {
    issuerURL  string
    claimMap   OIDCClaimMap
    keySet     jwx.JWKSet  // dynamic, cached 5min
}

type OIDCClaimMap struct {
    TenantID string // env: HERMES_OIDC_TENANT_CLAIM, default: "tenant_id"
    Roles    string // env: HERMES_OIDC_ROLES_CLAIM, default: "roles"
    Subject  string // default: "sub"
}
```

---

### US-08 动态计费定价

**挑战发现**：
1. 存在**两套并行价格表**：`metering/cost.go`（per-1K）和 `agent/pricing.go`（per-million），单位不一致
2. `UsageRecord.CostUSD` 在 flush 时计算，价格变更期间的 in-flight 记录语义需明确
3. Admin API 需要 list 接口（`GET /admin/v1/pricing-rules`），不应遗漏
4. `model_key` 格式（是否带 provider 前缀）需与 `UsageRecord.Model` 字段对齐

**挑战结论**：修改

**落地约束**：
- 合并两套价格表为单一来源（`metering/cost.go` 为权威，`agent/pricing.go` 迁移到动态查询）
- `pricing_rules` 表字段：`model_key TEXT PRIMARY KEY, input_per_1k NUMERIC, output_per_1k NUMERIC, cache_read_per_1k NUMERIC, updated_at TIMESTAMPTZ`
- 价格缓存 TTL 30s，启动时预热，热路径不同步查库
- `model_key` 格式统一为 `UsageRecord.Model` 的现有格式（无 provider 前缀）
- 价格变更不溯及既往（已写入的 `cost_usd` 不重算）
- 新增 list 接口

**接口约定**：
```
GET    /admin/v1/pricing-rules              → 200 [{model_key, input_per_1k, output_per_1k, cache_read_per_1k, updated_at}]
PUT    /admin/v1/pricing-rules/{model_key}  → 200 upsert
DELETE /admin/v1/pricing-rules/{model_key}  → 204 (回落代码默认值)
```

---

### US-09 用户级限流

**挑战发现**：
1. `AuthContext.Identity` 在 api_key 认证时是 key ID 不是 user ID，"一 user 多 key"可绕过用户级限流
2. 当前 tenant 级 key 变更后旧 key TTL 61s 内仍有效（双 key 并存期间 tenant 限流实质失效）
3. 两层检查（tenant + user）建议合并为单个 Lua script 保证原子性
4. Prometheus 指标不能加 `user_id` label（高基数风险）

**挑战结论**：修改

**落地约束（D-06 决策执行）**：
- api_key 认证时 key 格式：`rl:{tenantID}:key:{keyID}`，文档说明语义（非真 user 级）
- JWT/OIDC 认证时 key 格式：`rl:{tenantID}:user:{userID}`
- **两层都检查**（tenant 作为硬上限 + user/key 作为细粒度），任一超限返回 429
- Lua script 扩展为原子性双 key 检查
- `X-RateLimit-Remaining` 返回 `min(tenant_remaining, user_remaining)`
- `RateLimitConfig.UserLimitFn` 为 nil 时退化为 tenant-only（向后兼容）

---

### US-10 Provider 独立断路器

**挑战发现**：
1. 现有 breaker name 是 `"llm-" + model`（模型级），不是 provider 级
2. 每个 `Client` 创建时 new 独立 breaker 实例，不在进程级共享，provider-wide 熔断无效
3. `ChatStream` 不更新 breaker 统计（已知 bug，必须同步修复 D-09）

**挑战结论**：修改（D-09：ChatStream 统计修复为 US-10 附带必须项）

**落地约束**：
- 引入进程级 `ProviderBreakerRegistry`（`sync.Map`），Server 启动时初始化
- breaker 粒度从 model 改为 provider（`"llm-provider-" + providerName`）
- `ChatStream` 完成/失败后更新 breaker 统计（`registry.Execute` 包装或手动更新计数）
- 新增 Prometheus gauge：`hermes_circuit_breaker_state{provider}` 0=closed, 1=half-open, 2=open

**接口约定**：
```go
type ProviderBreakerRegistry struct {
    mu       sync.Mutex
    breakers map[string]*gobreaker.CircuitBreaker[any]
}
func (r *ProviderBreakerRegistry) Get(providerName string) *gobreaker.CircuitBreaker[any]
```

---

### US-11 月度账单 JSON API

**挑战发现**：
1. `tenant_id` 来源需统一：admin 查账单必须从路径参数 `{id}` 取，不从 context 取，防止越权
2. `UsageStore` 接口不能直接改（会破坏所有 mock），需新增 `BillingStore` 接口扩展
3. `usage_records` 大表月度 SUM 无物化视图，GA 阶段数据量有限但需在 runbook 中说明

**挑战结论**：修改（D-07: admin scope，D-08: realtime 语义）

**落地约束**：
- 挂载在 `AdminHandler`，`RequireScope("admin")` 鉴权
- `tenant_id` 只从路径参数取
- 响应体标注 `"type":"realtime"`，API 文档说明无不可变保证
- 新增 `BillingStore` 接口扩展（不改现有 `UsageStore`）
- 两个端点均支持 `?year=YYYY&month=MM` 参数，默认当月

**接口约定**：
```
GET /admin/v1/tenants/{id}/billing/summary?year=2026&month=05
→ 200 {tenant_id, period, total_input_tokens, total_output_tokens, total_cost_usd, record_count, type:"realtime"}

GET /admin/v1/tenants/{id}/billing/invoice?year=2026&month=05
→ 200 {tenant_id, period, line_items:[{model, input_tokens, output_tokens, unit_price_per_1k, subtotal_usd}], total_cost_usd, currency:"USD", type:"realtime", generated_at}
```

---

### US-12 运维 Runbook 全集

**挑战发现**：
- Runbook 内容依赖 US-07~US-11 接口锁定，提前输出会过期
- `docs/runbooks/` 现有文件：`pg-pitr-recovery.md`（需更新）

**挑战结论**：修改（阻塞在 US-07~US-11 接口锁定后）

**落地约束**：
- US-07~US-11 实现完成、接口契约锁定后，再开始 Runbook 写作
- 8 份 Runbook 清单（按优先级）：
  1. `incident-response.md`（P1，生产故障第一响应）
  2. `upgrade-guide.md`（P1，滚动升级步骤）
  3. `pg-pitr-recovery.md`（更新现有文件）
  4. `security-incident.md`（P1，安全事件处置）
  5. `tenant-migration.md`（P2，租户迁移流程）
  6. `performance-tuning.md`（P2，性能调优指南）
  7. `capacity-planning.md`（P2，容量规划）
  8. `onboarding.md`（P2，新工程师入职）
- 每份 Runbook 最小字段：触发条件、前置条件、步骤命令（附预期输出）、回滚步骤、Owner

---

## 分组 C 挑战结论 — 运维可靠性

### US-06 K8s PDB + HPA（已在分组A/B中覆盖背景，此处补充运维细节）

**挑战发现**：
1. `replicaCount: 1` 时 PDB `minAvailable: 1` 会阻止节点驱逐，`kubectl drain` 卡住
2. kind 默认无 metrics-server，HPA 会永远显示 `<unknown>` metric
3. HPA 在 kind 资源紧张环境下可能震荡

**挑战结论**：接受，加条件化约束

**落地约束**：
- PDB 条件化渲染（仅 `replicaCount >= 2` 时生成）
- kind 验证脚本先安装 metrics-server（`--kubelet-insecure-tls`）
- 测试用 values：`replicaCount: 2`，HPA targetCPU: 50%（便于触发）
- HPA minReplicas ≥ PDB minAvailable（values schema 注释约束）

---

### US-14 资源级 ABAC（降级为安全 Bug Fix）

**挑战发现**：
- `agent_chat.go:44-58` 在 session 已存在时直接使用，未校验 `sess.UserID == ac.Identity`
- 同租户不同用户可通过猜测 sessionID 访问他人会话
- 修复极其简单：Get 成功后加一次 Identity 比对，改动 < 5 行

**挑战结论**：接受，**不降级，定义为安全 Bug Fix**，改动量极小，必须 v1.2.0 修复

**落地约束**：
- `agent_chat.go`：`sess.UserID != "" && sess.UserID != userID` → 403
- `chat_handler.go` 消息列表路径同样补 owner 校验
- admin 角色豁免此校验
- 补 1 个表驱动集成测试：同租户跨用户访问 session 期望 403

---

### US-15 Admin API 租户 CRUD（路由迁移，非重复实现）

**挑战发现**：
- `internal/api/tenants.go` 已实现 `/v1/tenants` 完整 CRUD（含 admin 鉴权）
- `AdminHandler` 缺少 tenant 路由入口，US-15 本质是路由迁移 + PATCH 语义补充

**挑战结论**：接受，定义为路由迁移（不重复实现业务逻辑）

**落地约束**：
- `admin/handler.go` 注册 `/admin/v1/tenants` 路由，复用 `TenantHandler` 的 store 调用
- 补充 `PATCH` 语义（store 层 `PartialUpdate` 或 COALESCE 模式）
- 原 `/v1/tenants` 保持向后兼容，打 `Deprecated: use /admin/v1/tenants` 注释
- 新增 `DELETE /admin/v1/tenants/{id}/restore`（软删除恢复端点）

---

### US-16 API Key 强制过期（拆分）

**挑战结论**：拆分 — **90天默认 TTL v1.2.0，30天主动提醒降级 v1.3.0**

**落地约束（v1.2.0）**：
- `createAPIKey` handler 强制 `ExpiresAt = now() + 90d`
- 请求体可通过 `expires_in_days`（1-365）覆盖，不传则使用默认值
- List API key 响应增加 `expires_soon: true`（`ExpiresAt < now() + 30d`）
- 补 unit test：创建 key 后校验 `ExpiresAt` 不为 nil 且在 90d 内

**v1.3.0（通知渠道建立后）**：
- 定时 job 扫描临近过期 key，触发 webhook/email

---

### US-17 Trace-Log 关联

**挑战发现**：
- `TracingMiddleware` 正确创建 span 但未注入 slog context
- `LoggingMiddleware` 未提取 span context 的 trace_id/span_id
- 修复：3 行代码，高价值，零 breaking change

**挑战结论**：接受，改动极小，高价值

**落地约束**：
```go
// LoggingMiddleware 追加：
spanCtx := trace.SpanFromContext(r.Context()).SpanContext()
if spanCtx.IsValid() {
    attrs = append(attrs, "trace_id", spanCtx.TraceID().String())
    attrs = append(attrs, "span_id", spanCtx.SpanID().String())
}
```
- 补 unit test：注入 mock span context，断言 logger 携带 `trace_id` 字段
- OTel 未启用时（noop），`spanCtx.IsValid()` 返回 false，不输出全零 ID

---

### US-18 Cookie 会话粘滞（替代方案）

**挑战发现**：
- `nginx:alpine`（开源版）**不支持** `sticky cookie` 指令（Nginx Plus 专有特性）
- hermes 本身是无状态应用（session 在 PG，状态在 Redis），严格意义上不需要 sticky
- K8s 原生 `SessionAffinity: ClientIP` 更轻量

**挑战结论**：替代方案 — K8s SessionAffinity 替换 ip_hash（Docker Compose 保持 ip_hash + 注释），Cookie 粘滞降级 v1.3.0

**落地约束**：
- Helm chart `service.yaml` 增加条件化 `sessionAffinity: ClientIP`（`values.sessionAffinity.enabled: true`）
- `nginx-lb.conf` 保留 ip_hash，加注释说明这是 local-dev 模式，production 走 K8s Service SessionAffinity
- v1.3.0 评估是否需要 openresty sticky（届时看业务需求）

---

### US-19 CI 覆盖率门卡（分阶段）

**挑战发现**：
- `internal/api` 当前覆盖率 34.4%，`internal/api/admin` 14.1%，立即设 80% 门卡会锁死 CI

**挑战结论**：分阶段 — **v1.2.0 建立可见性，v1.3.0 启用硬阻断**

**落地约束（v1.2.0）**：
- CI 增加 `-coverprofile` 参数，`go tool cover -func` 输出总覆盖率，只做展示不阻断
- 上传 coverage artifact 追踪趋势
- `store/pg` 和 `observability` 排除在门卡之外（需集成测试环境）
- v1.3.0 前目标：api 层 ≥60%，api/admin 层 ≥50%，再启用 80% 硬阻断

---

### US-20 K8s NetworkPolicy（模板准备 v1.2.0，验证 v1.3.0）

**挑战发现**：
- kind 默认 kindnet 不支持 NetworkPolicy 强制执行
- 错误的 NetworkPolicy 规则会导致 pod 完全无法通信，危害大于无规则

**挑战结论**：模板准备 v1.2.0，`enabled: false` 默认关闭，v1.3.0 在 Calico/Cilium 集群验证后启用

**落地约束**：
- Helm chart 添加 NetworkPolicy 模板，`values.networkPolicy.enabled: false`
- 规则定义：Ingress 允许来自 Ingress Controller，Egress 允许 postgres:5432/redis:6379/minio:9000/otel:4317/llm-api:443

---

## v1.2.0 范围最终确认

### 保留 v1.2.0（含调整后）

| US | 原优先级 | 挑战后调整 |
|----|---------|----------|
| US-01 RLS WITH CHECK + FORCE RLS | CRITICAL | 保留，补 5 项额外约束 |
| US-02 审计日志不可篡改 | CRITICAL | 保留，方案改为权限剥夺 + SECURITY DEFINER |
| US-03 移除默认凭证 | CRITICAL | 保留，扩展为启动时弱密码校验 |
| US-04 GDPR MinIO 清理 | CRITICAL | 保留，补充幂等重试端点 + 207 响应 |
| US-05 pitr-drill.sh | CRITICAL | 保留，修复 4 个 bug + RPO delta 输出 |
| US-06 K8s PDB + HPA | HIGH | 保留，加条件化约束 |
| US-07 OIDC SSO | HIGH | 保留，新建独立 OIDCExtractor |
| US-08 动态计费 | HIGH | 保留，合并两套价格表 |
| US-09 用户级限流 | HIGH | 保留，两层检查模式 |
| US-10 Provider 断路器 | HIGH | 保留，附带修复 ChatStream 统计 |
| US-11 账单 API | HIGH | 保留，realtime 语义，admin scope |
| US-12 运维 Runbook | HIGH | 保留，阻塞在 US-07~11 接口锁定后 |
| US-13 MFA ACR claim | MEDIUM | 保留，改为 IDP 层 ACR 校验（成本大幅降低）|
| US-14 资源级 ABAC | MEDIUM | **提升**为安全 Bug Fix，不可推后 |
| US-15 Admin API 租户 CRUD | MEDIUM | 保留，定义为路由迁移 |
| US-16 API Key 90天 TTL | MEDIUM | 保留（提醒功能降级 v1.3.0） |
| US-17 Trace-Log 关联 | MEDIUM | 保留，3 行改动，高价值 |
| US-18 K8s SessionAffinity（替代 Cookie 粘滞）| MEDIUM | 保留替代方案，原始 Cookie 粘滞降级 |
| US-19 CI 覆盖率可见性 | MEDIUM | 保留（硬阻断降级 v1.3.0） |
| US-20 NetworkPolicy 模板准备 | MEDIUM | 保留（验证降级 v1.3.0） |

### 降级到 v1.3.0

| 功能 | 原因 |
|------|------|
| API Key 30天提醒通知 | 通知基础设施未建立 |
| CI 80% 覆盖率硬阻断 | 当前覆盖率 34%，需先提升基线 |
| Cookie 粘滞（nginx sticky） | Nginx 开源版不支持，K8s SessionAffinity 可替代 |
| NetworkPolicy 生产验证 | kind CNI 可靠性不足，需 Calico/Cilium 集群 |
| per-tenant OIDC IdP 映射 | 单 IdP 限制 GA 文档说明 |
| invoice 不可变快照 | `invoices` 表状态机工作量大 |

---

## 挑战会结论汇总

**当前阶段**: `requirement-challenge`  
**目标阶段**: `design-review`  
**就绪状态**: `handoff-ready` — 所有待确认项已收口，接口约定已产出

**已收口的关键待确认项**：
- ✅ OIDC 目标：通用 OIDC，单 IdP，GA 文档说明限制
- ✅ 发票格式：JSON API，realtime 语义
- ✅ PITR 环境：本地 Docker 模拟
- ✅ MFA 方案：IDP 层 ACR claim（非应用层 TOTP）
- ✅ 审计日志与 GDPR 路径：权限剥夺 + SECURITY DEFINER 函数
- ✅ 用户级限流 user 语义：api_key 场景绑定 key ID
- ✅ 账单 scope：admin scope 复用
- ✅ v1.2.0 vs v1.3.0 范围切割：已明确

**下一步**：进入 `/team-plan`，architect 输出 arch-design，devops-engineer 输出 delivery-plan
