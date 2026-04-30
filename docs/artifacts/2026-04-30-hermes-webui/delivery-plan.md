# Delivery Plan — Hermes Web UI

**版本**: v1.0  
**日期**: 2026-04-30  
**Owner**: tech-lead  
**Slug**: hermes-webui  
**阶段**: handoff-ready

---

## 版本目标

在 hermes-agent-go monorepo 的 `webui/` 子目录内交付一个可独立部署的 Vue 3 SPA，支持多租户多用户接入、聊天、记忆管理、技能管理和租户/API Key 管理（Admin）。

**放行标准**:
- Docker 镜像可一键运行，`HERMES_BACKEND_URL` 指向任意 hermes-agent-go 实例
- 连接、聊天、记忆、技能、Admin 页面全部可用
- Playwright api-isolation 套件 13/13 继续通过（后端零破坏）
- 所有页面实现 loading / empty / error / success 四态
- 键盘可导航，表单有显式 label

---

## 需求挑战会结论

**挑战时间**: 2026-04-30  
**参与角色**: product-manager, project-manager, architect

| # | 质疑 | 结论 |
|---|------|------|
| Q1 | Admin + 用户功能同一 MVP 范围过大 | 接受：Admin 页面复杂度低（CRUD 表格），不影响关键聊天路径 |
| Q2 | "无持久化" 对用户体验过于激进 | 修改：改用 sessionStorage（关闭 Tab 清除）作为折中方案 |
| Q3 | 无 BFF → API Key 暴露在 DevTools | 接受：内部工具场景，Key 可轮换，风险已记录 |
| Q4 | Go WriteTimeout=60s vs LLM 120s → HTML 502 崩溃 | 后端修复（前置条件）：WriteTimeout 已改为 150s |
| Q5 | X-Hermes-User-Id 自声明可伪造 | 接受：租户隔离由 API Key 强制，租户内用户分离是尽力而为 |
| Q6 | 非流式响应用户体验差 | 接受：v1 loading spinner + 禁用输入，SSE 进 backlog |
| Q7 | 并行化机会 | Sprint S0 完成后 S1/S2/S3 可并行（Auth store schema 是唯一跨层依赖） |

---

## 工作拆解

### Sprint S0 — 基础设施与脚手架（前置条件，约 1 天）

| 任务 | 主责 | 依赖 | 验收标准 |
|------|------|------|----------|
| [S0-1] 应用 Go WriteTimeout 150s 修复 | backend-engineer | 无 | server.go 修改，`go build` 通过，现有 E2E 13/13 仍通过 |
| [S0-2] `webui/` 目录初始化（Vite + Vue 3 + TS + Naive UI + Pinia） | frontend-engineer | 无 | `npm run dev` 启动，空页面可访问 |
| [S0-3] Pinia `auth` store + sessionStorage 读写 | frontend-engineer | S0-2 | `connect(apiKey, userId)` 调 `/v1/me`，成功/失败均有明确 state |
| [S0-4] `useApi.ts` 统一 fetch wrapper | frontend-engineer | S0-3 | 自动注入三个必需 header，502 HTML 检测，401 自动 disconnect |
| [S0-5] Vue Router + auth guard + hash mode | frontend-engineer | S0-3 | 未连接访问任意路由重定向 /connect；admin 路由非 admin 重定向 /chat |
| [S0-6] `webui/Dockerfile` 多阶段构建 | devops-engineer | S0-2 | `docker build webui/` 成功，`HERMES_BACKEND_URL` 可注入 |

### Sprint S1 — 聊天核心（约 2 天，依赖 S0）

| 任务 | 主责 | 依赖 | 验收标准 |
|------|------|------|----------|
| [S1-1] `ConnectPage` — API Key + UserId + ACP Token 表单 | frontend-engineer | S0-3/S0-5 | 连接成功跳 /chat；错误显示内联提示；4-state 完整 |
| [S1-2] `ChatPage` — SessionSidebar + MessageList + ChatInput | frontend-engineer | S0-4 | 发送消息，显示 loading，追加 assistant 回复；502 HTML 显示友好错误 |
| [S1-3] `chat` store — fetchSessions, selectSession, sendMessage | frontend-engineer | S0-4 | 4-state per operation；optimistic 用户消息；session ID 通过 header 传递 |
| [S1-4] 会话列表 sidebar — 新建会话、切换会话 | frontend-engineer | S1-3 | 切换会话加载历史消息；"新对话"清空 messages |

### Sprint S2 — 记忆与技能（约 1.5 天，可与 S1 并行）

| 任务 | 主责 | 依赖 | 验收标准 |
|------|------|------|----------|
| [S2-1] `MemoriesPage` — 列表 + 删除 | frontend-engineer | S0-4 | 显示 memory 列表；单条删除；4-state；delete loading per-key |
| [S2-2] `SkillsPage` — 只读列表 + 内容查看器 | frontend-engineer | S0-4 | 左侧技能列表；点击右侧显示内容；404 显示"未配置技能服务" |
| [S2-3] `/admin/skills` — 上传 + 删除技能 | frontend-engineer | S0-4 | 上传 .md 文件；`asAdmin: true` 使用 acpToken；成功刷新列表 |

### Sprint S3 — 管理员功能（约 1 天，可与 S1/S2 并行）

| 任务 | 主责 | 依赖 | 验收标准 |
|------|------|------|----------|
| [S3-1] `/admin/tenants` — 列表 + 创建租户 | frontend-engineer | S0-4 | 列表渲染；创建成功追加列表；4-state |
| [S3-2] `/admin/keys` — 列表 + 创建 + 撤销 | frontend-engineer | S0-4 | 创建 Key 后一次性展示原始值（复制确认后才可关闭弹窗） |

### Sprint S4 — 收口与部署（约 0.5 天）

| 任务 | 主责 | 依赖 | 验收标准 |
|------|------|------|----------|
| [S4-1] `docker-compose.webui.yml` 集成 | devops-engineer | S0-6 | `docker compose -f docker-compose.webui.yml up` 可访问所有页面 |
| [S4-2] UI 评审清单 + 可访问性自测 | frontend-engineer + qa-engineer | S3 | 键盘导航，表单有 label，4-state 验证，ui-review-checklist 填写完毕 |
| [S4-3] Playwright api-isolation 套件回归 | qa-engineer | S0-1 | 13/13 通过，无后端破坏 |

---

## 风险与缓解

| 风险 | 影响 | 缓解 | Owner |
|------|------|------|-------|
| API Key 浏览器暴露 | 安全（可接受） | sessionStorage + 可轮换 Key + 内部工具说明 | arch |
| Go WriteTimeout vs LLM 超时竞争 | 前端 JSON.parse 崩溃 | WriteTimeout 已改 150s；前端 Content-Type 检查 | backend |
| 无流式输出体验差 | 用户等待 | loading spinner + 禁用输入；SSE 进 backlog | product |
| Admin ACP Token 误输入 | 意外 admin 权限 | 字段明确标注"Admin only — leave blank" | frontend |
| API Key 一次性展示丢失 | 用户无法复用 Key | 创建后弹窗强制确认"已复制"才可关闭 | frontend |
| `/v1/audit-logs` 500 错误 | Admin 页面 tab 报错 | 各 tab 独立加载，单个 endpoint 失败不影响其他 tab | frontend |

---

## 节点检查

| 节点 | 条件 | 负责角色 |
|------|------|----------|
| S0 完成 | `go build` + `npm run dev` 均通过；WriteTimeout 已修复 | backend + frontend |
| 方案评审 | arch-design.md + ui-implementation-plan.md 已完成，handoff 状态确认 | tech-lead |
| 开发完成 | S1-S3 所有任务验收标准满足 | frontend-engineer |
| QA 完成 | ui-review-checklist 填写，Playwright 13/13，4-state 各页面验证 | qa-engineer |
| 发布准备 | Docker 镜像可启动，deployment-context.md 已写，launch-acceptance.md 已写 | devops-engineer |

---

## 角色分工

| 角色 | 职责 |
|------|------|
| `tech-lead` | 方案收口、handoff 决策、风险仲裁 |
| `frontend-engineer` | S0-2 ~ S3-2 全部 UI 实现任务 |
| `backend-engineer` | S0-1 WriteTimeout 修复；确认 API 契约 |
| `devops-engineer` | S0-6 Dockerfile；S4-1 docker-compose |
| `qa-engineer` | S4-2 UI 评审；S4-3 E2E 回归 |

---

## Handoff 状态

**当前阶段**: plan  
**目标阶段**: execute  
**就绪状态**: handoff-ready

**Readiness proof**:
- 需求挑战会完成（PM/PM/Arch 三方，7 条质疑已收敛）
- arch-design.md 完成（系统边界、组件拆分、接口约定、风险）
- ui-implementation-plan.md 完成（store schema、composable、组件拆分、4-state、router guard）
- 后端前置条件已应用（WriteTimeout 150s，`internal/api/server.go:181`）
- PRD 中所有 Out-of-Scope 项已确认不在 v1 范围

**阻塞项**: 无

**下一跳角色**: frontend-engineer → 从 S0-2 开始

---

## Out-of-Scope（v1 不做）

- SSE 流式输出
- Soul 上传/编辑 UI
- 平台频道管理（Telegram/Discord）
- 用量统计图表
- 主题切换（深/浅模式）
- 移动端优化
- BFF / httpOnly session cookie（安全增强，可作 v2 选项）
