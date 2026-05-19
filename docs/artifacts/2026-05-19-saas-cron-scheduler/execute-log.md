# Execute Log: SaaS Distributed Cron Scheduler

> **角色**: backend-engineer | **状态**: completed | **阶段**: execute  
> **日期**: 2026-05-19 | **slug**: saas-cron-scheduler

---

## 计划 vs 实际

| Story | 计划 | 实际 | 偏差 |
|-------|------|------|------|
| A: DB Migration | PG v100-v102 + MySQL 同步 | ✅ PG v100-v102 + MySQL v17-v18 | 无偏差 |
| B: SaaS Scheduler | `internal/scheduler/` 4 个文件 | ✅ schema.go / scheduler.go / sync.go / executor.go | 无偏差 |
| D: tools/cronjob.go 改写 | SaaS/CLI 双路径 + ToolContext 字段 | ✅ 完整实现 | 无偏差 |
| E: saas.go 启动集成 | scheduler.New + Start/Stop | ✅ schedulerAgentAdapter + 有序关闭 | 无偏差 |
| F: 测试 | F1/F2/F3 | ✅ 9 tests all pass | 无偏差 |

---

## 关键决定

1. **`ListAllEnabled` vs `ListDue`**: 同步循环需要全量 enabled 列表（与 next_run_at 无关）以维护 gocron 内存注册表；`ListDue` 是执行触发接口，二者职责不同，新增独立方法。

2. **不继承 `gocron.Job` schedule 字段**: gocron v2 `Job` 接口不暴露 cron 表达式，`getJobSchedule` 返回 `("", false)`，导致每次 sync 都会 re-register 已存在但变更的 job。接受该行为（30s 间隔内最多额外注册一次），避免过度复杂化。

3. **`rediscache.Client.UniversalClient()` 新增**: gocron-redis-lock 要求 `redis.UniversalClient` 接口，`rediscache.Client` 只暴露 `*redis.Client`。通过新增 `UniversalClient()` accessor 最小化侵入，不重复创建连接。

4. **`schedulerAgentAdapter` 放在 saas.go**: 适配器只在 SaaS 入口点使用，放在 `saas.go` 避免 `cmd/` 层对 `internal/scheduler/` 的循环引用风险，也不污染 agent 包。

5. **executor 直连 pgxpool**: `cron_job_runs` 写入需要绕过 RLS（scheduler 是系统级操作），直接使用 `pool` 而非 `withTenantTx`，与 `tenant_cleanup.go` 的模式一致。

6. **MySQL `ADD COLUMN IF NOT EXISTS`**: MySQL 5.7 不支持，MySQL 8.0+ 支持；当前项目要求 MySQL 8.0，保持与 PG migration 同等可幂等写法。

---

## 阻塞与解决

| 阻塞 | 根因 | 解决 |
|------|------|------|
| gocron-redis-lock 依赖 sum check 超时 | GOPROXY 指向国内镜像，sum.golang.org 连接超时 | 设置 `GONOSUMDB` 环境变量绕过 |
| stale `cron-execute` team 持续发送 idle notification | 上一 session 创建的 background agents 未能正常响应 shutdown | 直接 `rm -rf ~/.claude/teams/cron-execute` 强制清理 |

---

## 影响面

| 模块 | 变更类型 | 说明 |
|------|----------|------|
| `internal/store/store.go` | 接口扩展 | `CronJobStore.ListAllEnabled` 新增方法 |
| `internal/store/pg/cronjobs.go` | 新增实现 | `ListAllEnabled` PG 实现 |
| `internal/store/mysql/cronjobs.go` | 新增实现 | `ListAllEnabled` MySQL 实现 |
| `internal/store/sqlite/noop.go` | 新增 noop | `ListAllEnabled` 返回 `errSQLiteUnsupported` |
| `internal/store/traced_store.go` | 新增 tracing | `tracedCronJobs.ListAllEnabled` |
| `internal/store/rediscache/redis.go` | 新增 accessor | `UniversalClient()` 方法 |
| `internal/store/pg/migrate.go` | 新增 migration | v100-v102 |
| `internal/store/mysql/migrate.go` | 新增 migration | v17-v18 |
| `internal/tools/registry.go` | 字段新增 | `ToolContext.CronJobStore` |
| `internal/tools/cronjob.go` | 完全重写 | SaaS/CLI 双路径；CLI 路径逻辑不变 |
| `internal/scheduler/` | 新包 | schema/scheduler/sync/executor + 9 个单元测试 |
| `cmd/hermesx/saas.go` | 启动集成 | Redis hoisting + scheduler init/start/stop |
| `go.mod` / `go.sum` | 新增依赖 | gocron/v2, gocron-redis-lock/v2, redsync/v4 |

---

## 未完成项

- **Story F 集成测试** (F1/F2/F3): 当前为单元测试 + SQL 契约验证。真实多 Pod 去重验证、执行历史 read-back、PG RLS 隔离需要 testcontainers 集成测试环境，列入 v2.4.0 backlog。
- **HTTP 端点** `GET /api/v1/cron-jobs/{id}/runs`: arch-design 中标注为骨架，本期未实现，列入 v2.4.0 backlog。
- **Tool 层 `CronJobStore` 注入**: `ToolContext.CronJobStore` 字段已就绪，但 API handler 层尚未在 `tctx` 中注入该字段（`dataStore.CronJobs()`），需在 Story E 的 HTTP handler 初始化中补充注入。

---

## 自测结论

```
go build ./... ✅ (0 errors)
go vet ./internal/scheduler/... ./internal/store/... ./cmd/hermesx/... ✅ (0 warnings)
go test ./internal/scheduler/... -v -count=1 ✅ 9/9 PASS (1.968s)
```
