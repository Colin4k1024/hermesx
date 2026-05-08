# PRD: HermesX Web UI — SaaS Admin Console + User Agent Portal

**版本**: v0.1  
**日期**: 2026-05-08  
**Owner**: tech-lead  
**状态**: ready-for-plan  
**关联后端**: hermes-agent-go (hermesx) v2.1.0+  
**Slug**: hermesx-webui

---

## 背景

hermes-agent-go 已具备完整的 SaaS 后端能力（多租户、API Key、审计、定价、SSE Chat、Skills、Memory），
当前 UI 以内嵌静态 HTML（`internal/dashboard/static/`）交付，存在以下问题：

- 单文件 HTML，无组件复用，维护成本高
- Admin 功能分散，无完整租户生命周期管理流程
- Chat 界面缺乏多用户、多会话完整体验
- 无独立部署能力，前后端耦合

需要一个**独立前端项目**，对接现有后端 API，提供 SaaS 级别的 Admin Console 和 User Agent Portal。

## 目标与成功标准

| 目标 | 成功标准 |
|------|---------|
| Admin 可完成租户全生命周期操作 | 创建/查看/更新/删除租户 ≤ 3 步完成 |
| Admin 可管理 API Key、审计日志、定价规则、沙箱策略 | 全部 admin API 端点有对应 UI |
| 用户可与 Agent 流式对话 | SSE 流式输出延迟 < 200ms 首 token 显示 |
| 用户可管理多个会话、查看长期记忆、浏览技能 | 功能完整，无缺失态 |
| 独立部署 | `npm run build` 产出静态文件，可挂载进现有 Docker 镜像或独立 Nginx 部署 |
| 零 `any` 类型泄漏 | TypeScript strict mode，CI 拒绝 any |

## 用户故事与验收标准

### Portal A — SaaS Admin Console

**US-A1 租户管理**
- 作为 Admin，我可以创建新租户，指定 name / plan / rate_limit_rpm / max_sessions
- 作为 Admin，我可以查看租户列表，支持分页
- 作为 Admin，我可以更新租户配置
- 作为 Admin，我可以删除租户（需二次确认）
- 验收：对接 `POST/GET/PUT/DELETE /v1/tenants`，操作结果即时反映

**US-A2 API Key 管理**
- 作为 Admin，我可以为指定租户创建 API Key（name、roles、scopes、expiry）
- 作为 Admin，我可以查看租户下所有 API Key（掩码显示）
- 作为 Admin，我可以 Rotate / Revoke 指定 Key
- 验收：对接 `/admin/v1/tenants/{id}/api-keys`

**US-A3 审计日志**
- 作为 Admin，我可以查看全租户审计日志，支持时间范围、租户筛选、分页
- 验收：对接 `GET /admin/v1/audit-logs`，加载 < 2s

**US-A4 定价规则**
- 作为 Admin，我可以查看/新增/修改/删除模型定价规则
- 验收：对接 `/admin/v1/pricing-rules`

**US-A5 沙箱策略**
- 作为 Admin，我可以为指定租户设置/查看/清除沙箱策略
- 验收：对接 `/admin/v1/tenants/{id}/sandbox-policy`

### Portal B — User Agent Portal

**US-B1 多会话 Chat**
- 作为用户，我可以新建对话，与 Agent 流式对话
- 作为用户，我可以在左侧列表切换不同会话
- 作为用户，我可以删除指定会话
- 验收：SSE 流式显示，`stream: true`，token 逐步渲染

**US-B2 长期记忆管理**
- 作为用户，我可以查看 Agent 为我存储的记忆条目
- 作为用户，我可以删除指定记忆
- 验收：对接 `GET/DELETE /v1/memories`

**US-B3 技能浏览**
- 作为用户，我可以查看当前租户可用的 Skills 列表及详情
- 验收：对接 `GET /v1/skills`, `GET /v1/skills/{name}`

**US-B4 身份与用量**
- 作为用户，我可以查看当前登录身份（tenant_id、roles、scopes）
- 作为用户，我可以查看本月 token 用量
- 验收：对接 `GET /v1/me`, `GET /v1/usage`

## 技术范围 (In Scope)

- 独立前端项目：TypeScript + React 18 + Vite + Tailwind CSS
- 路由：React Router v6（两个子应用共用同一仓库，按 path 区分 `/admin/` 和 `/`）
- 状态：Zustand（UI 状态）+ React Query/TanStack Query（服务端数据 + 缓存）
- SSE 流式对话：原生 `EventSource` 或 `fetch + ReadableStream`
- Auth：Bearer Token 存 localStorage，`X-Hermes-Tenant-Id` header 注入
- 构建产物：静态文件，可挂载进 Nginx 或直接由 hermesx 的 `/static/` 路由服务
- 容器化：Dockerfile（nginx:alpine）+ K8s Deployment yaml（NodePort 30081）
- CI：`npm run lint && npm run typecheck && npm run build` 集成到 GitHub Actions

## 非目标 (Out of Scope)

- OIDC/SSO 登录页（后端已有，UI 阶段暂用 API Key 手动输入）
- 多语言 i18n（英文优先）
- 移动端适配（Desktop-first，响应式为 P2）
- 实时协同编辑
- Grafana/Prometheus 指标嵌入（link 即可）
- Skills 上传/编辑 UI（只读浏览，写操作 P2）

## API 契约摘要

后端 Base URL 由环境变量 `VITE_API_BASE_URL` 注入（默认 `http://localhost:31923`）。

| 端点 | Portal | 用途 |
|------|--------|------|
| POST/GET/PUT/DELETE /v1/tenants | Admin | 租户 CRUD |
| POST/GET/DELETE /v1/api-keys | Admin | Key 管理 |
| GET /admin/v1/tenants/{id}/api-keys | Admin | 租户 Key 列表 |
| POST/GET/DELETE /admin/v1/tenants/{id}/api-keys/{kid} | Admin | Key 操作 |
| GET /admin/v1/audit-logs | Admin | 审计日志 |
| GET/PUT/DELETE /admin/v1/pricing-rules/{model} | Admin | 定价规则 |
| GET/POST/DELETE /admin/v1/tenants/{id}/sandbox-policy | Admin | 沙箱策略 |
| POST /v1/chat/completions | User | SSE Chat |
| GET/DELETE /v1/sessions, GET /v1/sessions/{id} | User | 会话管理 |
| GET/DELETE /v1/memories | User | 记忆管理 |
| GET /v1/skills, GET /v1/skills/{name} | User | 技能浏览 |
| GET /v1/me | User | 身份信息 |
| GET /v1/usage | User | 用量统计 |

## UI 范围、终端与质量门禁

- **目标端**: Desktop Web（Chrome/Safari 最新版）
- **产品类型**: 企业管理后台（Admin）+ AI 对话工具（User）
- **设计语言**: 暗色主题（参考现有 chat.html 色彩体系，`#0d1117` bg），简洁专业
- **设计 Token**: 沿用现有 CSS 变量体系（`--bg`, `--surface`, `--accent` 等），通过 Tailwind theme 映射
- **信息密度**: Admin 中等密度（表格+表单），User 低密度（对话为主）
- **可访问性基线**: 颜色对比度 ≥ 4.5:1，键盘可访问，focus ring 可见
- **性能**: 首屏 LCP < 2.5s，bundle < 300KB gzip（不含 React）
- **状态完整性**: loading / empty / error / success 四态均须实现

**前端门禁（进入 execute 前必须满足）**:
- [ ] UI-SPEC 或 design-system-brief 已定义 token、布局、关键组件
- [ ] Admin Console 和 User Portal 关键页面线框确认
- [ ] SSE 流式渲染方案明确

## 参与角色

| 角色 | 职责 |
|------|------|
| tech-lead | 需求收口、架构决策、PRD review |
| architect | 前端项目结构设计、组件边界、API 集成方案 |
| frontend-engineer | 实现 Admin Console 和 User Portal 全部页面 |
| qa-engineer | 功能验证、流式测试、边界态覆盖 |

## 风险与依赖

| 风险 | 影响 | 缓解 |
|------|------|------|
| 后端 CORS 配置需更新 | Admin 跨域请求失败 | `SAAS_ALLOWED_ORIGINS` 加入前端 dev port |
| SSE 在某些 proxy/k8s 配置下断流 | 流式体验降级 | 实现自动 reconnect + 降级 polling |
| Admin API 路由部分仅 POST（无 GET list） | 缺少租户 Key 列表接口 | intake 阶段已识别，需后端补充 `GET /admin/v1/tenants/{id}/api-keys` |
| 前端独立部署与后端静态文件服务冲突 | 两套 UI 并存混乱 | 明确后端保留旧 HTML 作调试用，新 UI 走独立路由/端口 |

## 决策记录（2026-05-08 确认）

| # | 问题 | 决策 |
|---|------|------|
| 1 | 仓库位置 | **monorepo 子项目** — `web/` 目录置于 hermes-agent-go 根目录下 |
| 2 | 用户认证流 | **登录页** — User Portal 和 Admin Console 均有独立登录页，输入 API Key + Tenant ID，存 localStorage |
| 3 | Admin 初始化 | **超管引导页** — 首次部署时若无 admin key，展示 Bootstrap 引导页，通过 `HERMES_ACP_TOKEN` 完成首个 admin key 创建 |
| 4 | 后端缺口 | **后端补充** — `GET /admin/v1/tenants/{id}/api-keys` 需在 execute 阶段前补充到后端 |
| 5 | 旧静态 HTML | **全部下线** — `chat.html` / `admin.html` / `isolation-test.html` / `regression.html` 从镜像中移除，`index.html` 重定向到新 UI |

## 确认后范围更新

### 新增 In Scope
- 登录页：User Portal Login（API Key + Tenant ID 表单）
- Admin Login（API Key 表单）
- Bootstrap 引导页：首次部署检测无 admin key 时展示，通过 ACP Token 创建首个 admin key
- 后端补充：`GET /admin/v1/tenants/{id}/api-keys` 接口

### 新增 Out of Scope（移除）
- 旧静态 HTML 全部下线（`chat.html`, `admin.html`, `isolation-test.html`, `regression.html`）

### 项目目录结构
```
hermes-agent-go/
└── web/                        ← 前端子项目根
    ├── package.json
    ├── vite.config.ts
    ├── tailwind.config.ts
    ├── tsconfig.json
    ├── Dockerfile              ← nginx:alpine 独立容器
    ├── src/
    │   ├── main.tsx
    │   ├── router.tsx          ← React Router v6
    │   ├── api/                ← API client（typed fetch wrapper）
    │   ├── stores/             ← Zustand stores
    │   ├── pages/
    │   │   ├── login/          ← 登录页（user + admin）
    │   │   ├── bootstrap/      ← 超管引导页
    │   │   ├── admin/          ← Admin Console pages
    │   │   │   ├── tenants/
    │   │   │   ├── api-keys/
    │   │   │   ├── audit-logs/
    │   │   │   ├── pricing/
    │   │   │   └── sandbox/
    │   │   └── portal/         ← User Agent Portal pages
    │   │       ├── chat/
    │   │       ├── memories/
    │   │       ├── skills/
    │   │       └── usage/
    │   └── components/
    │       ├── layout/
    │       ├── chat/           ← SSE streaming renderer
    │       └── ui/             ← 基础组件（Button, Table, Modal...）
    └── public/

## 需求挑战会候选分组

| 分组 | 参与角色 | 挑战焦点 |
|------|---------|---------|
| Bootstrap 引导页安全性 | tech-lead + architect | ACP Token 在前端如何安全传递，防止泄漏 |
| SSE 流式渲染方案 | architect + frontend | fetch+ReadableStream vs EventSource，reconnect 策略 |
| 旧 HTML 下线时机 | tech-lead + devops | 新 UI 上线验收后统一下线，需明确切换 Gate |
