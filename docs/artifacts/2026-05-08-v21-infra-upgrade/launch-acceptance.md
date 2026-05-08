# Launch Acceptance: HermesX v2.1.0 基础设施升级

**日期:** 2026-05-08  
**评审角色:** qa-engineer  
**状态:** BLOCKED  
**关联 test-plan:** docs/artifacts/2026-05-08-v21-infra-upgrade/test-plan.md

---

## 验收概览

| 项目 | 内容 |
|------|------|
| 验收对象 | HermesX v2.1.0 — ObjectStore 接口、可观测性基础设施、MySQL adapter |
| 验收时间 | 2026-05-08 |
| 验收角色 | qa-engineer |
| 验收方式 | 代码评审 (code-reviewer + security-reviewer 并行) + 构建/测试验证 |

---

## 验收范围

**In Scope:**
- Phase 1: ObjectStore 接口抽象 + MinIOConfig→ObjStoreConfig 重命名
- Phase 2: pprof admin server + Prometheus 5 指标 + TracedStore + ACP RequestID
- Phase 3: MySQL adapter 全量 12 sub-store + migrate + PoolProvider 迁移

**Out of Scope:**
- MySQL 实例端到端集成测试
- RustFS 实际切换验证
- 前端变更（无）

---

## 验收证据

| 证据 | 结果 |
|------|------|
| `go build ./...` | ✅ PASS |
| `go test ./... -short -count=1` | ✅ 1583 passed, 35 packages |
| code-reviewer 评审 | ❌ 2 CRITICAL, 6 HIGH, 5 MEDIUM, 4 LOW |
| security-reviewer 评审 | ❌ 2 HIGH BLOCKING, 3 HIGH NON-BLOCKING, 2 MEDIUM |
| 编译断言 (ObjectStore, MySQLStore, 12 sub-store) | ✅ PASS |

---

## 风险判断

### 已满足项

- 构建与测试基线保持 v2.0.0 同等水平（1583 passed）
- ObjectStore 接口设计正确，调用方全量切换，无循环依赖
- MySQL adapter 语法正确，`?` 占位符全量使用，无 SQL 注入
- store.RegisterDriver init() 自注册模式正确
- OTel TracedStore 在无 OTLP endpoint 时零开销（noop tracer）
- ACP RequestID 中间件已接入

### 阻塞项（上线前必须解决）

| # | 描述 | 文件 | 优先级 |
|---|------|------|------|
| B1 | `apikey.Create` 始终返回非 nil error（行为回归） | `internal/store/mysql/apikey.go:24` | P0 |
| B2 | pprof admin server 绑定 0.0.0.0 无认证（安全） | `internal/api/admin_server.go:13` | P0 |
| B3 | `HasPermission` unscoped "admin" bypass 破坏租户隔离（安全） | `internal/store/mysql/roles.go:121-124` | P0 |
| B4 | messages.Search LIMIT 参数绑定错误，LIMIT 实际未生效 | `internal/store/mysql/messages.go:71` | P0 |

### 可接受风险（上线后跟进）

| # | 描述 | 跟进时限 |
|---|------|---------|
| R1 | execution_receipts.Create 使用 QueryRowContext 做 INSERT | 本 sprint |
| R2 | COUNT 错误静默（sessions/tenants/receipts List） | 本 sprint |
| R3 | rows.Scan 错误静默 | 本 sprint |
| R4 | migrate.go 无迁移版本表 | 进入生产 MySQL 前 |
| R5 | ListDue 无 LIMIT、无并发保护 | 进入生产前 |
| R6 | mergeConfig 缺 ObjStore 块 | 本 sprint |
| R7 | LIKE 元字符未转义 | 下个 sprint |
| R8 | 连接池未配置参数 | 进入生产前 |

---

## 上线结论

**❌ 不允许上线**

存在 4 项 P0 阻塞项：

1. **B1 (APIKeyStore.Create 行为回归)**: `fmt.Errorf("%w", nil)` 返回非 nil error，任何依赖 `apikey.Create` 的路径（API key 注册、初始化）均会误判失败。此 bug 在测试套件中因缺少 MySQL 集成测试而未被发现。
2. **B2 (pprof 安全)**: 生产环境启动时若设置了 `HERMESX_ADMIN_PORT`，heap/goroutine/profile 将对网络内所有对等方暴露。
3. **B3 (租户隔离安全)**: HasPermission 中 unscoped "admin" 字符串检查允许任意租户通过修改自身 API key 的 roles 字段获得全局 RBAC bypass。
4. **B4 (Search LIMIT bug)**: messages.Search 结果集无上限，返回所有匹配行，可导致大结果集内存压力和 DoS。

**前提条件（解决阻塞项后重新评审）：**
- 修复 B1-B4
- `go test ./... -short` 继续通过
- `go vet ./...` 零警告
- 重新提交 `/team-review`

---

## 下游质疑记录（qa-engineer 对 backend-engineer 交付的质疑）

1. **质疑内容：** execute-log.md 已知 bug 一节提及 `execution_receipts.go Create 用 QueryRowContext 做 INSERT`，但未列为 P0 阻塞。`APIKeyStore.Create` 的 `fmt.Errorf("%w", nil)` 问题更为严重却未被发现并记录。
   - **质疑目标：** execute-log.md 未完成项/已知 bug 章节
   - **结论：** 要求上游在下次 handoff 中将 B1 补充到 execute-log 未完成项，并在修复 sprint 中同步更新。

2. **质疑内容：** admin_server.go 的注释提及"production needs IP allowlist"，但该注释未阻止代码合并，且 saas.go 没有对应的 loopback 绑定。这表明安全 TODO 被遗留在注释中而非 backlog。
   - **质疑目标：** 安全 TODO 管理规范
   - **结论：** 要求安全相关 TODO 进入 backlog.md 追踪，不允许以代码注释代替 issue。
