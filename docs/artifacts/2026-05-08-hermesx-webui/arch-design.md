# Arch Design: hermesx-webui

**版本**: v0.1  
**日期**: 2026-05-08  
**Owner**: architect  
**关联 PRD**: docs/artifacts/2026-05-08-hermesx-webui/prd.md  
**关联 ADR**: ADR-003, ADR-004, ADR-005  

---

## 系统边界

```
┌─────────────────────────────────────────────────────────┐
│  Browser                                                │
│  ┌──────────────────┐    ┌──────────────────────────┐  │
│  │  User Portal (/) │    │  Admin Console (/admin/) │  │
│  │  Vue 3 SPA       │    │  Vue 3 SPA               │  │
│  └────────┬─────────┘    └────────────┬─────────────┘  │
│           │ HTTP/SSE                  │ HTTP           │
└───────────┼───────────────────────────┼────────────────┘
            │                           │
   ┌────────▼───────────────────────────▼────────────┐
   │  nginx:alpine (port 30081 in K8s / 30081 local)  │
   │  ┌──────────────────────────────────────────┐    │
   │  │  proxy_pass to hermesx-saas:8080         │    │
   │  │  proxy_buffering off (SSE)               │    │
   │  │  static file serving (dist/)             │    │
   │  └──────────────────────────────────────────┘    │
   └────────────────────────┬────────────────────────┘
                            │
   ┌────────────────────────▼────────────────────────┐
   │  hermesx-saas (port 8080 / NodePort 31923)      │
   │  Go + PostgreSQL + Redis + RustFS               │
   │  API: /v1/* (user), /admin/v1/* (admin)         │
   └─────────────────────────────────────────────────┘
```

**外部依赖：**
- hermesx-saas API（不修改后端逻辑，仅新增 3 个端点）
- 新增后端端点：`GET /admin/v1/bootstrap/status`、`POST /admin/v1/bootstrap`、`GET /admin/v1/tenants/{id}/api-keys`

**集成点：**
- Vite 构建产物（`dist/`）可直接挂载进 hermesx-saas 的 `SAAS_STATIC_DIR`，或独立 nginx 容器部署

---

## 目录结构（最终形态）

```
webui/
├── index.html                  ← User Portal 入口
├── admin.html                  ← Admin Console 入口
├── vite.config.ts              ← multi-page build 配置
├── tailwind.config.ts
├── tsconfig.json               ← strict mode
├── nginx.conf                  ← SSE 代理配置
├── Dockerfile                  ← nginx:alpine
├── src/
│   ├── shared/
│   │   ├── api/
│   │   │   ├── client.ts       ← useApi composable（shared）
│   │   │   └── sse.ts          ← useSse composable（fetch+ReadableStream）
│   │   ├── stores/
│   │   │   └── auth.ts         ← Pinia store（adminApiKey + userApiKey）
│   │   ├── types/
│   │   │   └── index.ts        ← 共享 TypeScript 类型
│   │   └── components/
│   │       ├── layout/         ← AppShell, Sidebar, Header
│   │       └── ui/             ← 基础组件（Button, Modal, StatusBadge）
│   ├── user/
│   │   ├── main.ts             ← User Portal Vue 实例 + TanStack Query
│   │   ├── App.vue
│   │   ├── router.ts           ← /login, /chat, /memories, /skills, /usage
│   │   └── pages/
│   │       ├── LoginPage.vue
│   │       ├── ChatPage.vue    ← SSE streaming
│   │       ├── MemoriesPage.vue
│   │       ├── SkillsPage.vue
│   │       └── UsagePage.vue
│   └── admin/
│       ├── main.ts             ← Admin Console Vue 实例 + TanStack Query
│       ├── App.vue
│       ├── router.ts           ← /login, /bootstrap, /tenants, /keys, /audit, /pricing, /sandbox
│       └── pages/
│           ├── AdminLoginPage.vue
│           ├── BootstrapPage.vue
│           ├── TenantsPage.vue
│           ├── ApiKeysPage.vue
│           ├── AuditLogsPage.vue
│           ├── PricingPage.vue
│           └── SandboxPage.vue
└── public/
```

---

## 关键数据流

### 1. 用户登录与 Auth 流

```
LoginPage (输入 API Key + User ID)
  → POST /v1/me (Authorization: Bearer <api_key>, X-Hermes-User-Id: <uid>)
  ← {tenant_id, user_id, roles, scopes}
  → 存入 sessionStorage (api_key, user_id, tenant_id)
  → 跳转 /chat
```

**关键约束：** `tenant_id` 从响应提取，**前端不发送 `X-Hermes-Tenant-Id` 请求 header**。

### 2. Admin 登录流

```
AdminLoginPage (输入 Admin API Key)
  → POST /v1/me (Authorization: Bearer <admin_key>)
  ← {tenant_id, roles: ["admin"], scopes: ["admin", ...]}
  → 验证 roles 包含 "admin"，否则拒绝
  → 存入 sessionStorage (adminApiKey, tenant_id)
  → 跳转 /admin/tenants
```

**关键约束：** Admin 页面使用 `adminApiKey`，不再使用 `acpToken`（ACP 是编辑器协议）。

### 3. SSE 流式对话

```
ChatPage.vue
  → 新建 AbortController
  → fetch('/v1/chat/completions', {
      method: 'POST',
      headers: {Authorization, X-Hermes-User-Id, X-Hermes-Session-Id},
      body: JSON.stringify({messages, model, stream: true})
    })
  → res.body.getReader()
  → 逐块解码 → 解析 data: {...} 行 → 追加到消息流
  → finish_reason === 'stop' → 结束
  → AbortController.abort() 用于停止生成
```

**不使用 EventSource**（无法设置 Authorization header）。

### 4. Bootstrap 引导流

```
Admin App 启动
  → GET /admin/v1/bootstrap/status
  → {bootstrap_required: true}  → 跳转 /admin/bootstrap
  → {bootstrap_required: false} → 正常加载（检查 sessionStorage 中的 adminApiKey）

BootstrapPage.vue
  → 用户输入 HERMES_ACP_TOKEN（运维提供，不存储）
  → POST /admin/v1/bootstrap (Authorization: Bearer <acp_token>)
  ← {api_key: "hx-...", key_id: "...", name: "initial-admin-key"}
  → 一次性展示明文 key（带复制按钮，确认后无法再查看）
  → 跳转 /admin/login
```

---

## 接口约定

| 端点 | 认证 | 用途 | 状态 |
|------|------|------|------|
| GET /admin/v1/bootstrap/status | 无 | 检查是否需要 bootstrap | 新增 |
| POST /admin/v1/bootstrap | ACP token | 创建首个 admin key（一次性）| 新增 |
| GET /admin/v1/tenants/{id}/api-keys | admin key | 列出租户 API keys（掩码）| 新增 |
| POST /v1/me | user/admin key | 登录验证 + 获取 tenant_id | 已有 |
| POST /v1/chat/completions | user key | SSE 流式对话 | 已有 |
| GET/DELETE /v1/sessions | user key | 会话列表/删除 | 已有 |
| GET/DELETE /v1/memories | user key | 记忆列表/删除 | 已有 |
| GET /v1/skills, /v1/skills/{name} | user key | 技能浏览 | 已有 |
| GET /v1/me | user key | 身份信息 | 已有 |
| GET /v1/usage | user key | 用量统计 | 已有 |
| POST/GET/PUT/DELETE /v1/tenants | admin key | 租户 CRUD | 已有 |
| POST/DELETE /admin/v1/tenants/{id}/api-keys/{kid} | admin key | Key 操作 | 已有（部分）|
| GET /admin/v1/audit-logs | admin key | 审计日志 | 已有 |
| GET/PUT/DELETE /admin/v1/pricing-rules/{model} | admin key | 定价规则 | 已有 |
| GET/POST/DELETE /admin/v1/tenants/{id}/sandbox-policy | admin key | 沙箱策略 | 已有 |

**认证头规范（统一）：**
- User 请求：`Authorization: Bearer <user_api_key>` + `X-Hermes-User-Id: <uid>`
- Admin 请求：`Authorization: Bearer <admin_api_key>`
- Bootstrap：`Authorization: Bearer <HERMES_ACP_TOKEN>`（仅 bootstrap 端点，不持久化）

---

## 技术选型

| 类别 | 选型 | 原因 |
|------|------|------|
| 框架 | Vue 3 + Composition API | 既有代码基础（ADR-003）|
| 状态管理 | Pinia + `@tanstack/vue-query` v5 | Pinia for UI state, TanStack for server state + cache |
| 路由 | Vue Router v4 | 既有配置 |
| UI 组件库 | Naive UI 2.39+ | 企业级 DataTable/Form/Modal，AdminConsole 首选 |
| 样式 | Tailwind CSS v4 | Naive UI 覆盖不到的自定义区域 |
| SSE | fetch + ReadableStream | EventSource 无法设置 Authorization header |
| 构建 | Vite 6 multi-page | ADR-004 |
| 容器 | nginx:alpine | 静态文件服务 + SSE 代理 |

---

## nginx 配置（SSE 关键项）

```nginx
server {
    listen 80;
    root /usr/share/nginx/html;

    # SSE: 禁用缓冲，延长超时
    location /v1/chat/ {
        proxy_pass http://hermesx-backend;
        proxy_buffering off;
        proxy_read_timeout 300s;
        proxy_http_version 1.1;
        proxy_set_header Connection '';
        add_header X-Accel-Buffering no;
    }

    location /admin {
        try_files $uri $uri/ /admin.html;
    }

    location / {
        proxy_pass http://hermesx-backend;  # API 请求
        try_files $uri $uri/ /index.html;   # SPA fallback
    }
}
```

**注意：** nginx 必须将 `/v1/chat/` 流量转发到后端，而非直接 serve 静态文件。

---

## 风险与约束

| 风险 | 影响 | 缓解 |
|------|------|------|
| K8s ingress/proxy 未配置 `proxy_buffering off` | SSE 断流，用户体验降级 | 实现自动重连（useReconnectingSse），降级为轮询 |
| Naive UI 某些组件 TypeScript 类型不完整 | 类型检查误报 | `@ts-expect-error` 局部处理，不影响业务逻辑 |
| Bootstrap 端点并发竞争（两个 admin 同时 bootstrap）| 创建两个 admin key | 后端原子检查（DB 事务 + unique constraint）|
| CORS：前端 dev port 与后端不同源 | Admin API 跨域失败 | `SAAS_ALLOWED_ORIGINS` 加入 `http://localhost:5173` |
| 旧 HTML 下线影响 CI smoke 测试 | 流水线失败 | Phase 3 前排查 `isolation-test.html` 依赖 |
