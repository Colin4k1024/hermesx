# HermesX Developer Guide

> 本文档面向开发者和架构师，梳理项目文档全景、演进脉络、构建命令、代码修改流程和架构审查最佳实践。

---

## 1. 文档清单

### 1.1 根目录文档

| 文件 | 用途 |
|------|------|
| `README.md` | 项目首页、功能矩阵、发布状态、安装说明 |
| `ARCHITECTURE.md` | 架构入口（指向 `docs/architecture.md`） |
| `ROADMAP.md` | 产品路线图和近期优先级 |
| `ENTERPRISE_READINESS.md` | 企业就绪评估矩阵（12 个能力域） |
| `SECURITY.md` | 安全报告流程 |
| `SECURITY_MODEL.md` | 威胁模型、认证链、信任边界 |
| `RBAC_MATRIX.md` | 5 角色 × 10 资源权限矩阵 |
| `GOVERNANCE.md` | 治理结构 |
| `CONTRIBUTING.md` | 贡献指南 |
| `CODE_OF_CONDUCT.md` | 社区行为规范 |
| `SUPPORT.md` | 支持渠道 |
| `AGENTS.md` | Claude Code 记忆上下文 |

### 1.2 核心文档 (`docs/`)

所有文档提供中英双语版本（`.md` = 中文，`.en.md` = English）。

**架构与设计：**

| 文件 | 内容 |
|------|------|
| `architecture.md` | 双运行模式（CLI/SaaS）、9 层中间件栈、租户隔离 |
| `AGENT_FIRST_ARCHITECTURE.md` | 6 层架构：Entry → Governance → Runtime → Execution → Workflow → Operations |
| `WORKFLOW_AGENT_BOUNDARY.md` | 工作流与 Agent 的职责边界 |
| `EXECUTION_RECEIPTS.md` | 审计回执格式规范 |

**配置与部署：**

| 文件 | 内容 |
|------|------|
| `saas-quickstart.md` | SaaS 部署快速开始 |
| `configuration.md` | 环境变量与配置项参考 |
| `authentication.md` | 认证机制和链路 |
| `deployment.md` | 部署指南（单节点/集群/K8s） |
| `database.md` | 数据库 schema、迁移和 RLS 策略 |

**运维与安全：**

| 文件 | 内容 |
|------|------|
| `observability.md` | 可观测性栈（OTel + Prometheus + Grafana + Jaeger） |
| `SECURITY_MODEL.md` | 威胁模型和 RLS 行级安全 |
| `RBAC_MATRIX.md` | 角色权限矩阵 |
| `ENTERPRISE_READINESS.md` | 企业评估 |

**功能指南：**

| 文件 | 内容 |
|------|------|
| `skills-guide.md` | 技能（工具）开发和部署 |
| `scheduler-guide.md` | 分布式定时任务引擎 |
| `workflow-guide.md` | DAG 工作流引擎 |
| `api-reference.md` | OpenAPI 参考 |

**ADR（架构决策记录）：**

| ADR | 主题 |
|-----|------|
| `adr/ADR-002` | 双层限流器 |
| `adr/ADR-003` | WebUI Vue3 渐进迁移 |
| `adr/ADR-004` | 多页 Vite 架构 |
| `adr/ADR-005` | Bootstrap 端点设计 |
| `adr/ADR-006` | WASM 安全层延期 |
| `adr/ADR-007` | 插件治理 |

**变更日志：**

| 文件 | 内容 |
|------|------|
| `CHANGELOG.md` | 完整版本历史（v2.0.0 → v2.4.0-dev） |

### 1.3 运维手册 (`site/runbooks/`)

| 文件 | 内容 |
|------|------|
| `backend-enterprise-validation-matrix.md` | 后端企业验证矩阵 |
| `enterprise-OIDC-integration-test-plan.md` | OIDC 集成测试计划 |
| `mysql-backup-restore.md` | MySQL 备份恢复 |
| `pg-pitr-recovery.md` | PostgreSQL PITR 恢复 |
| `platform-governance-center.md` | 平台治理中心 |
| `fix-hermesx-rls-policies.sql` | RLS 策略修复脚本 |

### 1.4 项目记忆 (`docs/memory/`)

| 文件 | 用途 |
|------|------|
| `project-context.md` | 当前任务、版本目标、风险（持续更新） |
| `lessons-learned.md` | 经验沉淀 |
| `backlog.md` | 待办与技术债 |
| `sessions/*.md` | 历史工作会话摘要 |

---

## 2. 项目演进：从个人助手到 SaaS 企业平台

### 2.1 演进时间线

```
┌──────────────────────────────────────────────────────────────────────────┐
│  阶段 1：个人 CLI 助手 (hermes-agent)                                    │
│  ─ 单用户交互式 Agent                                                    │
│  ─ SQLite 本地存储                                                        │
│  ─ 无租户概念、无审计                                                    │
│  ─ 直接调用 LLM API                                                      │
└────────────────────┬─────────────────────────────────────────────────────┘
                     │  Go 完全重写 (100% feature parity)
                     ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  阶段 2：HermesX v2.0.0 — 企业化改造                                    │
│  ─ 重命名 hermes → hermesx                                               │
│  ─ 多租户隔离 (PostgreSQL RLS)                                           │
│  ─ RBAC 5 角色权限体系                                                   │
│  ─ 审计回执 (ExecutionReceipt)                                           │
│  ─ Prometheus 业务指标                                                    │
│  ─ 生产级 Docker Compose                                                 │
│  ─ OpenAPI 规范                                                           │
└────────────────────┬─────────────────────────────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  阶段 3：v2.2.0 — 安全加固 + WebUI                                      │
│  ─ Supply-chain hardening (SHA pinning)                                  │
│  ─ Bootstrap 限流和幂等                                                   │
│  ─ React Admin/User WebUI                                                │
│  ─ API Key scope 治理                                                    │
│  ─ 内存 O(n²) 性能修复                                                   │
└────────────────────┬─────────────────────────────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  阶段 4：v2.3.0 — 分布式调度 + 安全纵深                                 │
│  ─ SaaS Cron Scheduler (gocron + Redis 分布式锁)                         │
│  ─ IronClaw 安全集成 (prompt injection defense)                          │
│  ─ Eino ADK POC (Agent 框架集成)                                         │
│  ─ 工作流引擎 (DAG-based SOP)                                            │
└────────────────────┬─────────────────────────────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  阶段 5：v2.4.0-dev — 当前开发                                           │
│  ─ Eino 0.9 Agent 主链                                                   │
│  ─ K8s Job 沙箱模式                                                      │
│  ─ 可观测性栈 (OTel + Grafana)                                           │
│  ─ Trusted Channel Login (飞书/企微/微信)                                │
│  ─ hermes-agent v0.15.2 upstream 吸收                                    │
│  ─ Channel Apps 管理 WebUI                                               │
└──────────────────────────────────────────────────────────────────────────┘
```

### 2.2 关键转折点

| 转折 | 变化 | 架构影响 |
|------|------|----------|
| Go 完全重写 | Python → Go | 编译型、并发安全、单二进制部署 |
| SaaS 模式引入 | 单用户 → 多租户 | PostgreSQL RLS、Redis 分布式锁、MinIO 对象存储 |
| 中间件栈 | 无治理 → 9 层强制顺序 | Tracing→Metrics→RequestID→Auth→Tenant→Logging→Audit→RBAC→RateLimit |
| 双运行模式 | CLI only → CLI + SaaS HTTP | 同一二进制、配置驱动切换 |
| Channel Auth | API Key only → OAuth 渠道登录 | 飞书/企微/微信三方 OAuth、Session Cookie、CSRF |

### 2.3 包结构演进

```
internal/
├── 核心 Agent 运行时
│   ├── agent/           # Agent loop
│   ├── agentruntime/    # 执行引擎
│   ├── eino/            # Eino 框架集成
│   └── gateway/         # 多平台网关
│
├── 治理层 (v2.0+ 新增)
│   ├── auth/            # 认证 (API Key + Channel Session)
│   ├── permissions/     # RBAC
│   ├── middleware/      # 9 层中间件
│   ├── metering/        # 用量计量
│   └── safety/          # 安全防护
│
├── 企业基础设施 (v2.0+ 新增)
│   ├── store/pg/        # PostgreSQL + RLS
│   ├── store/mysql/     # MySQL 适配
│   ├── store/rediscache/# Redis 缓存
│   ├── objstore/        # MinIO 对象存储
│   └── observability/   # OTel 集成
│
├── 自动化层 (v2.3+ 新增)
│   ├── workflow/        # DAG 工作流引擎
│   ├── scheduler/       # 分布式定时调度
│   ├── cron/            # Cron 表达式解析
│   └── jobs/            # 任务执行
│
└── 渠道层 (v2.4+ 新增)
    ├── channel/         # Provider 管理、Challenge 验证
    └── auth/channel_session.go  # Session 提取
```

---

## 3. 构建命令速查

### 3.1 后端 (Go)

```bash
# 编译
make build                    # → ./hermesx 二进制

# 测试
make test                     # 全量测试 (verbose)
make test-short               # 短测试 (跳过慢集成)
make test-race                # 竞态检测
make test-cover               # 覆盖率报告

# 代码质量
make lint                     # go vet ./...
make fmt                      # go fmt ./...

# 集成测试 (需要 Docker)
make test-infra-up            # 启动 PG/Redis/MinIO (隔离端口)
make test-integration         # 运行集成测试
make test-infra-down          # 清理

# 交叉编译
make build-linux              # amd64 + arm64
make build-darwin             # amd64 + arm64
make build-all                # 全平台
```

### 3.2 前端 (WebUI)

```bash
cd webui/

npm run dev                   # 开发服务器 (Vite HMR)
npm run build                 # TypeScript 编译 + Vite 生产构建
npm run typecheck             # 仅类型检查
npm run lint                  # ESLint
npm run preview               # 预览构建产物
```

### 3.3 文档站点

```bash
# 依赖安装
pip install mkdocs-material mkdocs-i18n

# 本地预览
mkdocs serve                  # http://localhost:8000

# 构建静态站
mkdocs build                  # 输出到 site/

# 部署到 GitHub Pages (CI 自动)
# .github/workflows/pages.yml 在 push main 时触发
```

### 3.4 Docker 一键启动

```bash
# 快速体验 (API + 基础设施 + Bootstrap)
make quickstart               # 首次会自动创建 .env

# WebUI 完整体验
make webui                    # http://localhost:3000

# E2E 测试
make test-e2e                 # 13 个 Playwright 隔离测试

# 清理
make teardown                 # 清除所有 quickstart 容器和卷
make webui-teardown           # 清除 webui 容器
```

### 3.5 可观测性栈

```bash
# 启动监控全家桶
docker compose -f docker-compose.observability.yml up -d

# 访问入口
# Grafana:    http://localhost:3001
# Prometheus: http://localhost:9090
# Jaeger:     http://localhost:16686
# OTel:       gRPC:4317, HTTP:4318
```

---

## 4. 代码修改流程

### 4.1 标准工作流

```
1. 理解现状
   └─ 读 docs/memory/project-context.md → 当前版本目标、风险
   └─ 读 docs/CHANGELOG.md → 近期变更上下文
   └─ 读相关 ADR → 历史决策约束

2. 定位代码
   └─ codegraph_context("你的任务描述") → 快速定位入口
   └─ codegraph_trace(from, to) → 追踪调用链
   └─ codegraph_impact(symbol) → 评估变更影响面

3. 修改实现
   └─ 遵循 internal/ 包边界，不跨层直接调用
   └─ 新增 API → 同步更新 OpenAPI spec
   └─ 新增 store 操作 → 考虑 RLS 策略
   └─ 新增中间件 → 维护 9 层顺序约定

4. 验证
   └─ make test → 全量通过
   └─ make lint && make fmt → 零 warning
   └─ 前端: npm run build → 零 error
   └─ 涉及安全: govulncheck ./...

5. 提交
   └─ feat/fix/refactor/docs/style/chore 前缀
   └─ 涉及多租户: 说明 RLS 影响
   └─ 涉及 API: 说明兼容性
```

### 4.2 新增 API 端点检查清单

- [ ] `internal/api/` 或 `internal/api/admin/` 添加 handler
- [ ] 路由注册在 `server.go` 的正确位置（中间件栈之后）
- [ ] RBAC scope 定义（`handle("METHOD /path", []string{"scope:read"}, handler)`）
- [ ] 请求体限制（`http.MaxBytesReader`）
- [ ] 租户边界校验（admin API 用 query param，user API 从 context 取）
- [ ] 审计日志（通过 `store.AuditLogs().Append()`）
- [ ] OpenAPI 文档更新
- [ ] WebUI 页面（如需要前端展示）
- [ ] 集成测试或 E2E 测试

### 4.3 前端页面修改检查清单

- [ ] 放在正确目录（`admin/pages/` 或 `user/pages/`）
- [ ] 路由注册（`router.tsx`）
- [ ] 导航入口（`AdminShell.tsx` 或 `UserShell`）
- [ ] 使用 `apiClient` 统一请求（不直接 fetch）
- [ ] Loading / Error / Empty 三态处理
- [ ] 503 降级提示（feature 未启用时）
- [ ] `npm run build` 零报错

---

## 5. 架构审查最佳实践

### 5.1 审查维度

| 维度 | 关注点 |
|------|--------|
| **租户隔离** | RLS 策略覆盖、`current_tenant` 设置、跨租户泄漏 |
| **认证链** | API Key → JWT → Channel Session 优先级、401/403 语义 |
| **中间件顺序** | 9 层强制顺序不可打乱，新中间件需要说明插入点 |
| **安全边界** | 输入限制、constant-time 比较、body 大小限制、CSRF |
| **可观测性** | trace propagation、metric label 基数、日志脱敏 |
| **向后兼容** | API 契约稳定、migration 只增不删、配置 fallback |

### 5.2 ADR 触发条件

以下场景必须新建 ADR：

- 引入新的外部依赖或框架
- 改变数据存储后端或 schema 设计模式
- 变更认证/授权机制
- 新增运行模式或部署拓扑
- 偏离已有 ADR 的决策

ADR 模板位于 `docs/adr/`，编号递增。

### 5.3 安全审查红线

```
绝对不允许：
├── 明文存储 secret（只存 ref，通过 SecretResolver 解析）
├── 跳过 RLS（所有写操作必须设置 current_tenant）
├── 未限制 request body（至少 1 MiB hard cap）
├── 非 constant-time 的 token 比较
├── 信任 client 传入的 tenant_id（user API 从 auth context 取）
└── 在日志/错误响应中暴露内部路径或 secret
```

### 5.4 变更影响评估

```bash
# 查看某个 symbol 的变更影响面
# codegraph_impact("ChannelSessionExtractor") → 列出所有下游依赖

# 查看完整调用链
# codegraph_trace("ChannelSessionExtractor.Extract", "AuthContext")

# 查看谁调用了这个函数
# codegraph_callers("createChannelApp")
```

### 5.5 版本发布检查

| 检查项 | 命令 |
|--------|------|
| 全量测试通过 | `make test` |
| 竞态安全 | `make test-race` |
| 代码格式 | `make fmt && make lint` |
| 安全漏洞 | `govulncheck ./...` |
| 前端构建 | `cd webui && npm run build` |
| E2E 通过 | `make quickstart && make test-e2e` |
| 文档同步 | `mkdocs build` (无 warning) |
| CHANGELOG 更新 | 手动确认 unreleased 段落完整 |

---

## 6. 技术栈速查

| 层 | 技术 |
|----|------|
| 语言 | Go 1.25 (后端), TypeScript 5.6 (前端) |
| 前端框架 | React 18 + Ant Design 5 + TanStack Query + Zustand |
| 构建工具 | Vite 6 + Tailwind CSS 3 |
| 数据库 | PostgreSQL 16 (主) / MySQL 8 / SQLite (CLI) |
| 缓存 | Redis 7 |
| 对象存储 | MinIO (S3 兼容) |
| 可观测 | OpenTelemetry + Prometheus + Grafana + Jaeger |
| CI/CD | GitHub Actions (test/lint/security/release/pages) |
| 文档 | MkDocs Material (中英双语) |
| 测试 | Go test + Playwright E2E |
| 容器 | Docker Compose (7 profile) |

---

## 7. 常用开发场景

### 场景 A：新增一个 Admin API

```bash
# 1. 在 internal/api/admin/ 新增 handler 文件
# 2. 在 internal/api/admin/handler.go 注册路由
# 3. 如需持久化：在 internal/store/ 添加接口和实现
# 4. 如需 migration：在 db/migrations/ 添加 SQL
# 5. make test && make lint
# 6. 可选：在 webui/src/admin/pages/ 添加管理页面
# 7. npm run build
```

### 场景 B：新增 WebUI 页面

```bash
# 1. 创建 webui/src/admin/pages/NewPage.tsx
# 2. 在 router.tsx 添加 lazy import 和路由
# 3. 在 AdminShell.tsx 添加导航项
# 4. npm run build 验证
```

### 场景 C：修改数据库 Schema

```bash
# 1. 在 db/migrations/ 添加新 migration 文件（编号递增）
# 2. 包含 RLS 策略（ENABLE + FORCE + USING + WITH CHECK）
# 3. 在 internal/store/pg/ 和 internal/store/mysql/ 实现新接口
# 4. make test-integration 验证
```

### 场景 D：文档更新

```bash
# 1. 编辑 docs/*.md（中文版）和 docs/*.en.md（英文版）
# 2. mkdocs serve 本地预览
# 3. 提交后 GitHub Pages workflow 自动部署
```
