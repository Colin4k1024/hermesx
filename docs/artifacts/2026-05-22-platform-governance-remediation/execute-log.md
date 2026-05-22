# Execute Log: Internal Platform Governance Remediation

> 任务：platform-governance-remediation  
> 日期：2026-05-22  
> 主责角色：backend-engineer  
> 状态：ready-for-review

---

## 计划 vs 实际

### 原计划

本轮目标是推进 30 天计划中的 3 条主线，并提前落一部分 60 天计划基础设施：

1. 双生产后端企业支持矩阵。
2. 共享学习治理基线。
3. Admin 分域权限模型。
4. 受控跨租户 usage 聚合。

### 实际完成

1. 补齐 PostgreSQL / MySQL 双后端企业支持矩阵文档与 runbook。
2. 将主路由 `/v1/usage` 切换为优先挂载 `UsageV2Handler`，保留旧 handler 作为降级路径。
3. 在 `metering` 中新增跨租户 aggregate-only 聚合接口，并补 MySQL 实现。
4. 将 Admin handler 从统一 `RequireScope("admin")` 改为分域 `RequireAnyScope(...)`。
5. 新增 evolution 共享治理接口：全局策略、租户策略、共享撤回。
6. 在 evolution store 中实现共享等级、租户策略解析、共享写入和共享撤回逻辑。
7. 新增与上述治理改动直接相关的测试。
8. 将 evolution sharing policy 从运行时内存态升级为持久化当前值 + 版本历史，并在 store 重开后自动恢复。
9. 新增 evolution sharing policy / tenant sharing policy 的 history 查询与 rollback API，并将 rollback 设计为“基于历史版本生成新 current version”。
10. 将 shared knowledge revoke 从“全量扫描后逐条删除”改为“数据库条件过滤 + bounded batch delete”，降低大批量撤回时的内存与执行窗口风险。

---

## 关键实现清单

| 主题 | 主要文件 |
| ---- | -------- |
| 双后端支持矩阵 | `docs/database.md`, `docs/database.en.md`, `docs/runbooks/backend-enterprise-validation-matrix.md`, `docs/runbooks/mysql-backup-restore.md` |
| 主路由 usage v2 | `internal/api/server.go`, `cmd/hermesx/saas.go`, `cmd/hermesx/main.go` |
| usage 聚合接口 | `internal/api/admin/usage.go`, `internal/metering/store.go`, `internal/metering/pg_store.go`, `internal/metering/mysql_store.go` |
| 分域 scope | `internal/api/admin/handler.go`, `internal/middleware/scope_check.go`, `internal/middleware/scope_check_test.go` |
| 共享治理 | `internal/evolution/config.go`, `internal/evolution/policy.go`, `internal/evolution/store.go`, `internal/api/admin/evolution.go`, `internal/api/admin/evolution_test.go` |
| 共享策略持久化版本化 | `internal/evolution/policy_persistence.go`, `internal/evolution/store.go`, `internal/evolution/evolution_test.go`, `internal/api/admin/evolution.go` |
| 共享策略历史与回滚控制面 | `internal/evolution/policy_persistence.go`, `internal/evolution/store.go`, `internal/api/admin/evolution.go`, `internal/api/admin/handler.go`, `internal/api/openapi.go` |
| 共享撤回批量执行 | `internal/evolution/policy_persistence.go`, `internal/evolution/store.go`, `internal/evolution/evolution_test.go` |
| 契约同步 | `internal/api/openapi.go`, `docs/api-reference*.md`, `docs/index*.md`, `docs/RBAC_MATRIX*.md`, `docs/SECURITY_MODEL*.md`, `docs/CHANGELOG*.md` |

---

## 关键决定

### 1. MySQL 生产支持不再借 PostgreSQL RLS 叙事

决定：把 MySQL 的企业支持证明收口为 Store tenant scoping + SQL 静态护栏 + 回归测试 + 恢复 runbook。

原因：用户已确认 MySQL 是企业生产支持后端，不能再以“PG 有 RLS”代替 MySQL 的治理证明。

### 2. 保留 `admin` scope 作为 break-glass 兼容

决定：不在本轮直接移除 `admin` scope，而是在 `RequireAnyScope` 中将其视为显式兼容放行。

原因：当前已有调用方和文档仍可能依赖 `admin`；本轮目标是先建立分域模型，再逐步收缩旧口径。

### 3. 共享治理从运行时策略推进到持久化版本化

决定：将 global / tenant sharing policy 的当前值和版本历史一并持久化到 evolution backend，并由 `GeneStore.Open` 在启动时恢复。

原因：仅运行时策略无法跨重启收敛，也无法作为后续 diff / rollback 的事实源；而直接在 evolution backend 内落 current + history 是本阶段最小且稳定的升级路径。

### 4. Policy rollback 采用“追加新版本”而不是“覆盖旧版本”

决定：回滚接口不直接覆写历史版本，而是读取目标版本快照，再写入一条新的 current/history 记录。

原因：这样可以保留完整审计链，避免回滚动作本身破坏版本序列，也更符合内部平台治理对可追溯性的要求。

---

## 阻塞与解决

| 问题 | 根因 | 处理结果 |
| ---- | ---- | -------- |
| Admin usage 聚合依赖 handler 关闭 RLS | 平台聚合能力和业务查询耦合 | 改为 `TenantUsageAggregator` 接口，下沉到 metering store |
| `/v1/usage` 仍挂旧 summary handler | 增强版 usage 已实现但未接线 | 在 `APIServer` 中优先挂载 `UsageV2Handler` |
| Admin 路由全部绑定 `admin` scope | 早期实现以快速上线为主 | 改为 domain scopes，保留 `admin` 作为兼容 scope |
| 共享学习缺少治理接口 | evolution 设计最初偏内部能力 | 新增 admin evolution governance endpoints + audit 写入 |
| 共享策略重启后丢失 | 策略只存在进程内存 `cfg`，没有持久化真相源 | 新增 policy current/history 持久化层，并修复首轮实现中的锁重入死锁 |
| 共享策略无法查询历史或做策略回滚 | history 表已存在，但缺少读路径和控制面 API | 新增 global/tenant history 查询与 rollback API，并补审计与回归测试 |
| shared revoke 大批量执行风险 | 原逻辑先攒全量 gene ID 再逐条删除，放大内存和执行时间窗口 | 改为同库 SQL 条件过滤 + batch delete 循环，每批固定上限 |

---

## 影响面

- API 面：`/v1/usage`, `/v1/usage/details`, `/admin/v1/usage`, `/admin/v1/usage/tenants`, `/admin/v1/evolution/*`
- 权限面：Admin scope 校验模型、OpenAPI、RBAC 文档、API 文档
- 后端面：metering store、evolution store、saas 启动装配
- 文档面：database、security、api-reference、index、runbooks、changelog

---

## 验证结果

已执行聚焦测试：

```bash
/usr/local/go/bin/go test ./internal/api/... ./internal/metering ./internal/middleware ./internal/evolution ./cmd/hermesx/...
```

结果：通过。

另外执行了定向测试：

- `internal/api/admin/handler_test.go`
- `internal/api/admin/usage_test.go`
- `internal/middleware/scope_check_test.go`
- `internal/evolution/evolution_test.go`

结果：通过。

补充执行：

```bash
/usr/local/go/bin/go test ./internal/evolution -run TestGeneStore_PolicyPersistsAcrossReopen -v -count=1 -timeout 10s
/usr/local/go/bin/go test ./internal/evolution ./internal/api/admin -count=1
/usr/local/go/bin/go test ./internal/evolution ./internal/api/admin -run 'TestGeneStore_SharingPolicyHistoryAndRollback|TestAdminEvolutionSharingPolicy_HistoryAndRollback|TestAdminEvolutionTenantSharingPolicy_HistoryAndRollback' -count=1
/usr/local/go/bin/go test ./internal/evolution -run 'TestGeneStore_RevokeShared_BatchedDeletion' -count=1
```

结果：通过。

---

## 未完成项

1. Web UI 治理中心未实现，仅有 API 和 runbook。
2. 更完整的 release gate 和恢复演练尚未写入当前 artifact。
3. 多实例场景下共享策略的主动刷新 / 同步机制仍未实现。
4. OIDC / API key / RoleStore 的统一 evaluator 仍有继续收敛空间。

---

## 当前结论

本轮已经完成“继续剩余任务”中最关键的实现收口：

- 代码、文档、测试三条线都已同步推进。
- 当前状态适合进入 review，而不是继续扩大范围。
