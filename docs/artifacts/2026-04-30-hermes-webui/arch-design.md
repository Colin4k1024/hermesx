# Arch Design — Hermes Web UI

**状态**: draft  
**日期**: 2026-04-30  
**Owner**: architect  
**Slug**: hermes-webui

---

## 1. 系统边界

```
┌─────────────────────────────────────────────────────────────┐
│ Browser (Vue 3 SPA)                                         │
│  sessionStorage: { apiKey: "hk_...", userId: "alice" }      │
│  每次请求 → Authorization: Bearer hk_...                     │
│          → X-Hermes-User-Id: alice                          │
└──────────────┬──────────────────────────────────────────────┘
               │ HTTPS
               ▼
┌──────────────────────────────────────────┐
│ Nginx  (仅生产环境)                        │
│  /           → 提供 SPA 静态文件          │
│  /v1/*       → proxy_pass Go backend     │
│  /health/*   → proxy_pass Go backend     │
│  proxy_read_timeout  180s                │
│  proxy_send_timeout  180s                │
└──────────────┬───────────────────────────┘
               │ HTTP (内部)
               ▼
┌──────────────────────────────────────────┐
│ hermes-agent-go  (Go API 服务)            │
│  :8080                                   │
│  corsMiddleware → Auth → Tenant → RBAC   │
│  WriteTimeout: 150s  ← 需修复 (当前 60s) │
│  LLM httpClient.Timeout: 120s            │
└──────────────┬───────────────────────────┘
               │
       ┌───────┴───────┐
       ▼               ▼
  PostgreSQL        MinIO (skills/soul)
  Redis             LLM API (upstream)
```

**开发模式**: Vite dev server `:5173` 代理 `/v1` 到 Go backend `:8080`。无 Nginx。CORS 由 Go `corsMiddleware` + `CORS_ORIGINS=http://localhost:5173` 处理。

**生产模式**: 单 Docker 镜像。Nginx `:80` 提供 SPA 静态文件并反代 API 路由。

---

## 2. 组件拆分

### 2.1 目录结构

```
webui/
├── src/
│   ├── api/                    # HTTP client 层
│   │   ├── client.ts           # fetch 封装，注入 Auth/User-Id 头
│   │   └── ...                 # 按域拆分：chat, sessions, memories, skills, admin, me
│   ├── stores/                 # Pinia stores
│   │   ├── auth.ts             # apiKey, userId, connected, isAdmin
│   │   ├── chat.ts             # sessions, messages, loading/error
│   │   ├── memory.ts           # memories list
│   │   └── skill.ts            # skills list + content
│   ├── composables/
│   │   └── useApi.ts           # 统一 fetch wrapper，header 注入入口
│   ├── pages/                  # 路由级页面组件
│   │   ├── ConnectPage.vue     # /connect
│   │   ├── ChatPage.vue        # /chat
│   │   ├── MemoriesPage.vue    # /memories
│   │   ├── SkillsPage.vue      # /skills
│   │   └── admin/              # /admin/*
│   ├── components/             # 展示组件（按页面/特性分组）
│   ├── router/index.ts         # 路由 + Auth Guard
│   └── utils/errors.ts         # normalizeApiError
├── vite.config.ts
├── nginx.conf
└── Dockerfile                  # 多阶段：build + Nginx
```

### 2.2 职责边界

| 层 | 模块 | 职责 |
|----|------|------|
| **API** | `composables/useApi.ts` | 单一 fetch 入口。注入 `Authorization`、`X-Hermes-User-Id`。检查 `Content-Type` 防 502 HTML 崩溃。401 → 自动 disconnect + 跳转 `/connect`。 |
| **Store** | `auth.ts` | sessionStorage 读写。`connect()` 调 `GET /v1/me` 验证凭据。`isAdmin = acpToken.length > 0`。 |
| **Store** | `chat.ts` | 会话列表、当前消息列表、optimistic 用户消息追加、4-state loading/error。 |
| **Store** | `memory.ts` / `skill.ts` | 各自领域数据 + 操作。per-key delete loading (Set) 避免锁全表。 |
| **Router** | `beforeEach` guard | 未连接 → `/connect`；非管理员访问 `/admin/*` → `/chat`。 |
| **View** | `ConnectPage` | 唯一入口。API Key + User ID + 可选 ACP Token。成功后跳 `/chat`。 |
| **View** | `ChatPage` | 左栏会话列表 + 右侧消息区。session ID 通过 `X-Hermes-Session-Id` 传递。 |

---

## 3. 关键数据流

### 3.1 连接流程

```
ConnectPage → authStore.connect(apiKey, userId) → GET /v1/me
  ├─ 200 OK  → sessionStorage.set, connected=true, router.push('/chat')
  └─ 401     → 清 sessionStorage, 显示错误, 留在 /connect
```

### 3.2 聊天流程

```
ChatInput.send()
  → chatStore.sendMessage(content)
    1. 追加 optimistic user 消息, isLoading=true
    2. POST /v1/chat/completions
       Headers: Authorization, X-Hermes-User-Id, X-Hermes-Session-Id
    3. 检查 Content-Type:
       ├─ application/json → parse → 追加 assistant 消息
       └─ text/html (502)  → 显示 "请求超时，请重试"
    4. isLoading=false
```

### 3.3 历史会话加载

```
SessionSidebar → chatStore.fetchSessions() → GET /v1/sessions
点击会话 → chatStore.selectSession(id) → GET /v1/sessions/{id}
→ 只读 MessageList 渲染历史消息
```

### 3.4 管理员流程

```
AdminPage (requiresAdmin: true 路由守卫)
  → useApi({ asAdmin: true })  // 使用 acpToken 替换 apiKey
    → GET/POST /v1/tenants
    → GET/POST/DELETE /v1/api-keys
```

---

## 4. 接口约定

### 4.1 必需请求头

| Header | 值 | 来源 | 适用范围 |
|--------|-----|------|----------|
| `Authorization` | `Bearer hk_...` | sessionStorage `apiKey` | 所有 `/v1/*` |
| `X-Hermes-User-Id` | 用户输入的 userId | sessionStorage `userId` | 所有 `/v1/*` |
| `X-Hermes-Session-Id` | 当前会话 ID | chatStore `sessionId` | 仅聊天 |
| `Authorization` | `Bearer {acpToken}` | sessionStorage `acpToken` | admin 路由（覆盖上面） |

### 4.2 聊天请求/响应

```typescript
// POST /v1/chat/completions
Request: { model: string, messages: ChatMessage[], stream?: false }
Response: { id: string, choices: [{ message: { role, content } }] }

// 502 防护: 检查 Content-Type 再调用 .json()
if (!response.headers.get('content-type')?.includes('application/json')) {
  throw new Error('Request timed out — please retry')
}
```

### 4.3 关键 API 端点汇总

| 端点 | 方法 | 用途 | Auth |
|------|------|------|------|
| `/v1/me` | GET | 身份验证 + 获取租户 ID | apiKey |
| `/v1/chat/completions` | POST | 发送消息 | apiKey |
| `/v1/sessions` | GET | 会话列表 | apiKey |
| `/v1/sessions/{id}` | GET | 历史消息 | apiKey |
| `/v1/memories` | GET | 记忆列表 | apiKey |
| `/v1/memories/{key}` | DELETE | 删除记忆 | apiKey |
| `/v1/skills` | GET | 技能列表 | apiKey |
| `/v1/skills/{name}` | GET/PUT/DELETE | 技能 CRUD | apiKey / acpToken |
| `/v1/tenants` | GET/POST | 租户管理 | acpToken |
| `/v1/api-keys` | GET/POST/DELETE | API Key 管理 | acpToken |

---

## 5. 技术选型

| 选型 | 理由 |
|------|------|
| **Vue 3 + Composition API** | 轻量，TypeScript 支持好，与 Pinia/Vue Router 原生集成 |
| **Naive UI** | 完整组件库（表格/表单/模态框/提示），MIT 协议，Vue 3 原生，树摇 |
| **Pinia** | Vue 3 官方状态管理，TypeScript-first，devtools 支持 |
| **sessionStorage** | 关闭 Tab 自动清除，比 localStorage 更安全；每次手动输入可接受 |
| **Hash-mode routing** | 避免 Nginx `try_files` 回退配置复杂性 |
| **Nginx** (生产) | 静态文件高效服务，超时可控（180s 覆盖 LLM 延迟窗口） |

---

## 6. 风险与约束

### R1: API Key 浏览器暴露（已接受）

**风险**: `hk_...` 存在 sessionStorage，DevTools 可见。  
**缓解**: sessionStorage 关 Tab 清除；API Key 可轮换；内部工具场景；文档说明勿在不可信设备使用。  
**状态**: 需求挑战会决策，已接受。

### R2: 超时竞争 — Go WriteTimeout vs LLM 延迟（需修复）

**风险**: Go `WriteTimeout=60s`，LLM client `Timeout=120s`。LLM 响应 61-120s 时，Nginx 返回 HTML 502，SPA `JSON.parse` 崩溃。  
**后端修复（前置条件）**: 将 Go `WriteTimeout` 从 60s 改为 150s。  
**前端防护**: useApi 在 parse 前检查 `Content-Type`，HTML 响应显示友好错误。  
**Nginx**: `proxy_read_timeout 180s`。

### R3: X-Hermes-User-Id 自声明（已知限制）

**风险**: 用户可任意设置 header，在同一租户内伪造其他 userId。  
**缓解**: 跨租户隔离由 API Key 强制（后端不可绕过）。租户内用户分离是尽力而为。  
**状态**: 已接受。

### R4: 无流式输出

**风险**: 响应最长阻塞 120s，无渐进反馈。  
**缓解**: loading spinner + 禁用输入；SSE 升级进 backlog。

---

## 7. 部署拓扑

### 开发

```
go run ./cmd/server              # :8080, CORS_ORIGINS=http://localhost:5173
cd webui && npm run dev          # :5173, Vite proxy /v1 → :8080
```

### 生产 Docker

```dockerfile
# Stage 1: Build SPA
FROM node:20-alpine AS build
WORKDIR /app
COPY webui/package*.json ./
RUN npm ci
COPY webui/ ./
RUN npm run build

# Stage 2: Nginx
FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY webui/nginx.conf /etc/nginx/conf.d/default.conf
ENV HERMES_BACKEND_URL=localhost:8080
EXPOSE 80
```

**nginx.conf 关键片段**:
```nginx
location / {
    try_files $uri $uri/ /index.html;   # SPA fallback
}
location /v1/ {
    proxy_pass http://${HERMES_BACKEND_URL};
    proxy_read_timeout 180s;
    proxy_send_timeout 180s;
}
```

### Kubernetes

Nginx 容器 + ConfigMap 注入 `nginx.conf`，`HERMES_BACKEND_URL` 指向 Go 服务 ClusterIP（如 `hermes-api:8080`）。

---

## 前置条件（后端变更）

在 WebUI 发布前，必须先应用以下后端修复：

**`internal/api/server.go`**:
```go
// 修改前 (WriteTimeout: 60s → LLM 120s 超时竞争)
WriteTimeout: 60 * time.Second,

// 修改后
WriteTimeout: 150 * time.Second,
```
