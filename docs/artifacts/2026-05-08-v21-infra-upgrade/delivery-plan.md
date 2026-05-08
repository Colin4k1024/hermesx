# Delivery Plan: HermesX v2.1.0 基础设施升级

**状态**: plan  
**日期**: 2026-05-08  
**Owner**: tech-lead  
**Slug**: v21-infra-upgrade  
**关联 PRD**: prd.md

---

## 需求挑战会结论

### 挑战 C1 — Phase 1 不是纯配置切换

- **质疑**：PRD 假设"仅替换 endpoint/credentials，代码层零改动"，但 `internal/objstore/minio.go` 无任何接口抽象，14 个调用方直接持有 `*MinIOClient` 具体类型
- **挑战者**：architect
- **结论**：必须先引入 `ObjectStore` 接口，将 MinIOClient 包装为接口实现，再通过 config 选择 RustFS endpoint。Phase 1 工作量从"配置修改"升级为"接口设计 + 配置切换"
- **接受状态**：已接受

### 挑战 C2 — requestId 已完成，不计工作量

- **质疑**：PRD 将 requestId 列为可观测性工作，但 `internal/middleware/requestid.go` 已实现，并已在 `internal/api/server.go:92` 挂载到完整的中间件链
- **挑战者**：backend-engineer（代码审计）
- **结论**：requestId 任务降级为"验证覆盖完整性 + ACP Server 补挂"，工作量 <2h
- **接受状态**：已接受

### 挑战 C3 — PostgreSQL 全文搜索无法移植 MySQL

- **质疑**：`pg/trigram_search.go` 使用 `ts_headline`、`plainto_tsquery`、`to_tsvector`、`@@` 操作符——这些是 PostgreSQL 专有能力，MySQL 无对等实现
- **挑战者**：architect
- **结论**：MySQL adapter 的 `MessageStore.Search` 降级为 `LIKE '%keyword%'` 实现；接口契约不变，行为降级在文档中注明。三元组搜索扩展 (`MemorySearcher.SearchSimilar`) 在 MySQL 模式下返回 `ErrNotSupported`
- **接受状态**：已接受，需在 MySQL adapter 中文档化降级行为

### 挑战 C4 — PoolProvider 接口暴露 pgxpool 需先清理

- **质疑**：`internal/store/types.go` 中的 `PoolProvider` 接口返回 `*pgxpool.Pool`，MySQL 实现无法编译通过此约束
- **挑战者**：backend-engineer
- **结论**：Phase 3 第一步删除或替换 `PoolProvider`，使 `types.go` 不再直接依赖 `pgxpool` 包；或将其移入 `pg/` 包作为 pg 专属工具
- **接受状态**：已接受

---

## 版本目标

| 版本 | 范围 | 放行标准 |
|------|------|---------|
| v2.1.0 Phase 1 | ObjectStore 接口 + RustFS 配置 | 单元测试 + 集成测试通过，go vet 无报错 |
| v2.1.0 Phase 2 | pprof + Prometheus 补齐 + OTel 扩展 + requestId 验证 | 所有新指标在 /metrics 可见，pprof 端点可访问，全链路 trace 可观测 |
| v2.1.0 Phase 3 | MySQL adapter 全量实现 | Store 接口所有方法 MySQL 测试通过，PostgreSQL 回归无损 |

---

## 工作拆解

### Phase 1 — ObjectStore 接口抽象 + RustFS 切换（低风险）

| # | 工作项 | 描述 | Owner | 依赖 | 估时 |
|---|--------|------|-------|------|------|
| 1.1 | 定义 ObjectStore 接口 | 在 `internal/objstore/objstore.go` 提取 9 方法接口 | architect | — | 2h |
| 1.2 | MinIOClient 实现接口 | 确保 MinIOClient 编译满足 ObjectStore 接口 | backend-engineer | 1.1 | 1h |
| 1.3 | 更新 14 个调用方 | 所有 `*MinIOClient` 替换为 `ObjectStore` 接口类型 | backend-engineer | 1.2 | 3h |
| 1.4 | config 支持 RustFS endpoint | MinIOConfig 结构体改名为 ObjStoreConfig，字段语义不变；NewMinIOClient 改为 NewObjStoreClient | backend-engineer | 1.3 | 1h |
| 1.5 | 集成测试 | 验证 RustFS endpoint 读写（可用 minio-go 指向 RustFS） | qa-engineer | 1.4 | 2h |

**Phase 1 总估时**: ~9h

### Phase 2 — 可观测性补齐（中风险，独立可并行）

| # | 工作项 | 描述 | Owner | 依赖 | 估时 |
|---|--------|------|-------|------|------|
| 2.1 | pprof admin 端点 | 注册 `/debug/pprof/*` 到独立 admin 路由组（env-gated，生产需 IP 白名单） | backend-engineer | — | 2h |
| 2.2 | requestId ACP 覆盖 | 验证 ACP Server 是否已挂载 RequestIDMiddleware；若无则补挂 | backend-engineer | — | 1h |
| 2.3 | Prometheus 补齐 | 新增 3 类指标：gateway 事件 counter、session 操作 histogram、objstore 操作 counter | backend-engineer | — | 3h |
| 2.4 | OTel span 扩展 | 为 Store 方法和 objstore 操作添加 span（HTTP handler → store → objstore 全链路） | backend-engineer | 2.3 | 4h |

**Phase 2 总估时**: ~10h（2.3 和 2.1/2.2 可并行）

### Phase 3 — MySQL Adapter（高风险，需 PostgreSQL 回归保护）

| # | 工作项 | 描述 | Owner | 依赖 | 估时 |
|---|--------|------|-------|------|------|
| 3.1 | 清理 PoolProvider | 将 types.go 中 PoolProvider 接口移入 pg/ 包或删除 | backend-engineer | — | 1h |
| 3.2 | MySQL 驱动注册 | 在 factory.go 注册 "mysql" 驱动；实现 MySQLStore struct | backend-engineer | 3.1 | 2h |
| 3.3 | MySQL DDL 迁移 | 将 pg/migrate.go 的 DDL 翻译为 MySQL 语法（UUID、DATETIME、TEXT 兼容） | backend-engineer | 3.2 | 4h |
| 3.4 | 实现 12 个子 Store | MySQL 实现全量（`?` 占位符、tenant_id WHERE 注入、无 ON CONFLICT） | backend-engineer | 3.3 | 16h |
| 3.5 | MessageStore.Search 降级 | MySQL 模式使用 LIKE '%keyword%'，文档化降级行为 | backend-engineer | 3.4 | 2h |
| 3.6 | MemorySearcher MySQL 桩 | SearchSimilar 返回 ErrNotSupported（MySQL 无向量扩展） | backend-engineer | 3.4 | 1h |
| 3.7 | MySQL 集成测试 | 在 CI 矩阵中新增 MySQL 8.0 服务，所有 Store 接口测试通过 | qa-engineer | 3.5, 3.6 | 4h |
| 3.8 | PostgreSQL 回归测试 | 确认 pg 实现无损，原有 1588 测试通过 | qa-engineer | 3.7 | 1h |

**Phase 3 总估时**: ~31h

---

## Brownfield 上下文快照

| 维度 | 当前状态 |
|------|---------|
| 测试基线 | 1588 tests passing，gofmt 全绿 |
| 对象存储 | MinIOClient (concrete)，9 方法，14 调用方，无接口层 |
| 可观测性 | Prometheus 11 指标，OTel HTTP/LLM/PGX span，requestId 已挂 API Server；pprof 缺失 |
| 存储层 | Store 接口 13 子接口，pg 实现全量，factory 驱动注册机制已有 |
| tenantID 传递 | 显式参数，pg 通过 set_config RLS，MySQL 需应用层 WHERE |
| DB 配置 | config.yaml `database.driver` 字段，factory.go 驱动注册表 |

---

## Story Slice 列表

| Slice | 目标 | 验收标准 | Owner | Handoff 终点 |
|-------|------|---------|-------|------------|
| S1: ObjectStore 接口 | 定义接口，所有调用方编译通过 | go build 无报错，go test ./internal/objstore/... 通过 | backend-engineer | architect review |
| S2: RustFS 配置 | ObjStoreConfig 切换，集成验证 | 集成测试 PUT/GET/DELETE 通过 | backend-engineer | qa-engineer |
| S3: pprof 端点 | /debug/pprof/* 可访问，有访问控制 | curl 成功，生产 env gating 有效 | backend-engineer | qa-engineer |
| S4: Prometheus 补齐 | 3 类新指标注册并有数据 | /metrics 端点可见新指标，单元测试覆盖 | backend-engineer | qa-engineer |
| S5: OTel 全链路 | store + objstore span 可见 | Jaeger/OTLP 收到完整链路 span | backend-engineer | qa-engineer |
| S6: MySQL 框架 | 驱动注册 + DDL 迁移 | `go run . --db mysql` 启动不 panic，表结构创建成功 | backend-engineer | architect review |
| S7: MySQL 全量实现 | 12 子 Store 实现 + tenant_id 注入 | MySQL CI 矩阵全量通过 | backend-engineer | qa-engineer |
| S8: PostgreSQL 回归 | pg 实现无损 | 原 1588 测试全部通过 | qa-engineer | tech-lead |

---

## 风险与依赖

| 风险 | 影响 | 缓解 | Owner |
|------|------|------|-------|
| RustFS SDK 兼容性未验证 | 中 — Phase 1 集成测试可能失败 | 提前用 minio-go 跑 S3 API 子集测试 | devops-engineer |
| MySQL ON CONFLICT 无原生支持 | 中 — MemoryStore.Upsert 需要 INSERT ... ON DUPLICATE KEY UPDATE | 在 MySQLStore 实现中单独处理 UPSERT 模式 | backend-engineer |
| trigram 搜索降级影响产品体验 | 低 — MySQL 模式搜索质量下降 | 在 config 和文档中明确说明 MySQL 模式限制 | tech-lead |
| pprof 暴露堆信息 | 高 — 生产安全风险 | env 变量控制，仅在非生产或 IP 白名单下启用 | backend-engineer |
| Phase 3 工作量失控（31h 估计） | 高 | 12 子 Store 按子接口拆分独立 PR，逐一合并 | backend-engineer |

---

## 节点检查

| 节点 | 条件 |
|------|------|
| Phase 1 开发完成 | ObjectStore 接口 + RustFS 配置 + 集成测试通过 |
| Phase 2 开发完成 | pprof 端点 + 新指标 + OTel 全链路 + ACP requestId 确认 |
| Phase 3 框架就绪 | PoolProvider 清理 + MySQL 驱动注册 + DDL 迁移通过 |
| Phase 3 全量完成 | MySQL CI 矩阵 + PostgreSQL 回归全量通过 |
| 放行准入 | 所有阶段完成，go vet + gofmt 全绿，无 CRITICAL 安全问题 |

---

## 角色分工

| 角色 | Phase 1 | Phase 2 | Phase 3 |
|------|---------|---------|---------|
| architect | 接口设计评审 | — | PoolProvider 清理方案 |
| backend-engineer | 接口实现 + 调用方迁移 | pprof + Prometheus + OTel | MySQL 全量实现 |
| qa-engineer | 集成测试 | 指标验证 + pprof 访问控制 | MySQL/pg 双向回归 |
| devops-engineer | RustFS 连接验证 | pprof 端口访问控制 | MySQL CI 服务配置 |
| tech-lead | 接口设计决策 | pprof 端口策略决策 | MySQL 降级策略确认 |

---

## 技术架构等级与 ADR

- 应用等级：内部基础设施工具（T3 参考基线）
- 本次变更涉及存储驱动切换和接口抽象，建议输出 2 个 ADR：
  - **ADR-001**: ObjectStore 接口设计与 RustFS 切换决策
  - **ADR-002**: MySQL adapter 策略（tenant_id filter 替代 RLS、搜索降级）

---

## implementation-readiness 结论

| 检查项 | 状态 |
|--------|------|
| PRD 存在且关键待确认项已收口 | ✅ Q1/Q2 已决策 |
| 需求挑战会已完成（C1~C4） | ✅ |
| brownfield 上下文已梳理 | ✅ |
| Story slice 有清晰验收标准 | ✅ |
| 风险已显式列出 | ✅ |
| Phase 1 + Phase 2：handoff-ready | ✅ 可进入 /team-execute |
| Phase 3：handoff-ready | ✅ 需先完成 3.1（PoolProvider 清理）再开始 3.2+ |
