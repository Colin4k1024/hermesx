# Test Plan: Internal Platform Governance Remediation

> 任务：platform-governance-remediation  
> 日期：2026-05-22  
> 主责角色：qa-engineer  
> 状态：ready-for-review

---

## 测试范围

### In Scope

| 模块 | 覆盖内容 |
| ---- | -------- |
| `internal/middleware/scope_check.go` | domain scope 放行、legacy 空 scope 拒绝、`admin` break-glass 放行 |
| `internal/api/admin/handler.go` | Admin 分域路由保护模型 |
| `internal/api/admin/usage.go` | aggregate-only tenant usage 聚合入口 |
| `internal/metering/pg_store.go` | PostgreSQL tenant usage 聚合实现 |
| `internal/metering/mysql_store.go` | MySQL tenant usage 聚合实现 |
| `internal/api/server.go` | `/v1/usage` 优先挂载 `UsageV2Handler` |
| `internal/evolution/store.go` | sharing mode、tenant policy、shared replay、shared revoke |
| `internal/api/admin/evolution.go` | evolution governance API 行为与审计写入路径 |
| `docs/database*.md` / `docs/api-reference*.md` / `internal/api/openapi.go` | 契约与文档同步 |

### Out of Scope

- Web UI 治理中心页面
- 外部 IdP 真实联调
- 生产 MySQL / PostgreSQL 恢复演练实操
- 长周期 retention engine / legal hold

---

## 测试矩阵

| ID | 场景 | 类型 | 预期结果 |
| -- | ---- | ---- | -------- |
| T01 | `RequireAnyScope` 无认证访问 | 单元 | 401 |
| T02 | `RequireAnyScope` 命中 domain scope | 单元 | 200 |
| T03 | `RequireAnyScope` 仅 legacy 空 scope | 单元 | 403 |
| T04 | `RequireAnyScope` 命中 `admin` 兼容 scope | 单元 | 200 |
| T05 | Admin pricing 路由允许 `billing:read` | 单元 | 200 |
| T06 | Admin pricing 路由拒绝空 scope | 单元 | 403 |
| T07 | Admin tenant usage 路由使用 `TenantUsageAggregator` | 单元 | 200，且 limit/offset 透传 |
| T08 | Admin tenant usage 无聚合器时返回 503 | 单元 | 503 |
| T09 | Evolution disabled 模式下无跨租户共享 | 单元 | tenant-b 查询不到 tenant-a gene |
| T10 | Evolution anonymous 模式下可共享但不暴露 contributor | 单元 | 返回 shared gene，`ContributorID==""` |
| T11 | Evolution trusted 模式下保留 contributor | 单元 | 返回 shared gene，`ContributorID==source tenant` |
| T12 | Evolution revoke 无条件且未 confirm_all | 单元 | 400 / error |
| T13 | `/v1/usage` 主路由挂载 v2 handler | 包级 | 支持 summary/details 语义 |
| T14 | PostgreSQL usage 聚合不依赖 handler 关闭 RLS | 代码审查 + 包级 | Admin handler 不再执行 RLS disable |
| T15 | MySQL usage 聚合可编译、可测试 | 包级 | 聚合接口可用 |
| T16 | Evolution sharing policy 更新后重开 store | 单元 | global / tenant policy 与 version 保持一致 |
| T17 | Evolution admin 接口写入持久化策略 | 包级 | admin 写入后当前 version 递增，相关包测试通过 |
| T18 | Evolution global sharing policy history / rollback | 单元 | 可列出历史版本，并在回滚后生成新的 current version |
| T19 | Evolution tenant sharing policy history / rollback | 单元 | 可列出租户历史版本，并在回滚后恢复指定策略 |
| T20 | Evolution shared revoke batched deletion | 单元 | 跨多个 batch 的共享基因可按条件撤回，未命中租户的共享基因保留 |

---

## 执行命令

### 已执行

```bash
/usr/local/go/bin/go test ./internal/api/... ./internal/metering ./internal/middleware ./internal/evolution ./cmd/hermesx/...
/usr/local/go/bin/go test ./internal/evolution -run TestGeneStore_PolicyPersistsAcrossReopen -v -count=1 -timeout 10s
/usr/local/go/bin/go test ./internal/evolution ./internal/api/admin -count=1
/usr/local/go/bin/go test ./internal/evolution ./internal/api/admin -run 'TestGeneStore_SharingPolicyHistoryAndRollback|TestAdminEvolutionSharingPolicy_HistoryAndRollback|TestAdminEvolutionTenantSharingPolicy_HistoryAndRollback' -count=1
/usr/local/go/bin/go test ./internal/evolution -run 'TestGeneStore_RevokeShared_BatchedDeletion' -count=1
```

### 建议补充执行

```bash
scripts/check_tenant_sql.sh
scripts/check_tenant_sql_mysql.sh
DATABASE_DRIVER=postgres DATABASE_URL="$POSTGRES_DATABASE_URL" /usr/local/go/bin/go test ./internal/store/pg ./internal/api ./internal/metering -count=1
DATABASE_DRIVER=mysql DATABASE_URL="$MYSQL_DATABASE_URL" /usr/local/go/bin/go test ./internal/store/mysql ./internal/api ./internal/metering -count=1
```

---

## 当前结果

### 已完成验证

| 范围 | 结果 |
| ---- | ---- |
| `internal/api/...` | ✅ PASS |
| `internal/api/admin` | ✅ PASS |
| `internal/metering` | ✅ PASS |
| `internal/middleware` | ✅ PASS |
| `internal/evolution` | ✅ PASS |
| `cmd/hermesx/...` | ✅ PASS（无测试文件，但编译通过） |
| `TestGeneStore_PolicyPersistsAcrossReopen` | ✅ PASS |
| `TestGeneStore_SharingPolicyHistoryAndRollback` | ✅ PASS |
| `TestAdminEvolutionSharingPolicy_HistoryAndRollback` | ✅ PASS |
| `TestAdminEvolutionTenantSharingPolicy_HistoryAndRollback` | ✅ PASS |
| `TestGeneStore_RevokeShared_BatchedDeletion` | ✅ PASS |

---

## 风险与回归关注点

| 风险 | 等级 | 说明 | 建议 |
| ---- | ---- | ---- | ---- |
| `admin` 兼容 scope 仍然可放行 | MEDIUM | 兼容旧调用方，但权限面仍偏宽 | 后续迁移到显式 break-glass 角色 |
| evolution sharing policy 多实例刷新尚未闭环 | MEDIUM | 当前值与版本历史已持久化，但多实例即时同步还未实现 | 后续补 reload / watcher 或统一控制面刷新 |
| MySQL 企业证明仍依赖环境级实测 | MEDIUM | 当前已补 runbook 与 store 实现，但缺 live drill 证据 | 发布前执行 backend validation matrix |
| 文档变更较多 | LOW | 易出现漏同步 | 发布前跑 OpenAPI / docs parity review |

---

## 放行建议

当前建议：**可以进入代码评审**。

理由：

- 本轮核心治理改动已有测试覆盖并通过。
- 主要剩余项集中在发布前 runbook 演练、多实例策略刷新和更完整的控制台能力，而不是当前实现错误。

放行前仍建议补做两项：

1. 双后端静态 SQL guard 结果归档。
2. PostgreSQL / MySQL 各做一次环境化 usage + GDPR 验证。
