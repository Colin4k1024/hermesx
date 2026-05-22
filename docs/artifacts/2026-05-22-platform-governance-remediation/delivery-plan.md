# Delivery Plan: Internal Platform Governance Remediation

> **版本目标**: platform-governance-remediation  
> **Owner**: tech-lead  
> **状态**: executing  
> **日期**: 2026-05-22  
> **规划周期**: 90 天  

---

## 目标

把 HermesX 从"功能可用的多租户 SaaS 实现"推进到"内部平台治理可证明、可审计、可回滚"。

本计划基于三个已确认决策：

1. MySQL 是企业生产受支持后端，不能只借 PostgreSQL RLS 作为企业隔离基线。
2. 跨租户共享学习允许存在，但必须显式治理、审计、禁用和回滚。
3. 当前优先级面向内部平台治理，先解决平台管理员控制、审计、分权、变更治理和资源治理。

## 重新定级

| 优先级 | 主题 | 为什么现在是这个级别 |
|--------|------|----------------------|
| P0 | 共享学习受控化 | 共享不是缺陷，但不可治理的共享会成为全局污染源。必须补租户开关、共享级别、审计、撤回。 |
| P0 | MySQL 与 PostgreSQL 企业能力对齐 | `DATABASE_DRIVER` 已允许切换后端，MySQL 已有完整 Store 入口；生产支持必须有隔离、审计、删除、备份恢复证明。 |
| P1 | 分域权限模型 | 当前 Admin API 统一 `RequireScope("admin")`，RBAC 仍有 admin bypass，不适合内部平台日常分权。 |
| P1 | 控制面契约漂移治理 | usage、execution receipts、OpenAPI、文档存在实现与契约不一致，会降低平台运维接入可信度。 |
| P2 | 外部合规强化 | 防抵赖链式审计、外部客户删除叙事仍有价值，但应排在平台控制中心建设之后。 |

## 证据

| 证据 | 文件 | 影响 |
|------|------|------|
| 架构文档仍把多租户隔离绑定到 PostgreSQL RLS | `ARCHITECTURE.md:81-91`, `ARCHITECTURE.md:350-356` | 与 MySQL 生产支持决策冲突。 |
| 配置允许通过 `DATABASE_DRIVER` 切换后端 | `internal/config/config.go:61-64`, `internal/config/config.go:207-213` | 后端能力矩阵必须覆盖 PostgreSQL 和 MySQL。 |
| MySQLStore 已注册并实现 Store 主接口 | `internal/store/mysql/mysql.go:13-100` | MySQL 不是概念性后端，需补企业证明和回归矩阵。 |
| Admin API 全部包在 `RequireScope("admin")` 下 | `internal/api/admin/handler.go:81-130` | 平台运维、安全审计、成本、租户管理职责混在一起。 |
| RBAC 仍保留 `admin` 全局 bypass | `internal/middleware/rbac.go:48` | 日常 admin 拥有过宽权限。 |
| 跨租户 usage 聚合关闭 RLS | `internal/api/admin/usage.go:20-36`, `internal/api/admin/usage.go:87-95` | 平台统计依赖高权限数据库角色，不适合作为治理基线。 |
| `/v1/usage` 主路由仍挂旧 handler | `internal/api/server.go:121-124`, `internal/api/usage.go:11-59`, `internal/api/usage_v2.go:13-44` | 增强版 usage 实现未成为主契约。 |
| Execution receipts 主路由只开放查询 | `internal/api/server.go:121-122`, `internal/api/execution_receipts.go:21-32` | 文档若仍含 create 语义，需要修正。 |
| Evolution store 仍是可选全局学习路径 | `internal/evolution/config.go:3-22`, `internal/evolution/store.go:35-88`, `internal/api/chat_handler.go:47-56` | 需要显式共享策略和审计模型，而不是隐式全局行为。 |

## 30 天计划

### 1. 企业后端支持矩阵

**目标**：定义 PostgreSQL 和 MySQL 的生产支持标准，不再用 PostgreSQL 能力替 MySQL 背书。

**落点文件**：
- `docs/database.md`, `docs/database.en.md`
- `ARCHITECTURE.md`, `SECURITY_MODEL.md`
- `internal/store/store.go`
- `internal/store/pg/*`, `internal/store/mysql/*`
- `tests/integration/*`

**交付物**：
- 新增后端支持矩阵：隔离、审计、删除、备份、恢复、统计、压测、迁移、故障降级。
- 标记每项状态：supported、partial、unsupported、requires-control-plane。
- 明确 MySQL 隔离基线：应用层 tenant 参数、Store 静态校验、跨租户攻击回归、审计补偿。

**验收标准**：
- 每个 Store 子接口在 PostgreSQL/MySQL 下都有一致性测试计划。
- 文档不再写"SaaS 必须 PostgreSQL"作为唯一生产路径，除非同步降级 MySQL 支持级别。

### 2. 共享学习治理基线

**目标**：把 evolution 共享从"启用后自然共享"改为"策略驱动共享"。

**落点文件**：
- `internal/evolution/config.go`
- `internal/evolution/store.go`
- `internal/evolution/improver.go`
- `internal/api/chat_handler.go`
- `internal/store/types.go`
- `internal/store/pg/migrate.go`
- `internal/store/mysql/migrate.go`

**交付物**：
- 定义共享级别：`disabled`、`anonymous`、`trusted`。
- 定义共享记录字段：source tenant、source session、task class、content label、sharing level、review status、hit count、revoked at。
- 定义租户策略：贡献开关、消费开关、敏感租户 opt-out、标签黑白名单。

**验收标准**：
- 默认策略保守：未配置租户不贡献可信共享。
- 每次共享写入都有审计事件。
- 可以按租户、时间窗、标签撤回共享知识。

### 3. Admin 权限拆分设计

**目标**：把 `admin` 从日常角色改为 break-glass 或兼容角色。

**落点文件**：
- `docs/RBAC_MATRIX.md`
- `internal/middleware/rbac.go`
- `internal/middleware/scope_check.go`
- `internal/api/admin/handler.go`
- `internal/auth/context.go`
- `internal/auth/apikey.go`
- `internal/auth/oidc.go`
- `internal/store/pg/roles.go`
- `internal/store/mysql/roles.go`

**交付物**：
- 最小角色集：`platform_admin`、`tenant_admin`、`auditor`、`security_admin`、`billing_admin`、`ops_admin`、`break_glass_admin`。
- 统一资源动作模型：tenant、audit、usage、sharing_policy、sandbox_policy、egress_policy、secret_policy、gdpr、key_management、pricing。
- 迁移策略：旧 `admin` 作为兼容 alias，逐步收敛到分域角色。

**验收标准**：
- Admin API 不再只有一个 `RequireScope("admin")` 外壳。
- OIDC claim、API key scopes、RoleStore 权限语义合并到同一套 evaluator。

## 60 天计划

### 4. MySQL 企业证明落地

**目标**：让 MySQL 生产支持从"接口实现"变成"可证明支持"。

**落点文件**：
- `internal/store/mysql/*.go`
- `internal/store/pg/*.go`
- `internal/metering/*`
- `internal/api/gdpr.go`
- `internal/api/execution_receipts.go`
- `internal/api/workflows.go`
- `scripts/check_tenant_sql.sh`
- 新增 `scripts/check_tenant_sql_mysql.sh`

**交付物**：
- 双后端一致性测试矩阵：sessions、messages、memories、audit logs、execution receipts、GDPR、usage records、workflows、cron jobs。
- MySQL 静态 SQL 护栏：所有租户对象读写必须含 tenant 条件或明确 cross-tenant capability。
- 删除证明：MySQL 下 GDPR 删除覆盖与 PostgreSQL 一致。
- 审计证明：MySQL 下 audit logs 查询、保留、删除补偿与 PostgreSQL 一致。

**验收标准**：
- `go test ./internal/store/pg ./internal/store/mysql ./internal/api -count=1` 绿。
- 集成测试可选择 PostgreSQL/MySQL 两种后端运行。
- MySQL 不依赖 PostgreSQL RLS 文档作为隔离证明。

### 5. 受控聚合通道

**目标**：替换 `SET LOCAL row_security = off` 式跨租户统计。

**落点文件**：
- `internal/api/admin/usage.go`
- `internal/api/admin/usage_aggregation.go`
- `internal/metering/store.go`
- `internal/metering/pg_store.go`
- 新增 MySQL usage store 或跨后端 aggregation store

**交付物**：
- 新增平台聚合接口，显式要求 `usage:read:all` 或 `billing_admin`。
- 聚合查询走专用 Store 方法，不在 handler 中直接关 RLS。
- 聚合结果默认脱敏：tenant ID、plan、时间窗、token/cost，不返回用户级明细。

**验收标准**：
- Admin usage 不需要应用 DB 用户拥有 BYPASSRLS/SUPERUSER。
- 所有跨租户聚合都有审计记录，包含操作者、过滤条件、结果规模。

### 6. 共享污染治理实现

**目标**：共享学习可禁用、可审计、可回滚。

**落点文件**：
- `internal/evolution/*`
- `internal/store/types.go`
- `internal/store/pg/migrate.go`
- `internal/store/mysql/migrate.go`
- 新增 `internal/api/admin/sharing.go`
- `internal/api/admin/handler.go`
- `docs/api-reference.md`, `internal/api/openapi.go`

**交付物**：
- 租户级 sharing policy CRUD。
- 共享内容审计查询。
- 撤回 API：按 tenant、label、time window、gene ID 回滚。
- 命中计数和回滚标记。

**验收标准**：
- 敏感租户 opt-out 后不贡献、不消费共享内容。
- 匿名共享不会暴露 source tenant/session 给普通消费路径。
- trusted 共享必须有审核状态。

## 90 天计划

### 7. 平台治理中心

**目标**：把分散的 sandbox、rate limit、egress、usage、共享、保留策略收进统一控制面。

**落点文件**：
- `internal/api/admin/handler.go`
- `internal/api/admin/sandbox.go`
- `internal/egress/admin_handler.go`
- `internal/api/admin/pricing.go`
- `internal/api/admin/secrets.go`
- `internal/api/admin/safety.go`
- `webui/src/admin/*`
- `docs/api-reference.md`
- `internal/api/openapi.go`

**交付物**：
- Tenant Governance Center：策略读取、变更、版本、diff、审批状态。
- 策略变更审计：before/after diff、operator、request ID、approval chain。
- 跨租户只读视图：平台管理员可观察但不可绕过业务权限直接改数据。

**验收标准**：
- 每类策略都有版本号和回滚路径。
- 每次策略变更可在 audit logs 和控制台中追溯。
- 控制台行为与 OpenAPI/API reference 一致。

### 8. 控制面契约收敛

**目标**：修正文档与实现漂移，让运维按正确契约接入。

**落点文件**：
- `internal/api/server.go`
- `internal/api/openapi.go`
- `docs/api-reference.md`, `docs/api-reference.en.md`
- `README.md`
- `docs/CHANGELOG.md`, `docs/CHANGELOG.en.md`
- `docs/RBAC_MATRIX.md`

**交付物**：
- `/v1/usage` 明确采用旧版或 v2，并同步 OpenAPI、API reference、测试。
- execution receipts 文档只描述已开放的 query 语义；create 只保留内部记录语义。
- GDPR、admin usage、sharing policy、governance center 统一进 OpenAPI。

**验收标准**：
- `internal/api/openapi_test.go` 覆盖新增/变更路径。
- README、API reference、OpenAPI、实际路由一致。

### 9. 内部平台放行门禁

**目标**：放行标准从"功能能跑"升级为"控制动作可证明、可追溯、可回滚"。

**门禁清单**：
- 跨租户访问回归。
- 跨后端一致性回归。
- 权限矩阵回归。
- 共享知识污染回归。
- 删除补偿回归。
- 备份恢复演练。
- 策略变更审计与回滚演练。

**验收标准**：
- 每个门禁有脚本或测试入口。
- 每次 release-plan 必须引用门禁结果。
- 未通过 P0/P1 门禁不得标记内部平台可放行。

## 模块映射

| 治理域 | 现有模块 | 需要新增或收敛 |
|--------|----------|----------------|
| 后端支持矩阵 | `internal/store/pg`, `internal/store/mysql`, `docs/database.md` | 双后端能力矩阵、MySQL 静态 SQL 护栏、双后端集成测试入口 |
| 租户隔离 | `middleware.TenantMiddleware`, Store 方法 tenant 参数, PostgreSQL RLS | MySQL 隔离证明、跨后端攻击测试、cross-tenant capability 标注 |
| 共享学习 | `internal/evolution/*`, `chatHandler.evolutionImprover` | sharing policy store、sharing audit log、rollback API、batched shared revoke |
| 权限分域 | `internal/middleware/rbac.go`, `scope_check.go`, `docs/RBAC_MATRIX.md` | 统一权限 evaluator、分域角色、break-glass admin |
| 聚合统计 | `internal/api/admin/usage.go`, `internal/metering/*` | 受控聚合 Store、审计化跨租户查询、MySQL usage backend |
| 控制中心 | `internal/api/admin/*`, `webui/src/admin/*` | tenant governance center、策略版本、diff、审批链 |
| 契约治理 | `internal/api/openapi.go`, `docs/api-reference.md`, `README.md` | 路由/文档一致性测试、OpenAPI 覆盖新增控制面 |

## 首先执行的 5 件事

| 状态 | 事项 | 当前结果 |
|------|------|----------|
| Done | 在 `docs/database.md` 和本计划中落 MySQL/PostgreSQL 企业支持矩阵 | `docs/database.md` / `docs/database.en.md` 已加入双后端矩阵。 |
| Done | 给 `internal/evolution` 定义 sharing policy 数据模型和默认策略 | 已新增 `sharing_mode=disabled|anonymous|trusted`；默认禁用共享贡献；匿名共享不暴露 contributor，可信共享保留 contributor。 |
| Done | 改造 Admin API 注册方式，为每组管理路由绑定具体资源动作 | `internal/api/admin/handler.go` 已按 `billing:*`、`security:*`、`audit:read`、`ops:*`、`key:*` 等 scope 分域。 |
| Done | 设计受控 usage aggregation store，移除 handler 层 `row_security = off` 依赖 | `metering.TenantUsageAggregator` 已落地，Admin usage 通过 metering store 聚合，不再关闭 RLS。 |
| Partial | 清理 `/v1/usage`、execution receipts、GDPR、OpenAPI/API reference 的契约漂移 | `/v1/usage` 可配置接入 v2 handler；execution receipts 文档改为 query-only；OpenAPI usage/admin/evolution 描述已更新。GDPR 文档仍需专项核对。 |

## 本轮执行记录

| 领域 | 已完成 |
|------|--------|
| MySQL 生产后端 | 新增 `internal/metering/mysql_store.go` 和 MySQL `usage_records` migration；新增 `scripts/check_tenant_sql_mysql.sh`。 |
| PostgreSQL/MySQL 隔离护栏 | `scripts/check_tenant_sql.sh` 和 MySQL 版本均通过；scheduler 跨租户查询已显式标记为系统能力。 |
| 受控跨租户聚合 | `GET /admin/v1/usage/tenants` 改为使用 `metering.TenantUsageAggregator`，不再依赖 BYPASSRLS/SUPERUSER。 |
| Usage 契约 | `APIServerConfig.UsageStore` 可切换 `/v1/usage` 到 `UsageV2Handler`，并向 Admin handler 注入同一 UsageStore。 |
| Admin 分权 | 新增 `middleware.RequireAnyScope`，显式 `admin` scope 作为 break-glass 兼容；legacy empty scopes 不再通过 Admin 分域检查。 |
| 共享治理 | `internal/evolution` 支持 `disabled`、`anonymous`、`trusted` 三种共享级别，并补测试覆盖。 |
| 租户共享策略 | 新增全局 sharing policy、租户 sharing policy、共享知识 revoke Admin API；策略变更和撤回写入 audit log。 |
| 共享撤回执行面 | `RevokeShared` 改为同库 SQL 条件过滤 + bounded batch delete，不再先全量累积待删 gene ID。 |
| 平台治理中心 | 新增 `docs/runbooks/platform-governance-center.md`，收口控制域、scope、审计契约和发布门禁。 |
| MySQL 运维证明 | 新增 `docs/runbooks/mysql-backup-restore.md` 和 `docs/runbooks/backend-enterprise-validation-matrix.md`。 |
| 文档 | 更新 database/configuration/API reference/OpenAPI/changelog 中与 MySQL、usage、execution receipts 相关的漂移。 |

## 本轮验证

```bash
/usr/local/go/bin/go test ./internal/evolution ./internal/config ./internal/metering ./internal/middleware ./internal/api/admin ./internal/api ./internal/store/mysql -count=1
/usr/local/go/bin/go test ./internal/evolution ./internal/api/admin ./internal/api -count=1
/usr/local/go/bin/go test ./... -count=1
scripts/check_tenant_sql.sh
scripts/check_tenant_sql_mysql.sh
git diff --check
```

备注：按仓库入口要求尝试执行 `python3 scripts/validate_library.py`，但当前 checkout 中不存在该脚本。

## 决策口径

| 问题 | Evidence | Reasoning | Implications |
|------|----------|-----------|--------------|
| 为什么 MySQL 是 P0 | 配置和部署已允许 MySQL，Store 已注册并实现主接口 | 生产支持必须由测试和运维证明支撑，不是代码存在即可 | 若不补齐，MySQL 租户隔离和删除保证无法对内承诺 |
| 为什么共享不是禁止而是治理 | evolution 已有租户 taskClass namespace，但共享策略未显式表达 | 业务允许跨租户学习时，风险来自不可解释、不可撤回 | 需要审计、标签、租户开关、撤回，而不是删除共享能力 |
| 为什么 admin 拆分是 P1 | Admin handler 统一 admin scope，RBAC 有 admin bypass | 内部平台日常操作天然需要分权和最小权限 | 先不阻断 P0，但必须在 60-90 天内进入控制中心 |
| 为什么外部合规是 P2 | 当前缺口首先影响平台控制动作可信度 | 内部治理未成型时，外部合规叙事会失真 | 先完成控制、审计、回滚，再强化防抵赖和客户叙事 |
