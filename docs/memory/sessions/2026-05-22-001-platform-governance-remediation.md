# Session Summary: Platform Governance Remediation Planning

**日期**: 2026-05-22  
**序号**: 001  
**Slug**: platform-governance-remediation  
**角色**: tech-lead  
**任务**: 2026-05-22-platform-governance-remediation  
**阶段**: execute

---

## 背景

本轮根据三项决策重排企业治理优先级：

1. MySQL 必须视为企业生产受支持后端。
2. 跨租户共享学习允许存在，但必须治理化。
3. 当前目标偏内部平台治理，而不是对外合规叙事。

## 主要产出

| 交付物 | 状态 |
|--------|------|
| `docs/artifacts/2026-05-22-platform-governance-remediation/delivery-plan.md` | updated |
| `docs/artifacts/2026-05-22-platform-governance-remediation/execute-log.md` | updated |
| `docs/artifacts/2026-05-22-platform-governance-remediation/test-plan.md` | updated |
| `docs/runbooks/platform-governance-center.md` | new |
| `docs/runbooks/backend-enterprise-validation-matrix.md` | new |
| `docs/runbooks/mysql-backup-restore.md` | new |

## 关键结论

1. P0 从"禁止跨租户共享"调整为"共享必须受控"。
2. P0 增加 MySQL/PostgreSQL 企业能力对齐，尤其是隔离、审计、删除、备份恢复证明。
3. P1 聚焦权限分域、控制面契约漂移和受控聚合通道。
4. P2 保留外部合规强化，但排在平台治理中心建设之后。

## 后续入口

优先执行：

1. 后端企业支持矩阵。
2. sharing policy 数据模型。
3. Admin API 分域授权。
4. 受控 usage aggregation store。
5. API 文档与实现漂移清理。

## 执行进展

| 事项 | 状态 |
|------|------|
| 双后端企业支持矩阵 | Done |
| MySQL usage metering store + migration | Done |
| MySQL tenant SQL 静态护栏脚本 | Done |
| Admin usage 移除 handler 层 `row_security = off` | Done |
| Admin API 分域 scope 包装 | Done |
| Evolution sharing mode: disabled / anonymous / trusted | Done |
| Evolution 全局 sharing policy Admin API | Done |
| Evolution 租户 sharing policy Admin API | Done |
| Evolution sharing policy history / rollback | Done |
| Evolution shared knowledge revoke API | Done |
| Evolution shared revoke batched deletion | Done |
| Governance policy mutation audit | Done |
| MySQL backup/restore runbook | Done |
| 双后端企业验证矩阵 runbook | Done |
| `/v1/usage` 可配置接入 UsageV2Handler | Done |
| OpenAPI/API reference usage 与 execution receipts 漂移清理 | Partial |

验证命令：

```bash
/usr/local/go/bin/go test ./internal/evolution ./internal/config ./internal/metering ./internal/middleware ./internal/api/admin ./internal/api ./internal/store/mysql -count=1
/usr/local/go/bin/go test ./internal/evolution ./internal/api/admin ./internal/api -count=1
/usr/local/go/bin/go test ./... -count=1
scripts/check_tenant_sql.sh
scripts/check_tenant_sql_mysql.sh
git diff --check
```

`python3 scripts/validate_library.py` was attempted per AGENTS.md, but this repository checkout does not contain that script.

## 本轮新增验证

```bash
/usr/local/go/bin/go test ./internal/evolution ./internal/api/admin -count=1
/usr/local/go/bin/go test ./internal/evolution -run 'TestGeneStore_RevokeShared_BatchedDeletion|TestGeneStore_RevokeShared_TimeWindow' -count=1
```

结果：通过。

## 剩余任务

1. 多实例 evolution sharing policy 主动刷新 / watcher 尚未实现，当前跨实例策略变更依赖重启或重开 store 生效。
2. 平台治理中心 Web UI 尚未落地，当前只有 Admin API、OpenAPI 和 runbook。
3. OIDC / API key / RoleStore 的统一权限 evaluator 仍需继续收敛，避免策略判断分散在多套入口。
4. 企业 release gate、恢复演练和 closeout 证据仍需在下一轮补齐。
5. `delivery-plan.md` 存在历史 markdown lint 噪音，已识别但未纳入本次代码提交范围。

## 当前收口判断

当前实现已经完成本轮最关键的平台治理能力闭环：双后端企业支持、usage 聚合下沉、Admin 分域权限、evolution sharing policy 持久化治理、history/rollback 控制面，以及 shared revoke 的有界批量执行。

当前代码与测试状态适合先提交当前批次，再基于 backlog 继续推进下一轮治理项。
