# Execute Log: HermesX v2.1.0 基础设施升级

**日期:** 2026-05-08  
**执行角色:** backend-engineer  
**状态:** completed

---

## 计划 vs 实际

| Phase | 计划 | 实际 | 偏差 |
|-------|------|------|------|
| Phase 1: ObjectStore 接口 | 引入 ObjectStore 接口，切换 RustFS config | 完成：8-method 接口 + NewObjStoreClient 工厂 + MinIOConfig→ObjStoreConfig 重命名，14 个调用方全量更新 | 无偏差 |
| Phase 2: 可观测性 | pprof + 5 Prometheus 指标 + OTel TracedStore + ACP RequestID | 完成：StartAdminServer + 5 promauto 指标 + TracedStore 全量包装 + ACP RequestID 中间件接入 | 无偏差 |
| Phase 3: MySQL Adapter | PoolProvider 迁移 + MySQL 驱动 + 全量 12 sub-store | 完成：PoolProvider 移至 pg/pool_provider.go，mysql 包含 12 sub-store + migrate.go | 无偏差 |

---

## 关键决定

1. **ObjectStore 接口放在 `internal/objstore/objstore.go`** — 与 MinIOClient 同包，避免循环依赖，调用方只需替换类型断言。
2. **NewObjStoreClient 返回接口而非具体类型** — 调用方零感知切换 RustFS（SDK 兼容，只改 endpoint）。
3. **PoolProvider 从 `store/types.go` 迁移至 `store/pg/pool_provider.go`** — 消除 `store` 包对 `pgxpool` 的依赖，MySQL adapter 无需 pgxpool 编译时依赖。
4. **TracedStore 包装全部 13 个子接口** — Sessions 和 Messages 附加完整 OTel span，其余透传，符合最小改动原则。
5. **MySQL Search 降级为 LIKE** — 无 FTS 向量，但接口语义不变，生产可按需迁移到 MySQL FULLTEXT。
6. **MySQL roles/scopes 以 JSON TEXT 存储** — 避免数组类型跨数据库兼容问题。

---

## 阻塞与解决

| 阻塞 | 根因 | 解决方式 |
|------|------|--------|
| go-sql-driver 未在 go.mod | `go get` 成功但 `go mod tidy` 未持久化 | 重新 `go get github.com/go-sql-driver/mysql@v1.10.0` 后 build 成功 |
| `store.PoolProvider` undefined | PoolProvider 从 store 包删除后 saas.go/main.go 未同步更新 | 逐一替换为 `pgstore.PoolProvider`，import alias 已在上一步添加 |
| LSP 报 MinIOClient 类型不匹配 | LSP 缓存滞后，sed 替换后 cache 未刷新 | `go build ./...` 确认为 false positive，以 build 结果为准 |

---

## 影响面

**新增文件（6个）:**
- `internal/objstore/objstore.go` — ObjectStore 接口
- `internal/api/admin_server.go` — pprof admin server
- `internal/store/traced_store.go` — OTel TracedStore
- `internal/store/pg/pool_provider.go` — PoolProvider 接口（从 types.go 迁移）
- `internal/store/mysql/` — 15 个文件（mysql.go, migrate.go, 12 sub-stores + 1 roles file）

**修改文件（10个）:**
- `internal/objstore/minio.go` — 编译断言 + NewObjStoreClient
- `internal/config/config.go` — MinIOConfig→ObjStoreConfig + yaml tag
- `internal/gateway/runner.go` — ObjectStore 接口替换
- `internal/api/server.go` — SkillsClient 已是 ObjectStore（Phase 1 前已完成）
- `internal/observability/metrics.go` — 5 个新 Prometheus 指标
- `internal/acp/server.go` — RequestID 中间件接入
- `internal/store/types.go` — 删除 PoolProvider + pgxpool import
- `internal/api/admin/handler.go` — pg.PoolProvider 替换
- `cmd/hermesx/main.go` — pgstore.PoolProvider 替换
- `cmd/hermesx/saas.go` — pgstore.PoolProvider + ObjectStore + pprof wiring

---

## 未完成项

- MySQL FULLTEXT INDEX（当前 LIKE 降级，生产可按需加 `FULLTEXT(content)` 并切换 Search 实现）
- MySQL RLS（当前为应用层 tenant_id filter，符合设计决策 Q2）
- MySQL 连接池调参（当前使用 `database/sql` 默认值，生产需设 MaxOpenConns/MaxIdleConns）

---

## 测试结果

```
go test ./... -short -count=1
1583 passed in 35 packages (PTY tests skipped in -short mode，同 v2.0.0 基线)
go build ./... → Success
```
