# PRD: Hermes Agent Go — Enterprise SaaS GA Hardening

**状态**: Draft  
**主责**: tech-lead  
**日期**: 2026-05-06  
**Slug**: enterprise-saas-ga  
**版本目标**: v1.2.0 GA

---

## 1. 背景

### 1.1 业务问题

hermes-agent-go v1.1.0 综合成熟度评分 6.2/10。核心多租户隔离、LLM 韧性、可观测性基础已达生产级，但在安全合规、计费完整性、运维可运营性三个维度存在明确缺口，导致无法通过企业采购审查。

### 1.2 触发原因

经代码审查（2026-05-06）发现：
- **5 个 CRITICAL 阻断缺口**：RLS 写操作未覆盖、审计日志可被删除、生产默认凭证存在、GDPR MinIO 清理缺失、备份不可验证
- **6 个 HIGH 优先级缺口**：无 OAuth2/OIDC、价格硬编码、无用户级限流、无 provider 独立断路器、无发票/账单、Runbook 缺失
- **9 个 MEDIUM 优先级缺口**：MFA、ABAC、Admin CRUD、API Key 过期策略、Trace-Log 关联、会话粘滞、覆盖率门卡、NetworkPolicy

### 1.3 当前状态评分

| 维度 | 完成度 | 风险 |
|------|--------|------|
| 认证与 API Key | 85% | HIGH — 无 OAuth2/OIDC、无 MFA |
| RBAC 权限模型 | 70% | HIGH — 无资源级 ABAC，权限硬编码 |
| 多租户隔离 | 90% | HIGH — RLS 写操作未覆盖 |
| 计量计费 | 55% | HIGH — 价格硬编码，无分级/发票 |
| 可观测性 | 70% | MEDIUM — Trace-Log 未关联 |
| LLM 韧性 | 75% | MEDIUM — 无 provider 级别独立断路器 |
| GDPR 合规 | 60% | HIGH — MinIO 对象清理缺失 |
| 数据备份 | 30% | HIGH — PITR 脚本为空壳 |
| Secrets 管理 | 40% | HIGH — 默认凭证存在 |
| 审计日志 | 70% | HIGH — 无 BEFORE DELETE 触发器 |
| K8s 部署 | 50% | HIGH — 无 PDB/HPA/NetworkPolicy |

### 1.4 约束条件

- 语言/框架：Go 1.25.9 + PostgreSQL 16 + Redis 7 + MinIO（不变）
- 认证预置：JWT extractor 已存在（`internal/auth/jwt.go`），OAuth2/OIDC 可直接对接
- 部署：Helm chart 已有（`deploy/helm/hermes-agent/`），HPA values 已定义但模板未生成对象
- 无 UI 变更（本次全量后端/基础设施/运维）
- CI 须全绿（go test, gofmt, govulncheck, gosec, trivy）

---

## 2. 目标与成功标准

### 2.1 业务目标

将 hermes-agent-go 从"准生产级（6.2/10）"推进到"企业 GA（8.5+/10）"，满足企业客户安全审查、合规审计和运维 SLA 要求。

### 2.2 用户价值

- **企业 IT 管理员**：可通过 OIDC/SSO 统一管理用户身份，无需额外账号
- **安全合规团队**：有不可篡改审计日志和 GDPR 完整删除链路，通过合规审查
- **财务/采购团队**：有动态定价和月度发票，支持成本核算
- **运维团队**：有完整 Runbook 和 K8s 高可用部署，满足 SLA 要求

### 2.3 成功标准（可量化）

| 标准 | 验收方式 |
|------|---------|
| 5 个 CRITICAL 缺口全部修复 | go test ./... PASS + 安全审查通过 |
| RLS WITH CHECK 覆盖 9 张表 | SQL 查询验证 + 隔离测试通过 |
| audit_logs BEFORE DELETE 触发器生效 | 尝试删除返回 PG 错误验证 |
| docker-compose 无默认凭证 | grep -r "test-secret-key\|hermes_secret" 无命中 |
| GDPR 删除覆盖 MinIO 对象 | 删除租户后 MinIO bucket 对应前缀为空 |
| pitr-drill.sh 执行成功 | RPO < 5min 有实测时间戳记录 |
| K8s PDB + HPA 生效 | kind 集群 kubectl get pdb/hpa 有对象 |
| CI pipeline 全绿 | GitHub Actions 所有 job PASS |
| Playwright E2E 隔离测试通过 | 13/13 PASS |

---

## 3. 用户故事（按优先级）

### P0 — CRITICAL 阻断级

**US-01 RLS 写操作隔离**
作为平台安全工程师，我需要所有写操作（INSERT/UPDATE/DELETE）都受 RLS WITH CHECK 策略约束，以确保应用层 bug 无法绕过租户隔离写入其他租户数据。

验收标准：
- 9 张表（sessions, messages, users, audit_logs, memories, user_profiles, cron_jobs, api_keys, roles）均有 WITH CHECK 策略
- 直接 SQL INSERT 跨租户数据返回 PG policy violation 错误
- `go test ./tests/integration/...` 包含跨租户写入被拒绝的测试用例

**US-02 审计日志不可篡改**
作为合规审计员，我需要 audit_logs 表的记录无法被删除，以满足审计追踪不可篡改要求。

验收标准：
- `BEFORE DELETE` 触发器添加，尝试 DELETE FROM audit_logs 返回错误
- purge_audit_logs 表在 GDPR 删除时被填充（记录"谁在何时删了什么"）
- GDPR 删除路径保留 purge_audit_logs 中的删除元数据

**US-03 无生产默认凭证**
作为 DevOps 工程师，我需要生产环境配置中不存在可被猜测的默认密码/Token，防止因配置疏漏导致安全事件。

验收标准：
- `docker-compose*.yml` 中移除所有 `:-test-secret-key`、`:-hermes_secret`、`:-hermespass` 等 fallback 默认值
- 缺少必要环境变量时服务启动失败并输出明确错误信息
- `.env.example` 中所有敏感值替换为 `CHANGEME_*` 占位符
- `grep -rn "test-secret-key\|hermes_secret\|hermespass" --include="*.yml" .` 无命中

**US-04 GDPR 删除覆盖 MinIO**
作为 GDPR 合规官，我需要删除租户数据时同时清除 MinIO 中的 soul/skills 对象存储，以满足第 17 条数据彻底删除要求。

验收标准：
- `DELETE /v1/gdpr/data` 触发后，MinIO `tenants/{tenantID}/` 前缀下的所有对象被删除
- 若 MinIO 删除失败，整个 GDPR 删除事务回滚并返回错误（不允许静默忽略）
- 集成测试验证 MinIO 清理

**US-05 备份可验证**
作为 SRE，我需要能在 30 分钟内完成 PG 时间点恢复演练，以证明 RPO < 5min、RTO < 1h 承诺可实现。

验收标准：
- `scripts/pitr-drill.sh` 完整实现：init stanza → 全量备份 → WAL 写入 → PITR 恢复 → 验证数据
- 脚本执行后输出 RPO 实测时间（最后 WAL 到恢复点的时间差）
- Prometheus 暴露 `hermes_backup_last_success_timestamp` 指标
- `docs/runbooks/pg-pitr-recovery.md` 补全完整恢复步骤

**US-06 K8s 高可用部署**
作为 K8s 平台工程师，我需要 Hermes 在计划内维护时保持可用，在流量峰值时自动扩容。

验收标准：
- Helm chart 生成 `PodDisruptionBudget`（minAvailable: 1）
- `HorizontalPodAutoscaler` 对象生成（CPU 70% 触发，min 2 max 10）
- `kind` 集群验证：`kubectl get pdb,hpa -n hermes` 有对象
- 滚动更新不中断服务（kubectl rollout 验证）

### P1 — HIGH 优先级

**US-07 OAuth2/OIDC SSO**
作为企业 IT 管理员，我需要通过公司 OIDC Provider（Keycloak/Google/Azure AD）登录，不创建额外账号。

验收标准：
- 支持标准 OIDC Discovery（`/.well-known/openid-configuration`）
- JWT 携带 tenant_id/roles claim 可自动映射
- `HERMES_OIDC_ISSUER` 环境变量配置后生效
- 现有 API Key 认证不受影响

**US-08 动态计费定价**
作为平台管理员，我需要在不发布代码的情况下更新 LLM 模型价格，以应对 provider 调价。

验收标准：
- `pricing_rules` 表存储 (model, provider, input_per_1k, output_per_1k, cache_read_per_1k, effective_from)
- `POST /admin/v1/pricing` 创建/更新价格规则
- 成本计算优先使用 DB 中最新规则，无规则时 fallback 到代码默认值
- 旧计费记录不被追溯修改

**US-09 用户级限流**
作为平台运营，我需要限制单个用户的 API 调用频率，防止租户内个别用户滥用消耗配额。

验收标准：
- Redis rate limiter key 从 `rl:{tenantID}` 扩展为 `rl:{tenantID}:{userID}`
- 租户级和用户级限流同时生效（两层检查）
- `X-RateLimit-Limit/Remaining` 响应头反映用户级配额
- 未认证请求继续使用 IP 限流

**US-10 Provider 独立断路器**
作为 SRE，我需要 Anthropic 故障时不影响 OpenAI 调用，每个 LLM provider 有独立熔断保护。

验收标准：
- 按 provider 名称维护独立 gobreaker 实例（map[string]*gobreaker.CircuitBreaker）
- Anthropic 断路器打开后，直接路由到 OpenAI，不影响 OpenAI 的断路器状态
- Prometheus 指标按 provider 标签区分（`hermes_llm_breaker_state{provider="anthropic"}`）

**US-11 月度账单与发票**
作为企业财务，我需要每月获得结构化账单数据，支持内部成本分摊。

验收标准：
- `GET /admin/v1/tenants/{id}/billing/summary?month=2026-05` 返回月度汇总
- `GET /admin/v1/tenants/{id}/billing/invoice?month=2026-05` 返回可下载 JSON 发票
- 发票包含：租户信息、计费周期、model 明细、总 token 数、总费用 USD

**US-12 运维 Runbook 全集**
作为运维工程师，我需要标准化的故障响应手册，在生产事故时能快速定位和恢复。

验收标准：
- 8 份 Runbook 创建于 `docs/runbooks/`：
  - `incident-response.md`
  - `capacity-planning.md`
  - `security-incident.md`
  - `disaster-recovery.md`
  - `performance-tuning.md`
  - `tenant-migration.md`
  - `upgrade-guide.md`
  - `onboarding.md`
- 每份 Runbook 包含：触发条件、响应步骤、验证命令、升级联系人

### P2 — MEDIUM 优先级

**US-13 MFA/2FA 支持**
Admin 账户启用 TOTP 两步验证保护。

**US-14 资源级 ABAC**
用户只能访问自己创建的 session/message，不能跨用户访问同租户数据。

**US-15 Admin API 租户 CRUD**
`POST/PATCH/DELETE /admin/v1/tenants` 完整管理端点（含租户恢复）。

**US-16 API Key 强制过期**
默认 90 天过期，30 天前邮件/Webhook 提醒，支持自定义过期时长。

**US-17 Trace-Log 关联**
slog context 中注入 trace_id/span_id，日志与 OTel trace 可关联查询。

**US-18 Cookie 会话粘滞**
Nginx 改用 sticky cookie（SERVERID）替代 ip_hash，K8s 使用 SessionAffinity: ClientIP。

**US-19 CI 覆盖率门卡**
CI 集成 `go test -coverprofile`，覆盖率低于 80% 时阻断合并。

**US-20 K8s NetworkPolicy**
定义 Ingress/Egress 策略：仅允许 hermes → PG/Redis/MinIO/LLM API，拒绝其他出站。

---

## 4. 范围

### In Scope
- 后端 Go 代码修复（internal/）
- PostgreSQL migration 新增（RLS WITH CHECK、触发器、pricing_rules 表）
- Helm chart 更新（PDB、HPA、NetworkPolicy）
- docker-compose 配置清理
- scripts/ 脚本实现（pitr-drill.sh、check_tenant_sql.sh）
- docs/runbooks/ 文档补全（8 份）
- CI pipeline 更新（覆盖率门卡）
- Nginx 配置更新（会话粘滞）

### Out of Scope
- WebUI 前端变更（无 UI 需求）
- Batch RL Training 接入
- 多云/多地域部署
- SOC2/ISO27001 认证流程（仅做技术准备）
- 真实 LLM Provider 费率谈判

---

## 5. 风险与依赖

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|---------|
| RLS WITH CHECK 破坏现有写入路径 | HIGH | MEDIUM | 先在测试库验证，逐表灰度添加 |
| OIDC 集成需要外部 IdP 配合 | HIGH | HIGH | 先实现 Generic OIDC，提供 mock IdP 测试 |
| pitr-drill.sh 在本地 Docker 环境受限 | MEDIUM | HIGH | Docker 模拟 WAL，实测值用 delta 时间戳 |
| 动态定价迁移旧计费记录 | MEDIUM | LOW | 历史记录保留 snapshot cost，不追溯修改 |
| API Key 强制过期影响现有集成 | MEDIUM | MEDIUM | 存量 Key 给 90 天宽限期，不立即过期 |

### 关键依赖
- PostgreSQL 16（RLS WITH CHECK、触发器）— 已满足
- Redis 7（用户级限流 key 扩展）— 已满足
- MinIO SDK（GDPR MinIO 清理）— `minio-go/v7` 已在 go.mod
- gobreaker v2.4.0（provider 独立断路器）— 已在 go.mod
- Helm v3（PDB/HPA 模板）— 已有 chart

---

## 6. 待确认项

### 技术待确认
- [ ] OIDC Provider 目标：Keycloak / Azure AD / Google / 通用 OIDC（需业务侧确认）
- [ ] API Key 过期宽限期：90 天是否合适，存量 Key 迁移策略
- [ ] 用户级限流默认配额：多少 RPM/user（建议与租户级相同默认值 60 RPM）
- [ ] 发票格式：JSON 是否足够，还是需要 PDF（影响工作量 3-5 天）
- [ ] MFA 方案：TOTP（Google Authenticator 兼容）还是 WebAuthn/FIDO2

### 运维待确认
- [ ] pgBackRest 演练环境：本地 Docker 模拟 vs 真实 staging 环境
- [ ] K8s 验证集群：kind 本地 vs 云端（影响 HPA 自动扩容测试）
- [ ] Runbook 语言：中文 vs 英文（当前文档混合）

### 企业治理待确认
- [ ] 应用等级评定：多租户 SaaS 平台建议 T2（高可用 + 多实例），需业务侧确认
- [ ] 数据合规范围：是否涉及跨境数据传输（影响 GDPR 实现深度）
- [ ] 密钥管理：env 变量是否满足合规要求，还是需要 Vault/KMS 集成

---

## 7. 参与角色

| 角色 | 职责 |
|------|------|
| tech-lead | 整体方案决策、CRITICAL 缺口优先级裁定、上线门禁 |
| architect | RLS 写操作设计、OIDC 集成架构、provider 断路器设计、K8s 高可用方案 |
| backend-engineer | Go 代码实现（RLS migration、GDPR MinIO、限流、断路器、计费） |
| devops-engineer | pitr-drill.sh、Helm chart PDB/HPA、NetworkPolicy、Runbook 编写 |
| qa-engineer | 集成测试补全、覆盖率门卡、GDPR 删除验证、K8s 部署验证 |
| security-reviewer | 默认凭证审查、审计日志不可篡改验证、OIDC 安全配置 |

---

## 8. 需求挑战会候选分组

### 分组 A — 安全合规（CRITICAL 优先）
**成员**: tech-lead + architect + security-reviewer  
**议题**:
- RLS WITH CHECK 是否需要 FOR ALL / FOR INSERT / FOR UPDATE / FOR DELETE 分别定义
- 审计日志不可篡改触发器是否影响 GDPR 删除路径（需要 purge_audit_logs 方案）
- 默认凭证移除后的 CI 环境变量注入方案

### 分组 B — 商业能力（计费/OIDC）
**成员**: tech-lead + architect + backend-engineer  
**议题**:
- pricing_rules 表设计：版本管理、生效时间、fallback 策略
- OIDC claim 到 TenantID/Roles 的映射规则
- 发票 JSON vs PDF 决策

### 分组 C — 运维可靠性（K8s/备份）
**成员**: tech-lead + devops-engineer + qa-engineer  
**议题**:
- HPA 触发指标：CPU 还是自定义指标（hermes_http_requests_in_flight）
- pitr-drill.sh 测试环境约束与 RPO 测量方法
- Runbook 覆盖的 8 个场景优先级排序

---

## 9. 工作量估算

| Phase | 内容 | 估算工作量 |
|-------|------|-----------|
| Phase 1 — CRITICAL 修复 | US-01~06 | 5-7 天 |
| Phase 2 — 商业完整性 | US-07~12 | 10-14 天 |
| Phase 3 — 能力完善 | US-13~20 | 7-10 天 |
| **总计** | **20 个 Story** | **22-31 天** |

---

## 10. 企业治理补充

- **应用等级建议**: T2（多实例、高可用、跨机房就绪）
- **技术架构等级**: 标准 SaaS 多租户架构，符合集团 PAAS 分层约束
- **合规相关**: GDPR 第 17 条（被遗忘权）、审计追踪不可篡改（SOC2 CC7.2）
- **关键组件偏离**: 使用 MinIO 替代集团 OSS（需 ADR 记录）
- **资产文档**: 待补全 `docs/artifacts/2026-05-06-enterprise-saas-ga/arch-design.md`

---

*当前阶段*: `intake`  
*目标阶段*: `design-review`  
*就绪状态*: `not-ready` — 待需求挑战会收口待确认项
