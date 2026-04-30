# Hermes Agent 多租户平台架构图

> 本文档基于 `saas-api` 模式（v2026-04-30 最新代码）生成，说明多租户多用户如何通过 Hermes Agent 平台进行 AI 对话。

---

## 一、系统全局架构

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                                  Hermes Agent SaaS Platform                                │
│                                     (kind cluster / k8s)                                  │
│                                                                                          │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐   │
│  │                            Kubernetes Namespace: hermes                              │   │
│  │                                                                                  │   │
│  │  ┌─────────────────────────────────────────────────────────────────────────┐    │   │
│  │  │                      hermes-agent Deployment (1 replica)                  │    │   │
│  │  │                                                                          │    │   │
│  │  │  ┌──────────────────────┐     ┌───────────────────────────────────────┐ │    │   │
│  │  │  │    SaaS API Server   │     │     Gateway Runner + API Adapter     │ │    │   │
│  │  │  │        (:8080)       │     │             (:8081)                  │ │    │   │
│  │  │  │                       │     │                                       │ │    │   │
│  │  │  │  ┌─────────────────┐ │     │  ┌─────────────────────────────────┐ │ │    │   │
│  │  │  │  │ Middleware Stack│ │     │  │   Per-Tenant Agent Factory      │ │ │    │   │
│  │  │  │  │ ① Tracing       │ │     │  │                                 │ │ │    │   │
│  │  │  │  │ ② Metrics       │ │     │  │  ┌─────────────────────────┐   │ │ │    │   │
│  │  │  │  │ ③ RequestID     │ │     │  │  │ Tenant A Agent Pool    │   │ │ │    │   │
│  │  │  │  │ ④ Auth          │ │     │  │  │   ↕ session cache      │   │ │ │    │   │
│  │  │  │  │ ⑤ Tenant        │ │     │  │  └─────────────────────────┘   │ │ │    │   │
│  │  │  │  │ ⑥ Logging       │ │     │  │  ┌─────────────────────────┐   │ │ │    │   │
│  │  │  │  │ ⑦ Audit         │ │     │  │  │ Tenant B Agent Pool    │   │ │ │    │   │
│  │  │  │  │ ⑧ RBAC          │ │     │  │  │   ↕ session cache      │   │ │ │    │   │
│  │  │  │  │ ⑨ RateLimit     │ │     │  │  └─────────────────────────┘   │ │ │    │   │
│  │  │  │  └─────────────────┘ │     │  │  ┌─────────────────────────┐   │ │ │    │   │
│  │  │  │                       │     │  │  │ Tenant C Agent Pool    │   │ │ │    │   │
│  │  │  │  ┌─────────────────┐ │     │  │  │   ↕ session cache      │   │ │ │    │   │
│  │  │  │  │   Chat Handler  │ │     │  │  └─────────────────────────┘   │ │ │    │   │
│  │  │  │  │  ↕ sessions     │ │     │  │                                 │ │    │   │
│  │  │  │  │  ↕ memories     │ │     │  │  ┌─────────────────────────┐   │ │ │    │   │
│  │  │  │  │  ↕ souls        │ │     │  │  │ Tenant N Agent Pool    │   │ │ │    │   │
│  │  │  │  └─────────────────┘ │     │  │  └─────────────────────────┘   │ │ │    │   │
│  │  │  │                       │     │  └─────────────────────────────────┘ │ │    │   │
│  │  │  │  ┌─────────────────┐ │     │                                       │ │    │   │
│  │  │  │  │   Admin Handlers │ │     │  ┌─────────────────────────────────┐ │ │    │   │
│  │  │  │  │  /v1/tenants    │ │     │  │   Delivery Router              │ │ │    │   │
│  │  │  │  │  /v1/api-keys   │ │     │  │   (routes responses back)      │ │ │    │   │
│  │  │  │  │  /v1/audit-logs │ │     │  └─────────────────────────────────┘ │ │    │   │
│  │  │  │  └─────────────────┘ │     │                                       │ │    │   │
│  │  │  └──────────────────────┘     └───────────────────────────────────────┘ │    │   │
│  │  │                                                                          │    │   │
│  │  │                    Hermes Service Account                                 │    │   │
│  │  └─────────────────────────────────────────────────────────────────────────┘    │   │
│  │                                                                                  │   │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────────┐   │   │
│  │  │  PostgreSQL      │  │     Redis       │  │         MinIO                │   │   │
│  │  │  StatefulSet     │  │  (via Helm)     │  │  StatefulSet                 │   │   │
│  │  │                  │  │                  │  │                              │   │   │
│  │  │  hermes DB       │  │  Session cache  │  │  ┌─────────────────────────┐ │   │   │
│  │  │  tenant_id FK    │  │  Rate limit     │  │  │ hermes-skills bucket   │ │   │   │
│  │  │  on EVERY table  │  │                 │  │  │                        │ │   │   │
│  │  └─────────────────┘  └─────────────────┘  │  │ {tenantID}/SOUL.md      │ │   │   │
│  │                                           │  │ {tenantID}/skill1/SKILL │ │   │   │
│  │                                           │  │ {tenantID}/skill2/SKILL │ │   │   │
│  │                                           │  └─────────────────────────┘ │   │   │
│  │                                           └─────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                          │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐   │
│  │                              External Services                                    │   │
│  │                                                                                  │   │
│  │  ┌──────────────────────┐                                                        │   │
│  │  │   LLM API Server     │  ◄── OpenAI-compatible API (Qwen3, GPT-4, etc.)     │   │
│  │  │  http://10.191.110  │     HERMES_BASE_URL=http://.../v1                     │   │
│  │  └──────────────────────┘                                                        │   │
│  └─────────────────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 二、多租户认证与隔离架构

```
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│                                 认证与租户隔离分层                                          │
└──────────────────────────────────────────────────────────────────────────────────────────────┘

  ┌──────────────────────────────────────────────────────────────────────────────────────┐
  │                           客户端 / 第三方应用                                          │
  │                                                                                      │
  │   方式 1: Bearer Token (静态Token / JWT)                                             │
  │   Authorization: Bearer <token>                                                       │
  │                                                                                      │
  │   方式 2: API Key (推荐生产环境)                                                      │
  │   Authorization: Bearer <api-key>                                                    │
  │   (API Key 存储于 PostgreSQL, hash 存储, tenant_id 绑定在 DB 记录中)                   │
  │                                                                                      │
  │   方式 3: 平台适配器 (Telegram / Discord / Slack / 飞书 等)                           │
  │   X-Hermes-Tenant-Id: <tenant-uuid>                                                 │
  │                                                                                      │
  └──────────────────────────────────────────────────────────────────────────────────────┘
                                          │
                                          ▼
  ┌──────────────────────────────────────────────────────────────────────────────────────┐
  │                           SaaS API Server (:8080)                                     │
  │                                                                                      │
  │   ┌─────────────────────────────────────────────────────────────────────────────┐   │
  │   │                         Middleware Stack (执行顺序)                            │   │
  │   │                                                                              │   │
  │   │  ① TracingMiddleware     → Trace ID 注入请求上下文                              │   │
  │   │       ↓                                                                     │   │
  │   │  ② MetricsMiddleware    → Prometheus metrics 采集                              │   │
  │   │       ↓                                                                     │   │
  │   │  ③ RequestIDMiddleware → X-Request-ID 生成                                    │   │
  │   │       ↓                                                                     │   │
  │   │  ④ AuthMiddleware       → ★ 认证提取 (见下方 Extractor Chain)                   │   │
  │   │       ↓                                                                     │   │
  │   │  ⑤ TenantMiddleware    → ★ 从 AuthContext 提取 tenant_id (不信任 header)        │   │
  │   │       ↓                                                                     │   │
  │   │  ⑥ LoggingMiddleware   → 结构化日志 (包含 tenant_id)                           │   │
  │   │       ↓                                                                     │   │
  │   │  ⑦ AuditMiddleware    → 操作审计日志                                           │   │
  │   │       ↓                                                                     │   │
  │   │  ⑧ RBACMiddleware     → 基于角色的权限检查                                     │   │
  │   │       ↓                                                                     │   │
  │   │  ⑨ RateLimitMiddleware→ 租户级 RPM 限流                                        │   │
  │   │                                                                              │   │
  │   └─────────────────────────────────────────────────────────────────────────────┘   │
  │                                                                                      │
  │   ┌─────────────────────────────────────────────────────────────────────────────┐   │
  │   │                      Auth Extractor Chain (短路模型)                          │   │
  │   │                                                                              │   │
  │   │   ┌──────────────────────────────┐                                          │   │
  │   │   │ 1. StaticTokenExtractor      │  验证静态 Bearer Token                     │   │
  │   │   │    → AuthContext{           │  返回固定 tenant_id                        │   │
  │   │   │      TenantID: "固定UUID",   │  适用于开发/单租户模式                      │   │
  │   │   │      Roles: ["admin"]        │                                          │   │
  │   │   │    }                        │                                          │   │
  │   │   └──────────────────────────────┘                                          │   │
  │   │              ↓ (失败则继续)                                                  │   │
  │   │   ┌──────────────────────────────┐                                          │   │
  │   │   │ 2. JWTExtractor             │  解析 RS256 JWT, 提取 claims              │   │
  │   │   │    → AuthContext{           │  tenant_id 从 JWT claims 获取              │   │
  │   │   │      TenantID: from claims,  │  支持角色和权限声明                        │   │
  │   │   │      Roles: from claims,    │                                          │   │
  │   │   │    }                        │                                          │   │
  │   │   └──────────────────────────────┘                                          │   │
  │   │              ↓ (失败则继续)                                                  │   │
  │   │   ┌──────────────────────────────┐                                          │   │
  │   │   │ 3. APIKeyExtractor           │  SHA-256 校验存储在 PostgreSQL 的 API Key  │   │
  │   │   │    → AuthContext{           │  tenant_id 从 DB 记录获取 ★                │   │
  │   │   │      TenantID: from DB row, │  每个 API Key 绑定唯一 tenant              │   │
  │   │   │      Roles: from DB row,    │  支持 Key 撤销和过期时间                    │   │
  │   │   │    }                        │                                          │   │
  │   │   └──────────────────────────────┘                                          │   │
  │   │              ↓ (全部失败 → 返回 nil → 401 Unauthorized)                       │   │
  │   └─────────────────────────────────────────────────────────────────────────────┘   │
  │                                                                                      │
  │   ┌─────────────────────────────────────────────────────────────────────────────┐   │
  │   │                      TenantMiddleware 核心隔离逻辑                             │   │
  │   │                                                                              │   │
  │   │   func TenantMiddleware(next):                                               │   │
  │   │     tenantID = ""                                                           │   │
  │   │     if AuthContext exists:                                                  │   │
  │   │       tenantID = AuthContext.TenantID  // ★ 来自凭证, 绝不信赖 X-Tenant-ID    │   │
  │   │     else:                                                                  │   │
  │   │       tenantID = "default"                                                 │   │
  │   │     ctx = WithTenant(ctx, tenantID)  // 注入请求上下文                        │   │
  │   │     next.ServeHTTP(ctx)                                                     │   │
  │   │                                                                              │   │
  │   │   ★ 安全设计: 即使攻击者伪造 X-Tenant-ID header, 也无法跨租户访问              │   │
  │   └─────────────────────────────────────────────────────────────────────────────┘   │
  │                                                                                      │
  └──────────────────────────────────────────────────────────────────────────────────────┘
                                          │
                                          ▼
  ┌──────────────────────────────────────────────────────────────────────────────────────┐
  │                              Tenant 隔离验证                                          │
  │                                                                                      │
  │   所有 PostgreSQL 查询强制包含 tenant_id 条件:                                        │
  │                                                                                      │
  │   SELECT * FROM sessions    WHERE tenant_id = $1  -- ★ 强制索引                    │
  │   SELECT * FROM messages   WHERE tenant_id = $1  -- ★ 每条消息隔离                 │
  │   SELECT * FROM memories   WHERE tenant_id = $1  -- ★ 用户记忆隔离                 │
  │   SELECT * FROM api_keys  WHERE tenant_id = $1  -- ★ API Key 隔离                  │
  │   SELECT * FROM audit_logs WHERE tenant_id = $1  -- ★ 审计日志隔离                 │
  │   SELECT * FROM users     WHERE tenant_id = $1  -- ★ 用户映射隔离                   │
  │                                                                                      │
  │   RBAC 角色:                                                                          │
  │   ┌─────────────┬──────────────────────────────────────────────────────────┐        │
  │   │  admin      │  /v1/tenants/**, /v1/api-keys/**, /v1/audit-logs/**      │        │
  │   │  user      │  /v1/sessions/**, /v1/memories/**, /v1/me              │        │
  │   │  operator  │  /v1/usage, /v1/status                                  │        │
  │   └─────────────┴──────────────────────────────────────────────────────────┘        │
  │                                                                                      │
  └──────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 三、对话全链路流程 (多租户用户发起对话)

```
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│                          场景: Tenant A 的 User 通过 API 与 Agent 对话                     │
│                          访问路径: Gateway API (:8081) + OpenAI-compatible 接口             │
└──────────────────────────────────────────────────────────────────────────────────────────────┘

  客户端应用
  (Web App / Mobile / CLI)
         │
         │  POST /v1/chat/completions
         │  Authorization: Bearer <api-key>
         │  X-Hermes-Session-Id: session-001
         │  X-Hermes-Tenant-Id: 00000000-0000-0000-0000-000000000001
         │  Content-Type: application/json
         │  {
         │    "model": "Qwen3-Coder-Next-4bit",
         │    "messages": [
         │      {"role": "system", "content": "You are helpful."},
         │      {"role": "user", "content": "帮我写个排序算法"}
         │    ]
         │  }
         ▼
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                          │
│  ① API Adapter 接收请求 (APIServerAdapter.handleChatCompletions)                         │
│     ┌──────────────────────────────────────────────────────────────────────────────┐    │
│     │  - 验证 Authorization Bearer Token                                              │    │
│     │  - 解析 X-Hermes-Session-Id / X-Hermes-Tenant-Id                                │    │
│     │  - 从请求体提取: system prompt, history, user message                            │    │
│     │  - 构造 MessageEvent 发送给 Gateway Runner                                        │    │
│     └──────────────────────────────────────────────────────────────────────────────┘    │
│                                        │                                                 │
│  ② Gateway Runner.handleMessage()                                                       │
│     ┌──────────────────────────────────────────────────────────────────────────────┐    │
│     │  - 检查 pairing store: IsUserAllowed(api, "api")                              │    │
│     │  - 解析 tenant_id from event.Metadata["tenant_id"]                             │    │
│     │  - GetOrCreateSession: platform=api, chat_id=session-001, tenant_id=A          │    │
│     │  - 处理 /new, /help, /status 等内置命令                                          │    │
│     └──────────────────────────────────────────────────────────────────────────────┘    │
│                                        │                                                 │
│  ③ getOrCreateAgent() — ★ 租户级 Agent 工厂                                             │
│     ┌──────────────────────────────────────────────────────────────────────────────┐    │
│     │                                                                                 │    │
│     │  // Agent 选项构建 (基于 tenant_id)                                               │    │
│     │  opts = [                                                                         │    │
│     │    WithPlatform("api"),                                                           │    │
│     │    WithSessionID("session-001"),                                                 │    │
│     │    WithTenantID("tenant-A-uuid"),            // ★ 租户隔离标识                    │    │
│     │    WithUserID("user-from-api-key"),          // ★ 用户身份                      │    │
│     │    WithQuietMode(true),                                                          │    │
│     │  ]                                                                               │    │
│     │                                                                                 │    │
│     │  // ★ 租户级 Memory Provider (PostgreSQL)                                        │    │
│     │  mp = NewPGMemoryProviderAsToolsProvider(pgPool, tenantID, userID)              │    │
│     │  opts += WithMemoryProvider(mp)                                                  │    │
│     │  //    → 记忆查询: WHERE tenant_id=$1 AND user_id=$2                             │    │
│     │                                                                                 │    │
│     │  // ★ 租户级 Skill Loader (MinIO)                                               │    │
│     │  loader = NewMinIOSkillLoader(minioClient, tenantID)                            │    │
│     │  opts += WithSkillLoader(loader)                                                │    │
│     │  //    → 技能加载: MinIO prefix="{tenantID}/"                                   │    │
│     │                                                                                 │    │
│     │  // ★ 租户级 Soul/Persona (MinIO)                                              │    │
│     │  soulData = minioClient.GetObject("{tenantID}/SOUL.md")                        │    │
│     │  opts += WithSoulContent(soulData)    // 注入系统人格                            │    │
│     │                                                                                 │    │
│     │  // ★ SaaS 模式跳过本地文件系统                                                 │    │
│     │  opts += WithSkipContextFiles(true)                                            │    │
│     │                                                                                 │    │
│     │  agent = New(opts...)                                                           │    │
│     │                                                                                 │    │
│     └──────────────────────────────────────────────────────────────────────────────┘    │
│                                        │                                                 │
│  ④ Agent 执行对话 (AIAgent.Chat / AIAgent.RunConversation)                             │
│     ┌──────────────────────────────────────────────────────────────────────────────┐    │
│     │                                                                                 │    │
│     │  构建系统提示词:                                                                │    │
│     │  ┌────────────────────────────────────────────────────────────────────────┐ │    │
│     │  │  [Soul Content]        ← {tenantID}/SOUL.md                            │ │    │
│     │  │  [Skills Content]      ← {tenantID}/skill1/SKILL.md                   │ │    │
│     │  │                         {tenantID}/skill2/SKILL.md                     │ │    │
│     │  │  [Memory Context]     ← SELECT * FROM memories                        │ │    │
│     │  │                         WHERE tenant_id=? AND user_id=?                │ │    │
│     │  │  [User History]        ← SELECT * FROM messages                        │ │    │
│     │  │                         WHERE tenant_id=? AND session_id=?             │ │    │
│     │  │  [User Message]         ← "帮我写个排序算法"                              │ │    │
│     │  └────────────────────────────────────────────────────────────────────────┘ │    │
│     │                                        │                                         │    │
│     │                                        ▼                                         │    │
│     │  ⑤ LLM API 调用 (OpenAI-compatible)                                            │    │
│     │     POST http://10.191.110.127:8000/v1/chat/completions                        │    │
│     │     Authorization: Bearer <llm-api-key>                                        │    │
│     │     {                                                                           │    │
│     │       "model": "Qwen3-Coder-Next-4bit",                                        │    │
│     │       "messages": [system+skills+memory+history+user]                        │    │
│     │     }                                                                           │    │
│     │                                        │                                         │    │
│     │                                        ▼                                         │    │
│     │     ⑥ LLM 响应                                                                  │    │
│     │     "Here's a Go quicksort implementation..."                                  │    │
│     │                                                                                 │    │
│     └──────────────────────────────────────────────────────────────────────────────┘    │
│                                        │                                                 │
│  ⑦ 响应与持久化                                                                         │
│     ┌──────────────────────────────────────────────────────────────────────────────┐    │
│     │  - DeliveryRouter.DeliverResponse() → 写入 pending channel                     │    │
│     │  - 构造 OpenAI 格式响应:                                                        │    │
│     │    POST Response Body:                                                         │    │
│     │    {                                                                          │    │
│     │      "id": "session-001",                                                      │    │
│     │      "model": "Qwen3-Coder-Next-4bit",                                         │    │
│     │      "choices": [{"message": {"role": "assistant",                              │    │
│     │                    "content": "Here's a Go quicksort..."}}]                    │    │
│     │    }                                                                          │    │
│     │                                                                                │    │
│     │  - PostgreSQL 持久化 (后台):                                                   │    │
│     │    INSERT INTO messages (tenant_id, session_id, role, content, ...)           │    │
│     │    VALUES ("tenant-A-uuid", "session-001", "assistant", "Here's...", ...)     │    │
│     │    UPDATE sessions SET message_count = message_count + 1                       │    │
│     │                                                                                │    │
│     └──────────────────────────────────────────────────────────────────────────────┘    │
│                                        │                                                 │
         ▼
  HTTP 200 OK
  {"id": "session-001", "choices": [...]}
```

---

## 四、MinIO 租户级数据存储布局

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                              MinIO: hermes-skills bucket                                   │
│                              Endpoint: minio:9000                                         │
│                              Root User: hermes-minio                                       │
└─────────────────────────────────────────────────────────────────────────────────────────────┘

  bucket/
  │
  ├── {tenant-A-uuid}/                    ← Tenant A 的命名空间 (完全隔离)
  │   ├── SOUL.md                        ← A 的 AI 人格/指令定义
  │   ├── coding-expert/
  │   │   └── SKILL.md                   ← A 的专属技能
  │   ├── code-reviewer/
  │   │   └── SKILL.md
  │   └── docs-assistant/
  │       └── SKILL.md
  │
  ├── {tenant-B-uuid}/                    ← Tenant B 的命名空间
  │   ├── SOUL.md                        ← B 的专属人格 (与 A 不同!)
  │   ├── pirate-helper/
  │   │   └── SKILL.md                   ← B 的专属技能
  │   └── ...
  │
  └── {tenant-C-uuid}/                    ← Tenant C 的命名空间
      ├── SOUL.md
      └── ...

  ★ 每个 Tenant 只能访问自己 prefix 下的文件
  ★ MinIOSkillLoader 使用 prefix = "{tenantID}/" 进行 ListObjects
  ★ Soul 内容在会话启动时加载, 作为系统提示词的一部分注入 LLM
```

---

## 五、PostgreSQL 数据模型 (租户隔离)

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                                    PostgreSQL Schema                                        │
│                                       hermes DB                                             │
└─────────────────────────────────────────────────────────────────────────────────────────────┘

  tenants ──────────────────────────────┐
  ├── id (PK, UUID)                      │
  ├── name                               │  ← 每个租户独立记录
  ├── plan (free/pro/enterprise)         │
  ├── rate_limit_rpm                     │  ← 租户级限流配置
  ├── max_sessions                       │
  └── created_at, updated_at             │
                                            │
         ┌──────────┬──────────┬──────────┼──────────┬──────────┐
         │          │          │          │          │          │
         ▼          ▼          ▼          ▼          ▼          ▼
   sessions    messages    users    api_keys   memories   audit_logs
   ────────   ────────   ───────   ────────   ────────   ────────
   tenant_id   tenant_id  tenant_id  tenant_id  tenant_id  tenant_id  ★ 每张表都有 FK
   user_id     session_id  ext_id    key_hash   user_id    user_id
   platform    user_id    display_   roles[]    key        action
   chat_id     role       name       revoked_   value      resource
   display_    content    created_   expires_   created_   tenant_id + WHERE
   name        tokens     at                   at         ━━━━━━━━━━━━━━━━━━━━
   created_    created_                              强制按租户隔离查询
     at          at

  ★ 索引: idx_{table}_tenant ON {table}(tenant_id)
  ★ 唯一: UNIQUE(tenant_id, user_id) 等复合唯一键
  ★ 租户删除时: CASCADE 删除所有关联数据 (GDPR 合规)
```

---

## 六、多用户多租户接入模式

```
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│                               接入方式矩阵                                                   │
└──────────────────────────────────────────────────────────────────────────────────────────────┘

  ┌─────────────────┬────────────────────┬────────────────────────────────────────────┐
  │ 接入方式         │ 认证凭证            │ 适用场景                                    │
  ├─────────────────┼────────────────────┼────────────────────────────────────────────┤
  │ OpenAI-compatible│ API Key (Bearer)   │ 开发者/应用直接集成, 最推荐                 │
  │ (:8081/v1/chat) │ 存于 PostgreSQL     │ Client SDK 无需改造                         │
  │                 │ tenant_id 绑定在 DB  │ 多租户隔离由平台保证                        │
  ├─────────────────┼────────────────────┼────────────────────────────────────────────┤
  │ SaaS REST API   │ Static Token (dev)  │ 后台管理界面, 内部工具                       │
  │ (:8080/v1/...)  │ JWT (production)   │ 完整的租户管理, API Key 管理, 审计日志      │
  │                 │                    │ RBAC 权限控制, GDPR 数据导出                 │
  ├─────────────────┼────────────────────┼────────────────────────────────────────────┤
  │ 平台适配器       │ 平台原生认证        │ 接入飞书/Discord/Telegram/Slack 等即时通讯  │
  │ (Telegram 等)   │ + X-Tenant-Id Hdr  │ 用户体验更自然的对话界面                    │
  │                 │ + Pairing 配对码    │ 首次使用需 /pair <code> 配对授权             │
  └─────────────────┴────────────────────┴────────────────────────────────────────────┘

  ┌──────────────────────────────────────────────────────────────────────────────────────┐
  │                              SDK / Client 示例                                         │
  │                                                                                      │
  │  # OpenAI SDK 方式 (只需改 baseURL + apiKey)                                         │
  │  from openai import OpenAI                                                           │
  │  client = OpenAI(                                                                    │
  │      api_key="pk-xxx-tenant-A",        # Tenant A 的 API Key                          │
  │      base_url="http://hermes:8081/v1"                                               │
  │  )                                                                                  │
  │  response = client.chat.completions.create(                                          │
  │      model="Qwen3-Coder-Next-4bit",                                                 │
  │      messages=[{"role": "user", "content": "Hello"}]                               │
  │  )                                                                                  │
  │                                                                                      │
  │  # Python requests方式                                                                │
  │  import requests                                                                     │
  │  resp = requests.post(                                                               │
  │      "http://hermes:8081/v1/chat/completions",                                      │
  │      headers={"Authorization": "Bearer pk-xxx-tenant-A"},                             │
  │      json={...}                                                                      │
  │  )                                                                                  │
  │                                                                                      │
  │  # cURL 方式                                                                         │
  │  curl -X POST http://hermes:8081/v1/chat/completions \                               │
  │    -H "Authorization: Bearer pk-xxx-tenant-A" \                                     │
  │    -H "Content-Type: application/json" \                                           │
  │    -d '{"model":"Qwen3-Coder-Next-4bit","messages":[...]]}'                       │
  │                                                                                      │
  └──────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 七、部署拓扑

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                              kind (desktop) cluster                                         │
│                                                                                           │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐   │
│  │  Node: desktop-control-plane (master)                                              │   │
│  │  ┌──────────────────────────────────────────────────────────────────────────────┐ │   │
│  │  │  hermes-agent Deployment                                                     │ │   │
│  │  │  image: hermes-agent-saas:local                                              │ │   │
│  │  │  args: ["saas-api"]                                                         │ │   │
│  │  │  ports: 8080 (SaaS API), 8081 (Gateway API)                                 │ │   │
│  │  │  health: /health/live, /health/ready                                       │ │   │
│  │  │                                                                              │ │   │
│  │  │  Env:                                                                       │ │   │
│  │  │  DATABASE_URL       → postgres://hermes:xxx@postgres-postgresql:5432/hermes  │ │   │
│  │  │  REDIS_URL         → redis://redis-master:6379                              │ │   │
│  │  │  MINIO_ENDPOINT    → minio:9000                                             │ │   │
│  │  │  HERMES_BASE_URL   → http://10.191.110.127:8000/v1  (LLM)                   │ │   │
│  │  │  HERMES_API_KEY_LLM→ (llm-api-key)                                          │ │   │
│  │  │  HERMES_MODEL      → Qwen3-Coder-Next-4bit                                  │ │   │
│  │  │  SAAS_API_PORT     → 8080                                                   │ │   │
│  │  │  HERMES_API_PORT   → 8081                                                   │ │   │
│  │  │  HERMES_ACP_TOKEN  → (admin token)                                          │ │   │
│  │  └──────────────────────────────────────────────────────────────────────────────┘ │   │
│  └────────────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                           │
│  ┌──────────────────────┐  ┌──────────────────────┐  ┌──────────────────────────────┐   │
│  │ desktop-worker       │  │ desktop-worker2      │  │ desktop-worker3              │   │
│  │ (无 hermes pod)      │  │ (无 hermes pod)      │  │ (无 hermes pod)              │   │
│  └──────────────────────┘  └──────────────────────┘  └──────────────────────────────┘   │
│                                                                                           │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐   │
│  │  Services (all ClusterIP, hermes namespace):                                       │   │
│  │  hermes-agent    → NodePort:30472 → ClusterIP:10.96.211.118:8080                │   │
│  │  postgres-postgresql → ClusterIP:10.96.78.127:5432  (Helm bitnami/postgresql)     │   │
│  │  redis-master    → ClusterIP:10.96.215.57:6379   (Helm bitnami/redis)            │   │
│  │  minio           → ClusterIP:10.96.55.62:9000                                    │   │
│  └────────────────────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────────────────────┘

  外部访问:
  kubectl port-forward -n hermes svc/hermes-agent 8080:8080  → SaaS API  http://localhost:8080
  kubectl port-forward -n hermes <pod> 8081:8081             → Gateway API http://localhost:8081
```

---

## 八、API 端点一览

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                               SaaS API (:8080)                                              │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│  公共 (无需认证)                                                                          │
│    GET  /health/live               → {"status":"alive"}                                   │
│    GET  /health/ready              → {"database":"ok","status":"ready"}                   │
│    GET  /metrics                   → Prometheus metrics                                   │
│                                                                                           │
│  认证 (Bearer Token / API Key / JWT)                                                      │
│    GET  /v1/me                     → 当前用户身份                                           │
│    POST /v1/chat/completions      → OpenAI-compatible 对话 (走 mockchat handler)        │
│    GET  /v1/sessions              → 当前租户下所有会话                                     │
│    GET  /v1/sessions/:id          → 会话消息历史                                          │
│    GET  /v1/memories              → 用户记忆列表                                           │
│    DELETE /v1/memories/:id        → 删除记忆                                              │
│    GET  /v1/usage                 → 用量统计                                               │
│    GET  /v1/openapi               → OpenAPI 3.0 规范 (需认证)                             │
│    GET  /admin.html               → Admin 管理后台 (SPA)                                  │
│                                                                                           │
│  Admin 角色 (需 admin 权限)                                                              │
│    GET  /v1/tenants               → 租户列表                                             │
│    POST /v1/tenants               → 创建租户                                             │
│    GET  /v1/tenants/:id           → 租户详情                                             │
│    PUT  /v1/tenants/:id           → 更新租户                                             │
│    DELETE /v1/tenants/:id         → 删除租户 (CASCADE)                                   │
│    GET  /v1/api-keys              → API Key 列表                                         │
│    POST /v1/api-keys              → 创建 API Key                                         │
│    DELETE /v1/api-keys/:id        → 撤销 API Key                                         │
│    GET  /v1/audit-logs            → 审计日志                                              │
│    GET  /v1/gdpr/export           → GDPR 数据导出                                        │
│    DELETE /v1/gdpr/data          → GDPR 数据删除                                         │
│                                                                                           │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│                            Gateway API (:8081)                                            │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│    POST /v1/chat/completions   → ★ Full Agent 模式 (走 Gateway Runner + AIAgent)      │
│    GET  /v1/health              → {"status":"ok"}                                        │
│    GET  /health/live             → {"status":"alive"}                                    │
│    GET  /health/ready           → {"status":"ready"}                                    │
│                                                                                           │
│  Headers:                                                                                 │
│    Authorization: Bearer <hermes-api-key>                                               │
│    X-Hermes-Session-Id: <session-id>      (可选, 默认自动生成)                            │
│    X-Hermes-Tenant-Id: <tenant-uuid>     (可选, 默认 "default")                         │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 九、关键设计原则总结

| 设计点 | 实现机制 | 安全保证 |
|--------|---------|---------|
| **租户身份来源** | API Key → DB 查 tenant_id; JWT → claims 提取 | TenantID 来自凭证, 不信赖任何 header |
| **数据隔离** | PostgreSQL 每表 `tenant_id` FK + 索引 | 查询自动按租户过滤 |
| **技能隔离** | MinIO 桶内 prefix = `"{tenantID}/"` | ListObjects 限定 prefix |
| **记忆隔离** | `WHERE tenant_id=? AND user_id=?` | 多轮对话上下文不跨租户 |
| **会话隔离** | session key = `(platform, chat_id, tenant_id)` | 同 session_id 不同租户互不影响 |
| **人格隔离** | 每个租户独立的 `SOUL.md` | AI 行为因租户而异 |
| **认证** | Extractor Chain (短路模式) | 支持 Static Token / JWT / API Key |
| **权限** | RBACMiddleware (admin/user/operator) | API Key 的 roles[] 字段控制 |
| **限流** | RateLimitMiddleware (租户级 RPM) | 按 tenants.rate_limit_rpm 动态调整 |
| **审计** | AuditMiddleware + audit_logs 表 | 记录每次操作的 tenant_id, user_id, action |
| **Agent 缓存** | per-session agent 缓存 (保留 prompt cache) | 相同 session 复用 agent 实例 |
