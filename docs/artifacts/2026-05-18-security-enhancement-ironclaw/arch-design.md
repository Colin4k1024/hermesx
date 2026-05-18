# Architecture Design: HermesX Security Enhancement

> **Slug:** security-enhancement-ironclaw  
> **State:** plan  
> **Date:** 2026-05-18  
> **Owner:** architect  
> **Status:** draft

---

## 系统边界

```
┌──────────────────────────────────────────────────────────────────┐
│                        API Server                                 │
│  Middleware: Tracing→Metrics→RequestID→Auth→Tenant→Logging→       │
│             Audit→RBAC→RateLimit→Handler                          │
└──────────────────────────────┬───────────────────────────────────┘
                               │
┌──────────────────────────────▼───────────────────────────────────┐
│                       Agent Runtime                                │
│                                                                    │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  NEW: Safety Interceptor Layer                              │  │
│  │  ┌──────────────┐  ┌───────────────┐  ┌────────────────┐   │  │
│  │  │ Input Guard  │  │ Output Guard  │  │ Canary Detect  │   │  │
│  │  └──────┬───────┘  └───────┬───────┘  └───────┬────────┘   │  │
│  └─────────┼───────────────────┼───────────────────┼───────────┘  │
│            │                   │                   │               │
│  ┌─────────▼───────────────────▼───────────────────▼───────────┐  │
│  │              LLM Client ←→ Tool Loop                        │  │
│  └──────────────────────────┬──────────────────────────────────┘  │
│                             │                                      │
│  ┌──────────────────────────▼──────────────────────────────────┐  │
│  │  NEW: Secure Tool Dispatch                                  │  │
│  │  ┌──────────────┐  ┌───────────────┐  ┌────────────────┐   │  │
│  │  │ Secret Vault │  │ Egress Policy │  │ Leak Scanner   │   │  │
│  │  └──────────────┘  └───────────────┘  └────────────────┘   │  │
│  └──────────────────────────┬──────────────────────────────────┘  │
│                             │                                      │
└─────────────────────────────┼──────────────────────────────────────┘
                              │
              ┌───────────────▼───────────────┐
              │       Tool Runtime             │
              │  (existing 50 tools)           │
              │  + Secure HTTP Transport       │
              └───────────────────────────────┘
```

---

## 关键设计决定

### D1: Safety Layer 不在 HTTP Middleware Stack 中

**原因：** Prompt injection 发生在 agent loop 内部（用户消息 → LLM → 工具结果 → LLM），不在 HTTP API 边界。HTTP middleware 看到的是 API client 请求，不是 agent 内部的消息流。

**位置：** `internal/agent/` 中新增 `safety_interceptor.go`，wrap LLM call 的上游和下游。

### D2: Credential Isolation 通过 ToolContext 扩展实现

**原因：** 50 个工具的 `ToolHandler` 接口不变，通过向 `ToolContext` 增加 `ResolveSecret(name string) (string, error)` 方法渐进迁移。

**不做：** opaque handle（工具最终需要明文值做 HTTP auth header），改为 just-in-time resolve + 输出泄漏检测双保险。

### D3: Network Allowlist 统一到 url_safety.go

**原因：** 已有 SSRF 防护基础，allowlist 是其逻辑扩展。避免两套独立的网络控制层。

**实现：** 在共享 `http.Transport` 的 `DialContext` hook 中统一检查 SSRF block + tenant allowlist。

### D4: WASM Sandbox 降级为 POC/Future

**原因（挑战会结论）：** 
- 50 个现有工具使用 `net/http`、`os/exec`、database driver，无法编译为 WASI
- 需要完整的 host function binding，等价于重建 tool SDK
- Docker sandbox + seccomp 已覆盖高风险场景
- ROI 不支持本轮实施

**替代：** 加强 Docker sandbox seccomp profile，为未来新工具预留 WASM 接口标准。

---

## 组件拆分

### 1. Safety Interceptor (`internal/safety/`)

```
internal/safety/
├── interceptor.go      // SafetyInterceptor interface + chain
├── input_guard.go      // 用户输入注入检测
├── output_guard.go     // LLM/工具输出合规检查
├── canary.go           // Canary token 检测
├── patterns.go         // 注入模式规则集 (regex + heuristic)
├── policy.go           // Per-tenant 安全策略
├── policy_store.go     // PostgreSQL 存取
└── interceptor_test.go
```

**核心接口：**
```go
type SafetyInterceptor interface {
    CheckInput(ctx context.Context, tenantID string, messages []Message) (*SafetyResult, error)
    CheckOutput(ctx context.Context, tenantID string, output string) (*SafetyResult, error)
}

type SafetyResult struct {
    Allowed  bool
    Reason   string
    Action   SafetyAction // Block, Log, Mask
    Matches  []PatternMatch
}
```

### 2. Secret Vault (`internal/secrets/`)

```
internal/secrets/
├── vault.go            // SecretVault interface
├── env_vault.go        // 从 env var 加载（当前行为兼容）
├── pg_vault.go         // PostgreSQL 加密存储（future）
├── resolver.go         // Just-in-time secret resolution
├── leak_scanner.go     // 输出泄漏检测 (pattern matching)
├── leak_scanner_test.go
└── rotation.go         // Hot-reload support
```

**ToolContext 扩展：**
```go
type ToolContext struct {
    // ... existing fields ...
    SecretResolver SecretResolver // NEW: just-in-time secret access
}

type SecretResolver interface {
    Resolve(ctx context.Context, name string) (string, error)
    RegisterPattern(name string, pattern *regexp.Regexp) // for leak detection
}
```

### 3. Egress Policy (`internal/egress/`)

```
internal/egress/
├── policy.go           // EgressPolicy interface
├── allowlist.go        // Host+path allowlist rules
├── store.go            // PostgreSQL CRUD
├── transport.go        // Custom http.Transport with DialContext hook
├── transport_test.go
└── admin_handler.go    // Admin API endpoints
```

**统一到 url_safety.go 的拦截链：**
```go
// 在 shared Transport 的 DialContext 中：
// 1. SSRF block (existing) — 拒绝 private IP
// 2. Tenant allowlist (new) — 仅允许白名单 host
// 3. Audit log — 记录所有 allow/deny 决定
```

---

## 关键数据流

### Prompt Injection Defense (F3)

```
User Message → [Input Guard] → LLM Call → LLM Response
                   ↓ (if blocked)          ↓
              Audit Log + 拒绝          [Output Guard] → Tool Call
                                             ↓ (if leak detected)
                                        Mask + Audit Log
```

### Credential Isolation (F2)

```
Tool needs secret → ToolContext.SecretResolver.Resolve("OPENAI_KEY")
                         ↓
                    Vault lookup (env/pg)
                         ↓
                    Return plaintext (just-in-time)
                         ↓
Tool executes HTTP request
                         ↓
Tool returns result → [Leak Scanner] → check against registered patterns
                                            ↓ (if match)
                                       Mask + Audit + Alert
```

### Network Allowlist (F4)

```
Tool HTTP request → shared Transport.DialContext
                         ↓
                    Resolve DNS → IP
                         ↓
                    Check 1: isBlockedIP (SSRF) → block
                         ↓
                    Check 2: tenantAllowlist.IsAllowed(host, path) → block/allow
                         ↓
                    Establish connection
                         ↓
                    Audit log (allow/deny)
```

---

## 接口约定

### Admin API 扩展

| Method | Path | 说明 |
|--------|------|------|
| GET | `/admin/safety/policies` | 列出安全策略 |
| PUT | `/admin/safety/policies/{tenant_id}` | 更新租户安全策略 |
| GET | `/admin/egress/rules` | 列出出口规则 |
| POST | `/admin/egress/rules` | 创建出口规则 |
| DELETE | `/admin/egress/rules/{id}` | 删除出口规则 |
| GET | `/admin/secrets/patterns` | 列出泄漏检测模式 |
| POST | `/admin/secrets/patterns` | 注册新的泄漏模式 |

### 新增 PostgreSQL Migration

```sql
-- safety_policies: per-tenant 安全策略
CREATE TABLE safety_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    mode TEXT NOT NULL DEFAULT 'log_only', -- 'enforce', 'log_only', 'disabled'
    input_patterns JSONB NOT NULL DEFAULT '[]',
    output_rules JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id)
);

-- egress_rules: per-tenant 网络白名单
CREATE TABLE egress_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    host_pattern TEXT NOT NULL,         -- e.g. "api.openai.com", "*.internal.corp"
    path_prefix TEXT DEFAULT '/',       -- e.g. "/v1/"
    action TEXT NOT NULL DEFAULT 'allow', -- 'allow', 'deny'
    priority INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_egress_rules_tenant ON egress_rules(tenant_id);

-- secret_patterns: 泄漏检测模式注册
CREATE TABLE secret_patterns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    pattern TEXT NOT NULL,              -- regex pattern
    severity TEXT NOT NULL DEFAULT 'high',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, name)
);
```

---

## 技术选型

| 决策 | 选择 | 原因 |
|------|------|------|
| 注入检测 | Regex + heuristic（规则引擎） | Go 原生，P99 < 5ms，可热更新 |
| 泄漏检测 | Aho-Corasick 多模式匹配 | 50+ patterns 并行扫描，O(n) 复杂度 |
| Secret 存储 | env var（v1）→ PostgreSQL AES-256-GCM（v2） | 渐进式，不破坏现有部署 |
| 网络策略 | DialContext hook + glob match | 连接级拦截，解决 DNS rebinding TOCTOU |
| WASM | 延后（wazero POC only） | 挑战会结论：ROI 不支持本轮 |

---

## 风险与约束

| 风险 | 影响 | 缓解 |
|------|------|------|
| Injection 误报 | 正常用户被拦截 | 先 log_only 模式 2 周 |
| DialContext hook 性能 | 每个 TCP 连接增加查询 | Redis 缓存 allowlist |
| Secret 迁移不完整 | 部分工具仍读 env | Go linter 禁止 `os.Getenv` in tools/ |
| DNS rebinding | allowlist 绕过 | DialContext 在连接级检查 |
