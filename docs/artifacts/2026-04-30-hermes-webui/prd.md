# PRD — Hermes Web UI

**状态**: draft  
**日期**: 2026-04-30  
**Owner**: tech-lead  
**Slug**: hermes-webui  
**阶段**: intake → plan

---

## 背景

hermes-agent-go 是一个多租户 SaaS AI Agent 后端，已具备租户隔离、Soul/Skill/Memory 管理、REST API。但目前只有一个单页 `chat.html`（嵌入在二进制中），缺乏可独立部署、面向真实用户的 Web 前端。

参考项目 `github.com/Colin4k1024/hermes-web-ui`（Vue 3 + Naive UI + Koa BFF）提供了技术栈和组件风格参考，但其设计是单用户/单 agent 模式，需要适配 hermes-agent-go 的多租户语义。

---

## 目标与成功标准

| # | 目标 | 成功标准 |
|---|------|---------|
| 1 | 用户可通过 API Key + User ID 接入任意租户 | 连接页正确路由到对应租户，`GET /v1/me` 返回正确身份 |
| 2 | 多会话聊天，历史持久 | 可新建/切换会话，历史从 `/v1/sessions/{id}` 加载 |
| 3 | 记忆面板 | 浏览/删除当前用户的记忆条目（`GET/DELETE /v1/memories`）|
| 4 | 技能面板（只读） | 展示当前租户已安装的技能列表及详情 |
| 5 | 技能管理（Admin） | 上传/删除技能（`PUT/DELETE /v1/skills/{name}`）|
| 6 | 租户 & API Key 管理（Admin） | 创建/列出租户，创建/撤销 API Key（需 ACP Token）|
| 7 | 独立部署 | `webui/` 子目录，`HERMES_BACKEND_URL` env，Nginx 反向代理 Docker 镜像 |
| 8 | 后端零破坏 | Playwright isolation 13/13 继续通过 |

---

## 用户故事

### 普通用户

- 作为租户用户，我可以输入 API Key 和 User ID 连接到我的租户，以便开始聊天
- 作为租户用户，我可以创建新会话并在多个会话之间切换，以便分主题与 Agent 对话
- 作为租户用户，我可以查看 Agent 积累的关于我的记忆，并删除不需要的条目
- 作为租户用户，我可以查看当前租户安装了哪些技能，以便了解 Agent 的能力边界

### 管理员

- 作为管理员，我可以输入 ACP Token 进入管理模式，管理租户和 API Key
- 作为管理员，我可以为租户上传或删除技能文件（Markdown 格式）
- 作为管理员，我可以创建新租户并为其生成 API Key

---

## 功能范围

### In Scope（v1）

| 模块 | 功能 |
|------|------|
| 连接页 | API Key + User ID 输入，连接验证（`GET /v1/me`），每次手动输入（不持久化）|
| 聊天页 | 多会话侧栏、发送消息、显示 AI 回复（非流式 JSON 轮询）、loading/error 状态 |
| 会话管理 | 新建会话（生成 session ID）、会话列表、历史消息加载 |
| 记忆面板 | 列出当前用户记忆、删除单条记忆 |
| 技能面板 | 只读展示技能列表 + 技能内容 |
| 技能管理 | 上传 Markdown 文件作为技能、删除技能（Admin 模式）|
| 租户管理 | 列出租户、创建租户（Admin 模式，需 ACP Token）|
| API Key 管理 | 列出 API Key、创建 API Key、撤销（Admin 模式）|
| 部署 | `webui/` 子目录，`Dockerfile`，`docker-compose.webui.yml`，Nginx 反代配置 |

### Out of Scope（v1）

- SSE 流式输出（backlog）
- Soul 上传/编辑 UI
- 平台频道管理（Telegram/Discord 等）
- 用量统计图表
- Web Terminal
- 主题切换（深/浅模式）
- 移动端优化

---

## 技术约束

| 约束 | 决策 |
|------|------|
| 仓库结构 | monorepo：`webui/` 子目录在 hermes-agent-go |
| 前端技术栈 | Vue 3 + TypeScript + Vite + Naive UI + Pinia + Vue Router |
| 后端层 | 纯静态 SPA（无 Koa BFF），Nginx 反代到 `HERMES_BACKEND_URL`|
| CORS | hermes-agent-go 已有 `corsMiddleware`，配置 `CORS_ORIGINS` 允许 WebUI 域名 |
| Auth | `Authorization: Bearer hk_...`，ACP Token 用于 admin 路由 |
| 非流式 | `POST /v1/chat/completions` 返回整块 JSON，前端 loading spinner 替代流式 |
| 登录持久化 | 不持久化（每次手动输入 API Key + User ID）|
| Node 版本 | ≥ 20（LTS），与参考项目对齐但不强依赖 Node 23 |

---

## 架构概览

```
浏览器
  └── Vue 3 SPA (Vite build → /webui/dist)
        └── Nginx
              ├── / → dist/index.html (SPA)
              └── /v1, /health → HERMES_BACKEND_URL (proxy_pass)

部署方式:
  docker run -e HERMES_BACKEND_URL=http://hermes-saas:8080 hermes-webui
```

---

## 页面结构

```
/connect        连接页（API Key + User ID 输入）
/chat           聊天页（默认落点，含左侧会话栏 + 右侧消息区）
/memories       记忆面板
/skills         技能面板（只读）
/admin/skills   技能管理（Admin）
/admin/tenants  租户管理（Admin）
/admin/keys     API Key 管理（Admin）
```

---

## UI 质量门禁

| 维度 | 要求 |
|------|------|
| 目标端 | 桌面浏览器 ≥ 1280px，响应式适配平板 |
| 状态完整性 | loading / empty / error / success 四态全部实现 |
| 可访问性 | 键盘可导航，表单有显式 label，不以颜色为唯一信号 |
| 前端门禁 | 进入 QA 前须完成 ui-review-checklist |

---

## 风险与依赖

| 风险 | 影响 | 缓解 |
|------|------|------|
| 后端无流式 | 用户体验偏慢 | loading spinner + 明确提示"生成中"；SSE 升级进 backlog |
| CORS 配置 | 跨域请求被拒 | Nginx proxy_pass 完全规避 CORS，开发环境用 Vite proxy |
| API Key 无持久化 | 用户每次手动输入 | 这是用户决策，减少共享设备上的凭据泄露风险 |
| Admin 权限无分层 | 任何持有 ACP Token 的用户可操作所有租户 | Admin 页明显标注风险，后续可增加 RBAC |

---

## 待确认项（已收敛）

| # | 问题 | 决策 |
|---|------|------|
| Q1 | API Key/UserID 是否持久化 | 否，每次手动输入 |
| Q2 | 技能管理是否纳入 v1 | 是（CRUD） |
| Q3 | 租户/API Key 管理是否纳入 v1 | 是（Admin 模式）|
| Q4 | 仓库结构 | monorepo（`webui/` 子目录）|
| Q5 | 是否先升级 SSE | 否，非流式先做 |
