# PRD: HermesX v2.1.0 基础设施升级

**状态**: intake  
**日期**: 2026-05-08  
**Owner**: tech-lead  
**Slug**: v21-infra-upgrade

---

## 背景

HermesX v2.0.0 完成品牌独立与安全加固。v2.1.0 聚焦三类基础设施演进：
1. 数据库多方言支持（MySQL 适配），降低部署门槛
2. 对象存储从 MinIO 切换到 RustFS，统一内部存储技术栈
3. 可观测性纵深补齐（pprof、requestId 贯通、logrus 迁移、Prometheus 扩展、OTel 完整化）

---

## 目标与成功标准

| 目标 | 成功标准 |
|------|---------|
| MySQL 适配 | 使用 MySQL 启动 HermesX，所有 Store 接口通过测试；PostgreSQL 仍可用 |
| RustFS 替换 MinIO | objstore 包切换至 RustFS endpoint，读写功能验证通过；MinIO SDK 版本复用（S3 兼容） |
| pprof 端点 | `/debug/pprof/*` 在 internal/admin 端口可访问，生产环境有访问控制 |
| requestId 贯通 | 每个请求 X-Request-ID 全链路可见（已有中间件，需确认 HTTP 链路是否已挂载） |
| 日志框架迁移 | logrus 替换 log/slog（529 处），统一日志格式与 level 控制 |
| Prometheus 扩展 | 补齐 gateway 事件、session 操作、objstore 操作的 metrics |
| OTel 完整化 | span 覆盖 HTTP handler → store → objstore 全链路；现有 tracer.go 基础上扩展 |

---

## 范围

**In Scope**
- MySQL adapter（`internal/store/mysql/`）实现所有 Store 子接口
- objstore 接口抽象 + RustFS 切换（endpoint/config 替换，SDK 兼容）
- pprof 注册到独立端口或 admin 路由组
- requestId 中间件挂载验证（已有实现，仅确认 wiring）
- logrus 迁移（替换 log/slog 全部 529 处调用）
- Prometheus metrics 补齐（gateway、session、objstore 层）
- OTel span 扩展（handler + store + objstore）

**Out of Scope**
- MySQL 专属特性（存储过程、JSON 函数差异优化）
- RustFS 本身的部署运维
- 分布式 tracing UI/dashboard 建设
- logrus 结构化日志字段规范制定（独立 sprint）

---

## 关键假设与挑战点

> 以下为 `karpathy-guidelines` 收敛后的显式假设，需在需求挑战会确认。

### A1 — logrus 迁移 ✅ 已决策
- **决策**（2026-05-08）：**放弃 logrus 迁移，保留 log/slog**
- **理由**：slog 是 Go 1.21+ 标准库，已支持结构化日志 + level 控制；logrus 引入额外依赖无实质收益，529 处迁移成本不可接受
- **影响**：Phase 4 从范围中移除，可观测性增强聚焦 pprof / requestId 贯通 / Prometheus / OTel

### A2 — MySQL RLS 兼容性 ✅ 已决策
- **决策**（2026-05-08）：**MySQL 场景使用应用层 tenant_id filter 实现多租户隔离**
- **策略**：Store 所有查询方法在 MySQL 实现中强制附加 `AND tenant_id = ?` 条件，不依赖数据库原生 RLS
- **影响**：MySQL adapter 实现时须在每个 Store 方法中注入 tenantID；需补安全审计确保无遗漏

### A3 — RustFS SDK 兼容性
- **现状**：RustFS 声称完全 S3 兼容，MinIO Go SDK 可直接指向 RustFS endpoint
- **假设**：只需替换 endpoint/credentials 配置，代码层零改动
- **待确认**：RustFS 是否通过 MinIO Go SDK 兼容性测试？multipart upload、presigned URL 是否正常？

### A4 — requestId 已有实现
- **发现**：`internal/middleware/requestid.go` 已存在完整实现（X-Request-ID + 自动生成 32 字节 hex）
- **待确认**：是否已挂载到 HTTP 服务链？若已挂载，此项可标记为 done，不计入 sprint 工作量

### A5 — OTel 已有基础
- **发现**：`internal/observability/tracer.go` 已集成 OTLP gRPC 导出，支持 `OTEL_EXPORTER_OTLP_ENDPOINT`
- **待确认**：当前 span 覆盖范围（仅 LLM client？还是已扩展到 handler 层）？

---

## 用户故事

1. **部署工程师**：我使用 MySQL 作为数据库部署 HermesX，期望无需修改代码即可通过环境变量切换
2. **运维工程师**：我使用 RustFS 替代 MinIO，期望仅修改 endpoint 配置即可完成切换
3. **SRE**：我能通过 `/debug/pprof` 在性能问题时快速定位 goroutine 泄漏和热点函数
4. **开发者**：每个请求的日志都包含 request_id，方便跨服务关联排查
5. **可观测平台**：所有关键操作的 span 和 metrics 完整，Grafana/Jaeger 可直接消费

---

## 风险与依赖

| 风险 | 影响 | 缓解 |
|------|------|------|
| logrus 迁移 529 处工作量失控 | 高 | 建议挑战该需求，评估 slog handler 方案 |
| MySQL 无 RLS 导致多租户安全漏洞 | 极高 | 明确 MySQL 模式不支持多租户，或在应用层补 tenant_id filter |
| RustFS 与 MinIO SDK 不兼容 | 中 | 先跑集成测试验证 S3 API 子集覆盖 |
| pprof 端点暴露到外网 | 高 | 强制要求 admin 端口或 IP allowlist |
| Store 接口不完整导致 MySQL 实现缺方法 | 中 | pg 实现作为参考，逐一对应接口方法 |

---

## 参与角色

| 角色 | 职责 |
|------|------|
| `tech-lead` | 挑战 logrus 需求、确认 MySQL RLS 策略、架构决策收口 |
| `architect` | Store 接口扩展设计、objstore 接口抽象方案、OTel span 扩展设计 |
| `backend-engineer` | MySQL adapter 实现、pprof 注册、Prometheus metrics 补齐 |
| `qa-engineer` | MySQL/PostgreSQL 切换回归、RustFS 集成测试、pprof 访问控制验证 |
| `devops-engineer` | RustFS 部署验证、pprof 端口访问控制、OTel collector 配置 |

---

## 待确认项

| # | 问题 | 目标角色 | 优先级 |
|---|------|---------|-------|
| Q1 | logrus 迁移的核心驱动是什么？能否用 slog handler 替代全量迁移？ | tech-lead | P0 | ✅ 放弃 logrus，保留 slog |
| Q2 | MySQL 场景是否需要多租户 RLS？ | tech-lead | P0 | ✅ 应用层 tenant_id filter |
| Q3 | requestId 中间件是否已挂载到 HTTP 服务链？ | backend-engineer | P1 |
| Q4 | OTel 当前 span 覆盖范围？ | backend-engineer | P1 |
| Q5 | RustFS SDK 兼容性是否已通过测试？ | devops-engineer | P1 |
| Q6 | pprof 端点目标端口/路由策略？独立端口还是 admin 路由组？ | tech-lead | P1 |
| Q7 | 三项变更是否拆成独立 sprint 还是合并交付？ | tech-lead | P2 |

---

## 需求挑战会候选分组

### 分组 A — 存储层（高风险，需独立讨论）
- MySQL adapter + RLS 策略
- Store 接口 MySQL 兼容方案
- 参与角色：tech-lead, architect, backend-engineer

### 分组 B — 对象存储（低风险，可快速收口）
- RustFS SDK 兼容验证
- objstore 接口抽象设计
- 参与角色：architect, devops-engineer

### 分组 C — 可观测性（范围最大，需拆分）
- logrus vs slog 最终决策
- pprof 端口策略
- OTel span 扩展范围
- Prometheus metrics gap 分析
- 参与角色：tech-lead, backend-engineer, devops-engineer

---

## 建议交付顺序（供 /team-plan 参考）

1. **Phase 1**（独立、低风险）：RustFS 替换 + objstore 接口抽象
2. **Phase 2**（中风险）：pprof + requestId 贯通验证 + Prometheus 补齐 + OTel 扩展
3. **Phase 3**（高风险、高工作量）：MySQL adapter（明确 RLS 策略后执行）
4. ~~**Phase 4**（已取消）：logrus 迁移~~ — 决策：保留 slog，Phase 4 从范围移除

---

## 企业治理待确认项

- pprof 端点需明确访问控制策略，避免生产泄露堆信息
- MySQL 多租户方案若需应用层 tenant_id filter，需补充安全审计
- RustFS 替换需确认数据迁移和备份策略
