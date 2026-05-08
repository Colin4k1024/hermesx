# Arch Design: HermesX v2.1.0 基础设施升级

**状态**: draft  
**日期**: 2026-05-08  
**Owner**: architect  
**Slug**: v21-infra-upgrade

---

## 系统边界

```
┌─────────────────────────────────────────────────────────────────┐
│                        HermesX Process                          │
│                                                                  │
│  HTTP Middleware Chain                                           │
│  Tracing → Metrics → RequestID → Auth → Tenant → Logging        │
│                           │                                      │
│  ┌──────────┐   ┌─────────┴──────────┐   ┌──────────────────┐  │
│  │  Agent   │   │    API Handlers     │   │   Admin Server   │  │
│  │  Loop    │   │  /v1/chat /v1/...   │   │  /debug/pprof/*  │  │
│  └────┬─────┘   └─────────┬──────────┘   └──────────────────┘  │
│       │                   │                                      │
│  ┌────▼───────────────────▼──────────────────┐                  │
│  │              Store (interface)             │                  │
│  │  Sessions / Messages / Users / Tenants /   │                  │
│  │  APIKeys / Memories / Roles / CronJobs ... │                  │
│  └────────────┬──────────────────┬────────────┘                 │
│               │                  │                               │
│  ┌────────────▼──┐  ┌────────────▼──┐                           │
│  │  pg.PGStore   │  │ mysql.MySQLSt. │  (factory.go 驱动注册)   │
│  │  pgxpool      │  │ database/sql   │                           │
│  └───────────────┘  └───────────────┘                           │
│                                                                  │
│  ┌───────────────────────────────────────────┐                  │
│  │            ObjectStore (interface)  NEW    │                  │
│  │  GetObject / PutObject / DeleteObject ...  │                  │
│  └──────────────────┬────────────────────────┘                  │
│                     │                                            │
│  ┌──────────────────▼──┐                                        │
│  │  objstore.Client    │  (RustFS endpoint via minio-go SDK)    │
│  └─────────────────────┘                                        │
└─────────────────────────────────────────────────────────────────┘

外部依赖:
  PostgreSQL 16  OR  MySQL 8.0+  (via config driver 选择)
  RustFS  (S3-compatible, replaces MinIO endpoint)
  Redis 7  (session cache, unchanged)
  OTel Collector  (OTLP gRPC, OTEL_EXPORTER_OTLP_ENDPOINT)
```

---

## Phase 1 — ObjectStore 接口设计

### 接口定义

```go
// internal/objstore/objstore.go
package objstore

import "context"

// ObjectStore is the storage abstraction for object/blob operations.
type ObjectStore interface {
    EnsureBucket(ctx context.Context) error
    Bucket() string
    Ping(ctx context.Context) error
    GetObject(ctx context.Context, key string) ([]byte, error)
    PutObject(ctx context.Context, key string, data []byte) error
    DeleteObject(ctx context.Context, key string) error
    ObjectExists(ctx context.Context, key string) (bool, error)
    ListObjects(ctx context.Context, prefix string) ([]string, error)
}
```

### 配置结构变更

```go
// Before:
MinIO    MinIOConfig    `yaml:"minio"`

// After:
ObjStore ObjStoreConfig `yaml:"objstore"`

type ObjStoreConfig struct {
    Endpoint  string `yaml:"endpoint"`   // RustFS / MinIO endpoint
    AccessKey string `yaml:"access_key"`
    SecretKey string `yaml:"secret_key"`
    Bucket    string `yaml:"bucket"`
    UseSSL    bool   `yaml:"use_ssl"`
}
```

**向后兼容策略**：yaml tag 可保留 `minio` alias，使旧配置文件无需改动。

### 调用方类型替换

所有 14 个文件的 `*objstore.MinIOClient` 字段类型改为 `objstore.ObjectStore` 接口。构造函数返回接口类型。

---

## Phase 2 — 可观测性架构

### pprof 端点设计

```
决策：独立 admin HTTP server（不挂载到 API Server）

┌─────────────────────────────────────────────┐
│  Admin Server (env HERMESX_ADMIN_PORT)       │
│  默认端口：6060（生产需 IP 白名单/VPN）       │
│                                              │
│  GET /debug/pprof/         — index          │
│  GET /debug/pprof/cmdline  — cmdline        │
│  GET /debug/pprof/profile  — CPU profile    │
│  GET /debug/pprof/symbol   — symbol         │
│  GET /debug/pprof/trace    — trace          │
│  GET /debug/pprof/{other}  — all pprof types│
└─────────────────────────────────────────────┘

实现方式: 直接使用 net/http/pprof 的 init() 副作用注册 + DefaultServeMux
仅当 HERMESX_ADMIN_PORT != "" 时启动
```

### Prometheus 新增指标

```go
// Gateway 事件
hermesx_gateway_events_total{platform, event_type}  // Counter

// Session 操作
hermesx_session_operations_total{operation, status}   // Counter
hermesx_session_operation_duration_seconds{operation} // Histogram

// ObjectStore 操作  
hermesx_objstore_operations_total{operation, status}  // Counter
hermesx_objstore_operation_duration_seconds{operation} // Histogram
```

### OTel Span 扩展

```
现有 span 覆盖:
  HTTP handler → llm.Chat → pgx.Query

目标补齐:
  HTTP handler
    └── store.Sessions.Create  ← NEW span
    └── store.Messages.Append  ← NEW span
    └── objstore.PutObject     ← NEW span
    └── objstore.GetObject     ← NEW span
```

**实现方式**：在 Store 接口的使用层（非 pg/ 实现层）通过 wrapper 或中间件添加 span，避免修改每个具体实现文件。

### requestId ACP 补挂

当前 API Server 已挂载，需检查 `internal/acp/server.go` 是否缺失。若缺失则在 ACP 路由链中补加 `RequestIDMiddleware`。

---

## Phase 3 — MySQL Adapter 架构

### 驱动注册模式（保持不变）

```go
// factory.go 现有机制
func NewStore(cfg StoreConfig) (Store, error) {
    driver, ok := drivers[cfg.Driver]  // "postgres" or "mysql"
    ...
}

// internal/store/mysql/mysql.go 新增注册
func init() {
    store.Register("mysql", func(cfg store.StoreConfig) (store.Store, error) {
        return New(cfg.URL)
    })
}
```

### tenantID 隔离策略（应用层）

```
PostgreSQL 模式（现有）:
  withTenantTx() → set_config('app.current_tenant', tenantID)
  → PostgreSQL RLS 策略自动过滤

MySQL 模式（新增）:
  每个 SQL 查询/命令显式追加 WHERE tenant_id = ?
  INSERT 语句包含 tenant_id 列
  无数据库级 RLS，完全由应用层负责

安全审计要求: MySQL 实现需代码 review 确认所有方法均有 tenant_id WHERE 注入
```

### SQL 方言差异映射

| PostgreSQL | MySQL 等效 |
|-----------|----------|
| `$1, $2, $3` | `?, ?, ?` |
| `RETURNING id` | 无原生支持，用 `sql.Result.LastInsertId()` |
| `ON CONFLICT ... DO UPDATE` | `INSERT ... ON DUPLICATE KEY UPDATE` |
| `gen_random_uuid()` | `UUID()` 或应用层生成 |
| `COALESCE(NULLIF($1,'')::uuid, ...)` | 应用层 Go 处理 |
| `ts_headline / plainto_tsquery / @@` | `LIKE '%keyword%'`（降级） |
| `pgx.ErrNoRows` | `sql.ErrNoRows` |
| `tag.RowsAffected()` | `sql.Result.RowsAffected()` |

### 目录结构

```
internal/store/
├── store.go           (接口，不变)
├── types.go           (移除 PoolProvider 或迁移至 pg/)
├── factory.go         (注册 "mysql" 驱动入口)
├── pg/                (现有，不变)
└── mysql/             (新增)
    ├── mysql.go       (MySQLStore struct + New() + 驱动注册)
    ├── migrate.go     (MySQL DDL 迁移)
    ├── sessions.go
    ├── messages.go
    ├── memories.go
    ├── users.go
    ├── roles.go
    ├── tenant.go
    ├── auditlog.go
    ├── apikey.go
    ├── cronjobs.go
    ├── execution_receipts.go
    ├── userprofiles.go
    └── pricing.go
```

---

## 接口约定

| 接口 | 协议 | 新增 |
|------|------|------|
| `objstore.ObjectStore` | Go interface | ✅ Phase 1 新增 |
| `store.Store` (13 sub-interfaces) | Go interface | 不变 |
| `GET /debug/pprof/*` | HTTP | ✅ Phase 2 新增（admin port） |
| `GET /metrics` | HTTP (Prometheus) | 扩展新指标 |
| OTLP gRPC | gRPC | 扩展 span 范围 |

---

## 技术选型

| 决策 | 选择 | 理由 |
|------|------|------|
| 对象存储 SDK | minio-go（保留）| RustFS S3 兼容，SDK 零改动 |
| MySQL 驱动 | `github.com/go-sql-driver/mysql` + `database/sql` | 标准 Go SQL 接口，无 pgx 依赖 |
| MySQL UPSERT | `INSERT ... ON DUPLICATE KEY UPDATE` | MySQL 8.0+ 原生支持 |
| MySQL 搜索降级 | `LIKE '%keyword%'` | 功能降级优于引入 MySQL 全文索引配置复杂度 |
| pprof 部署 | 独立 admin server，env-gated | 避免 API Server 意外暴露，生产可不启动 |

---

## 风险与约束

| 风险 | 类型 | 缓解 |
|------|------|------|
| RustFS S3 multipart/presigned URL 兼容性 | 技术不确定性 | Phase 1 集成测试需覆盖 minio-go 核心操作子集 |
| MySQL UPSERT 语义与 pg ON CONFLICT 不完全等价 | 行为差异 | 逐一测试 MemoryStore.Upsert 等关键 UPSERT 路径 |
| pprof admin server 生产暴露 | 安全 | 默认 disabled（HERMESX_ADMIN_PORT="" 时不启动） |
| Store span wrapper 引入额外 alloc | 性能 | 使用 otel.Tracer.Start noop 模式（无 OTLP 配置时零开销） |
