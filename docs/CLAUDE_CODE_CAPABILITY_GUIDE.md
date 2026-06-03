# HermesX Claude Code 能力操作手册：迁移、重构、升级与架构转型

本报告说明如何利用当前 Claude Code 中已配置的 commands、agents、skills 和 MCP 服务，系统化地完成 HermesX 项目的迁移、重构、版本升级和架构改造工作。基于实际探索 `~/.claude/` 配置产出，非模板堆砌。

---

## 一、工具链全景

### 1.1 命令层 (Commands) — 84 个

| 分类 | 命令 | 用途 |
|------|------|------|
| **主链生命周期** | `/team-intake` → `/team-plan` → `/team-execute` → `/team-review` → `/team-release` → `/team-closeout` | 完整交付链路（需求→设计→实现→评审→发布→收口） |
| **快速模式** | `/quick` | ≤3文件/≤100行的小改动，跳过完整链路 |
| **辅助入口** | `/team-help` | 根据当前阶段推荐下一步命令 |
| **Go 专项** | `/go-build`, `/go-test`, `/go-review` | 构建修复 / TDD / 代码审查 |
| **重构清理** | `/refactor-clean` | 死代码识别 + 安全删除循环 |
| **通用规划** | `/plan`, `/prp-prd`, `/prp-plan` | 实现计划 / PRD 生成 / 详细规划 |
| **代码审查** | `/code-review`, `/quality-gate` | 综合审查 / 质量门禁 |
| **TDD** | `/tdd`, `/go-test` | 测试驱动开发 |
| **E2E** | `/e2e` | Playwright 端到端测试 |
| **文档** | `/update-docs`, `/update-codemaps` | 文档/代码地图更新 |
| **验证** | `/verify` | 全量验证循环 |
| **Handoff** | `/handoff`, `/promote` | 角色交接 / 变更升级 |
| **多团队** | `/multi-plan`, `/multi-execute`, `/multi-backend`, `/multi-frontend` | 多模块并行编排 |
| **会话管理** | `/save-session`, `/resume-session`, `/checkpoint` | 断点续传 |

### 1.2 角色代理 (Agents) — 54 个

| 角色 | 代理名 | 职责 |
|------|--------|------|
| **技术负责人** | `tech-lead` | 仲裁冲突、收口决策、组织 Challenge/Review |
| **架构师** | `architect` | 系统边界、组件拆分、接口契约 |
| **后端工程师** | `backend-engineer` | Go 后端实现 |
| **前端工程师** | `frontend-engineer` | React/TS 前端实现 |
| **QA** | `qa-engineer` | 测试计划、放行建议 |
| **DevOps** | `devops-engineer` | 部署、监控、回滚 |
| **代码审查** | `code-reviewer` | 代码质量 |
| **安全审查** | `security-reviewer` | OWASP Top 10、凭证泄漏 |
| **性能优化** | `performance-optimizer` | 瓶颈识别、优化建议 |
| **重构清理** | `refactor-cleaner` | knip/deadcode + 安全删除 |
| **DB 审查** | `database-reviewer` | SQL/Schema/索引优化 |
| **Go 审查** | `go-reviewer` | 并发、错误处理、惯用法 |
| **TS 审查** | `typescript-reviewer` | 类型安全、async 正确性 |
| **GSD 系列** | `gsd-planner`, `gsd-executor`, `gsd-verifier`, `gsd-debugger`, `gsd-roadmapper` 等 18 个 | 完整项目交付编排 |

### 1.3 技能 (Skills) — 10 本地 + 264 市场

**本地核心技能：**

| 技能 | 用途 |
|------|------|
| `codegraph` | 符号搜索、调用链追踪、影响面分析（MCP-backed） |
| `gitnexus` | 跨模块深度代码图谱分析（brownfield 项目） |
| `open-design` | 外部设计工作台集成 |
| `axi-front-design` | HTML 原型 / 落地页设计 |

**市场技能（与迁移/重构相关）：**
`database-migrations`, `batch-path-refactor-checklist`, `deployment-patterns`, `doc-architecture`, `api-design`, `architect`, `code-review-excellence`, `tdd-workflow`, `quick-execution`

### 1.4 MCP 服务

| 服务 | 能力 |
|------|------|
| **Codegraph** | `codegraph_context` / `codegraph_trace` / `codegraph_impact` / `codegraph_callers` / `codegraph_callees` |
| **Context7** | 最新库文档查询（Go/React/任何框架） |
| **DrawIO** | 架构图创建与编辑 |
| **Claude-mem** | 知识持久化、跨会话记忆搜索 |

---

## 二、核心工作流：/team-* 生命周期

```
/team-help (阶段路由)
    │
    ▼
/team-intake ──→ /team-plan ──→ /handoff ──→ /team-execute ──→ /team-review ──→ /team-release ──→ /team-closeout
   PRD              Arch Design    Ready?       Code + Test        QA + Gate       Deploy + Monitor    Lessons + Backlog
```

**进入条件：**

| 阶段 | 前置 |
|------|------|
| `/team-plan` | PRD 存在、关键项收敛 |
| `/team-execute` | delivery-plan + handoff-ready + `workflow:readiness` 通过 |
| `/team-review` | execute-log 写入、自测通过 |
| `/team-release` | 放行建议为"建议放行"、deployment-context 就绪 |
| `/team-closeout` | 发布观察窗口结束 |

**产出物落盘：** `docs/artifacts/{YYYY-MM-DD}-{slug}/` + `docs/memory/` 持续更新

---

## 三、场景一：完整项目迁移

> 例：单体 → 微服务拆分、SQLite → PostgreSQL 迁移

### 推荐流程

```
Step 1: /team-intake
  ├─ architect agent: 边界分析
  ├─ codegraph_context("service boundaries")：识别耦合点
  └─ 产出: PRD + 服务拆分方案

Step 2: /team-plan
  ├─ Requirement Challenge Session
  ├─ codegraph_trace(from→to): 追踪跨模块调用链
  ├─ DrawIO: 绘制目标架构图
  ├─ Context7: 查新依赖文档
  └─ 产出: delivery-plan.md, arch-design.md, ADR

Step 3: /team-execute (分阶段)
  ├─ Phase 1: 接口抽象层 → /tdd + /go-test
  ├─ Phase 2: 新实现 → /go-build + /verify
  ├─ Phase 3: 数据迁移脚本 → database-reviewer 审查
  ├─ Phase 4: 切换 + 旧代码清理 → /refactor-clean
  └─ 每阶段: /handoff 交接

Step 4: /team-review
  ├─ /code-review + /go-review
  ├─ security-reviewer: 新攻击面
  ├─ /e2e: 全链路集成
  └─ /quality-gate

Step 5: /team-release → Step 6: /team-closeout
```

### 关键命令链

```bash
# 1. 理解当前架构
codegraph_context → codegraph_impact → codegraph_trace

# 2. 规划拆分
/team-intake → /team-plan

# 3. 分阶段实现
/tdd → /go-build → /go-test → /verify → /handoff (循环)

# 4. 清理旧代码
/refactor-clean (deadcode 工具: `deadcode ./...`)

# 5. 验证 + 发布
/e2e → /quality-gate → /team-release
```

---

## 四、场景二：模块级重构

> 例：重写 store 层、替换中间件、重构 handler

### 推荐流程

```
Step 1: 影响面分析
  └─ codegraph_impact("TargetModule") → 列出所有受影响符号

Step 2: 接口锁定
  └─ codegraph_callers("TargetInterface") → 确认所有消费者

Step 3: TDD 先行
  └─ /tdd → 为新行为写测试 (RED)

Step 4: 实现替换
  └─ /go-build → 编译通过 (GREEN)
  └─ /go-test → 测试通过

Step 5: 清理旧实现
  └─ /refactor-clean → 安全删除循环

Step 6: 验证
  └─ /verify → 全量回归
```

### 快速模式判断

| 条件 | 走哪条路 |
|------|----------|
| ≤3 文件、≤100 行、无 API 契约变更 | `/quick` |
| 涉及接口变更或多消费者 | 完整 `/team-plan` → `/team-execute` |
| 跨模块影响 | `/multi-plan` → `/multi-execute` |

---

## 五、场景三：版本升级

> 例：Go 1.24 → 1.25、依赖大版本升级、React 18 → 19

### 推荐流程

```
Step 1: 文档研究
  └─ Context7: resolve-library-id → query-docs (breaking changes)

Step 2: 影响面扫描
  └─ codegraph_files: 列出相关文件
  └─ /go-build: 编译检测

Step 3: 修复 + 验证
  └─ /go-build: 修复编译错误
  └─ /go-test: 跑全量测试 + -race
  └─ /verify: 行为回归验证

Step 4: 质量门禁
  └─ /quality-gate: 覆盖率 + vet + lint
  └─ /code-review: 变更审查
```

### 命令链（Go 升级）

```bash
# 查文档
Context7: query-docs("/golang/go", "migration guide 1.25")

# 编译修复循环
/go-build  (自动修复编译错误)

# 测试验证
/go-test   (table-driven tests + race detection)

# 最终确认
/verify → /quality-gate
```

### 命令链（前端依赖升级）

```bash
# 查文档
Context7: query-docs("/vercel/next.js", "upgrade from v14 to v15")

# 构建修复
npm run build → 逐个修复

# 验证
/e2e → /quality-gate
```

---

## 六、场景四：架构转型

> 例：新增子系统（Channel Auth）、变更数据层（MinIO → RustFS）、加入新中间件

### 推荐流程

```
Step 1: /team-intake
  ├─ /prp-prd: 生成 PRD
  ├─ architect agent: 设计新子系统边界
  ├─ codegraph_context: 理解集成点
  └─ 产出: PRD + 初步接口定义

Step 2: /team-plan
  ├─ Requirement Challenge (architect vs tech-lead)
  ├─ ADR: 记录关键技术决策
  ├─ api-contract.md: 新接口契约
  ├─ DrawIO: 数据流图
  └─ 产出: delivery-plan.md, arch-design.md, ADR-NNN

Step 3: /team-execute (分层)
  ├─ Layer 1: Store/Interface 层 → /tdd
  ├─ Layer 2: Service 业务逻辑 → /go-test
  ├─ Layer 3: API/Handler 层 → /go-build
  ├─ Layer 4: 中间件集成 → /verify
  └─ 每层完成后 /code-review

Step 4: /team-review
  ├─ security-reviewer: 新攻击面评估
  ├─ database-reviewer: Schema/索引审查
  ├─ /e2e: 全系统集成
  └─ 产出: test-plan.md, launch-acceptance.md

Step 5: /team-release + /team-closeout
  └─ /update-docs + /update-codemaps: 架构文档同步
```

### HermesX 实际案例

| 版本 | 架构变更 | 使用的命令/技能 |
|------|----------|----------------|
| v2.1.0 | ObjectStore 接口 + RustFS 替换 MinIO | `/team-plan` → `codegraph_trace` → `/tdd` → `/team-execute` |
| v2.3.0 | IronClaw 安全三包集成 | `/team-intake` → `security-reviewer` → `/team-execute` → `/quality-gate` |
| v2.4.0 | Trusted Channel Login 子系统 | `/team-plan` → `architect` → `/go-test` → `/team-review` |

---

## 七、快速参考

### 7.1 按阶段的命令选择

| 我想... | 用什么命令 |
|---------|-----------|
| 了解当前应该做什么 | `/team-help` |
| 快速修个小 bug | `/quick` |
| 启动新需求 | `/team-intake` |
| 做架构设计 | `/team-plan` + `architect` agent |
| 开始写代码 | `/team-execute` |
| 代码写完要审查 | `/team-review` + `/code-review` |
| 准备发布 | `/team-release` |
| 项目收尾 | `/team-closeout` |

### 7.2 按场景的命令组合 (Recipes)

**Recipe A: 理解陌生模块**
```
codegraph_context("模块名") → codegraph_explore("关键符号") → codegraph_trace(from, to)
```

**Recipe B: 安全重构**
```
codegraph_impact("要改的符号") → /tdd (先写测试) → 改代码 → /go-test → /refactor-clean
```

**Recipe C: 升级依赖**
```
Context7(库文档) → /go-build (修编译) → /go-test (跑测试) → /verify
```

**Recipe D: 新增 API**
```
/team-plan (api-contract.md) → /tdd → /go-build → /e2e → /code-review
```

**Recipe E: 前端页面新增**
```
/team-plan → frontend-engineer agent → /tdd → /e2e → /code-review
```

**Recipe F: 全量代码清理**
```
/refactor-clean → deadcode ./... → 逐个安全删除 → /verify
```

### 7.3 常见误用与修正

| 误用 | 修正 |
|------|------|
| 直接 `/team-execute` 没有 handoff | 先 `/team-plan` + `/handoff` |
| 大改动用 `/quick` | 超过 3 文件必须走 `/team-plan` |
| 只做 `/code-review` 不做 `/verify` | `/verify` 包含完整回归验证 |
| 改了架构不写 ADR | `/team-plan` 阶段必须产出 ADR |
| 忽略 `/update-codemaps` | 架构变更后必须更新代码地图 |

---

## 八、附录

### A. 产出物模板清单

| 模板 | 路径 | 用途 |
|------|------|------|
| `api-contract.md` | `~/.claude/templates/` | 接口契约 |
| `deployment-context.md` | `~/.claude/templates/` | 部署上下文 |
| `release-plan.md` | `~/.claude/templates/` | 发布计划 |
| `closeout-summary.md` | `~/.claude/templates/` | 收口总结 |
| `design-system-brief.md` | `~/.claude/templates/` | 设计系统摘要 |
| `ui-implementation-plan.md` | `~/.claude/templates/` | UI 实现计划 |
| `ui-review-checklist.md` | `~/.claude/templates/` | UI 评审清单 |
| `launch-acceptance.md` | `~/.claude/templates/` | 上线验收 |
| `backlog-snapshot.md` | `~/.claude/templates/` | Backlog 快照 |

### B. MCP 工具用法速查

```
# 理解架构
codegraph_context(task="channel auth 子系统如何工作")

# 追踪调用链
codegraph_trace(from="HandleChannelLogin", to="CreateBrowserSession")

# 分析影响面
codegraph_impact(symbol="ChannelStore", depth=3)

# 查找调用者
codegraph_callers(symbol="ValidateChallenge")

# 查库文档
Context7: resolve-library-id("Go") → query-docs(libraryId, "context cancellation patterns")

# 画架构图
DrawIO: create_new_diagram(xml="...")
```

### C. GSD 工作流（替代路径）

当 `/team-*` 链路感觉过重时，GSD agents 提供更轻量的选择：

```
gsd-roadmapper → gsd-planner → gsd-executor → gsd-verifier
```

适用于：个人项目、快速原型、不需要多角色协作的场景。

---

## 验证方式

1. 运行 `/team-help` 确认路由逻辑与本报告一致
2. 在任意场景中执行 `codegraph_context` 验证 MCP 可用性
3. 查看 `docs/artifacts/` 下已有交付物，确认模板结构匹配
4. 对比 `docs/memory/project-context.md` 中的版本历史与本报告案例
