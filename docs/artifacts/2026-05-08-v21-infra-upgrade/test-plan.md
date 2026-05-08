# Test Plan: HermesX v2.1.0 基础设施升级

**日期:** 2026-05-08  
**评审角色:** qa-engineer  
**状态:** review  
**关联任务:** docs/artifacts/2026-05-08-v21-infra-upgrade/

---

## 测试范围

### In Scope

| 模块 | 测试类型 | 说明 |
|------|---------|------|
| Phase 1: ObjectStore 接口 | 编译断言 + 构建验证 | 接口实现正确性，14 个调用方类型替换 |
| Phase 2: 可观测性 | 构建验证 + 指标注册确认 | pprof、Prometheus 5 指标、TracedStore、ACP RequestID |
| Phase 3: MySQL adapter | 编译断言 + 构建验证 | 12 sub-store 接口全量实现，store.RegisterDriver 注册 |
| 全量测试基线 | 单元/集成测试 | `go test ./... -short -count=1` |

### Out of Scope

- MySQL 实例集成测试（当前无 MySQL Testcontainer 支持）
- RustFS/MinIO 端到端 object store 验证
- Prometheus 指标数值正确性验证（需运行时环境）
- pprof 堆转储内容验证

---

## 测试矩阵

| 场景 | 类型 | 状态 | 风险 |
|------|------|------|------|
| `go build ./...` 全量编译 | 构建 | ✅ PASS | — |
| `go test ./... -short -count=1` 1583 个用例 | 单元 | ✅ PASS (1583 passed, 35 packages) | — |
| ObjectStore 接口编译断言 `var _ ObjectStore = (*MinIOClient)(nil)` | 编译 | ✅ PASS | — |
| MySQLStore 接口编译断言 `var _ store.Store = (*MySQLStore)(nil)` | 编译 | ✅ PASS | — |
| 12 sub-store 编译断言 | 编译 | ✅ PASS | — |
| APIKeyStore.Create 返回 nil 成功 | 行为 | ❌ **FAIL** | CRITICAL: `fmt.Errorf("%w", nil)` 始终返回 non-nil error |
| execution_receipts.Create INSERT 正确执行 | 行为 | ⚠️ 未覆盖 | HIGH: QueryRowContext 误用于 INSERT，可静默吞噬失败 |
| HasPermission 租户隔离 | 安全 | ❌ **FAIL** | HIGH: unscoped "admin" 字符串绕过 tenant 作用域 |
| pprof admin server 不对外暴露 | 安全 | ❌ **FAIL** | HIGH: 绑定 0.0.0.0，无认证 |
| sessions/tenants/execution_receipts COUNT 错误不静默 | 行为 | ❌ **FAIL** | HIGH: 3 处 Scan 错误未检查，返回错误的分页 total |
| rows.Scan 错误传播 | 行为 | ❌ **FAIL** | HIGH: 所有 store 的 Scan 错误静默丢弃，污染结果集 |
| messages.Search LIMIT 实际生效 | 行为 | ❌ **FAIL** | MEDIUM: LIMIT ? 是第 3 个参数但 SQL 只有 2 个 `?` |
| ObjStore yaml config 不丢失 | 配置 | ❌ **FAIL** | MEDIUM: mergeConfig 缺少 ObjStore 块，yaml 配置静默丢失 |
| MySQL adapter migration 幂等性 | 运维 | ⚠️ 部分 | HIGH: 仅 CREATE TABLE IF NOT EXISTS，无版本追踪 |

---

## 已确认通过项

- 全量 1583 测试用例在 -short 模式下通过，与 v2.0.0 基线一致
- `go build ./...` 零错误
- ObjectStore 接口设计正确，14 个调用方全量切换
- MySQL `?` 占位符、`ON DUPLICATE KEY UPDATE`、`key_name` 等 dialect 差异处理正确
- `store.RegisterDriver("mysql", ...)` init() 注册模式正确
- roles.go HasPermission 的 `fmt.Sprintf` 仅插入 `?,?` 字面量，无 SQL 注入风险
- auditlog.go 动态 WHERE 子句仅拼接硬编码 SQL 片段，无 SQL 注入风险
- 所有 DML 均使用 `?` 绑定参数，无字符串拼接用户输入进入 SQL

---

## 阻塞问题（Abort Gate）

### CRITICAL-1: APIKeyStore.Create 始终返回非 nil error

**位置:** `internal/store/mysql/apikey.go:24`

```go
return fmt.Errorf("%w", wrapNil(err))
```

`wrapNil(nil)` 返回 `nil`，但 `fmt.Errorf("%w", nil)` 返回非 nil 的 `*errors.errorString`。所有 API 密钥创建调用均会触发调用方的 `if err != nil` 分支，将成功 INSERT 视为错误。**这是行为回归，影响所有 API 密钥创建路径。**

**修复：** 删除 `fmt.Errorf` 包装和 `wrapNil` 函数。

### CRITICAL-2: pprof admin server 暴露于全网络接口，无认证

**位置:** `internal/api/admin_server.go:13`

`http.ListenAndServe(addr, nil)` 绑定 `0.0.0.0`，DefaultServeMux 上挂载了 pprof 路由（含 `/debug/pprof/profile`），无任何认证。任何可到达进程端口的网络对等方可获取 heap dump、goroutine stack（含 session ID、DSN）、30s CPU profile。

**修复：** 绑定 `127.0.0.1`，增加 IP 白名单或 Bearer token 认证。

### CRITICAL-3: HasPermission unscoped "admin" bypass 破坏租户隔离

**位置:** `internal/store/mysql/roles.go:121-124`

在执行 SQL 前，代码检查 roles 切片中是否含有字符串 `"admin"` 并直接返回 `true`，未验证该 role 是否属于 `tenantID`。任何租户的 API key 若 roles JSON 中含 `"admin"`，即可对所有 resource/action 返回 true，绕过 RBAC。

**修复：** 删除 unscoped 字符串快捷方式，由数据库层 `resource='*', action='*'` 行表达超级权限，并始终 scoped 到 tenant。

---

## 高风险问题（Revision Gate）

| 编号 | 位置 | 描述 | 建议 |
|------|------|------|------|
| H1 | `execution_receipts.go:18-31` | QueryRowContext 误用于 INSERT，error 判断依赖 `!= sql.ErrNoRows` 不可靠 | 改用 ExecContext |
| H2 | `sessions.go:58`, `tenant.go:74`, `execution_receipts.go:79` | COUNT 错误静默丢弃，分页 total 返回错误值 | 参照 auditlog.go:59 的错误检查模式 |
| H3 | 所有 mysql store `rows.Next()` 内 | rows.Scan 错误静默丢弃，污染结果集 | 逐行检查 Scan 错误 |
| H4 | `messages.go:71` | LIMIT ? 是第 3 个参数，SQL 只有 2 个 `?`，LIMIT 未生效 | 修正 SQL 占位符 |
| H5 | `config.go mergeConfig` | ObjStore 块缺失，yaml 配置中的 objstore 字段静默丢弃 | 补加 ObjStore merge 逻辑 |
| H6 | `migrate.go:187-194` | 无迁移版本表，ALTER TABLE 操作重启后会重复执行失败 | 引入迁移版本追踪（如 schema_migrations 表）|
| H7 | `cronjobs.go:88-107` | ListDue 无 LIMIT，多节点并发执行无幂等保护 | 加 LIMIT，考虑 claimed_by/locked_until 列 |

---

## 中低风险（待跟踪）

| 编号 | 位置 | 描述 |
|------|------|------|
| M1 | `traced_store.go:23-34` | 仅 Sessions/Messages 有 span，其余 10 个 sub-store 透传 |
| M2 | `messages.go:66`, `users.go:68` | LIKE 模式未转义 `%` / `_` 元字符，导致宽泛匹配 |
| M3 | `messages.go:62-71` | 未限制 query 长度，超长 LIKE 模式可 DoS MySQL 模式匹配 |
| M4 | `tenant.go:38-40` | not-found 返回 `fmt.Errorf("get tenant: %w", sql.ErrNoRows)`，其他 store 返回 `nil, nil`，不一致 |
| M5 | `mysql.go:36-44` | 无连接池参数，生产需设 MaxOpenConns/MaxIdleConns |
| M6 | `apikey.go:62-63,82-83` | json.Unmarshal 错误静默丢弃，roles/scopes 损坏时降级为零权限 |
| L1 | `apikey.go:88` | wrapNil 是死代码（修复 CRITICAL-1 时一并删除）|
| L2 | `traced_store.go:179` | `var _ = time.Second` 仅为抑制编译器错误，应删除 time import |
| L3 | `admin_server.go:13` | 无 ReadTimeout/WriteTimeout（修复 CRITICAL-2 时一并加固）|

---

## 放行建议

**结论：不建议放行（BLOCK）**

三项 CRITICAL 问题在当前代码中可验证：
1. `apikey.Create` 行为回归（所有 API 密钥创建返回 error）
2. pprof 暴露（安全）
3. HasPermission 租户隔离失效（安全）

以及 H4（messages.Search 的 LIMIT 未生效）为功能 bug。

修复上述四项后可重新评审。H1-H3/H5-H7 建议在本 sprint 内跟进；M1-M6/L1-L3 纳入下一 sprint backlog。
