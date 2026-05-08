# ADR-005: hermesx-webui Admin Bootstrap 端点设计

## 决策信息

| 字段 | 值 |
|------|-----|
| 编号 | ADR-005 |
| 标题 | POST /admin/v1/bootstrap — 首次部署超管引导端点 |
| 状态 | Accepted |
| 日期 | 2026-05-08 |
| Owner | architect + backend-engineer |
| 关联需求 | docs/artifacts/2026-05-08-hermesx-webui/prd.md (US-Bootstrap) |

## 背景与约束

- PRD 决策 #3：首次部署时若无 admin key，展示 Bootstrap 引导页，通过 `HERMES_ACP_TOKEN` 完成首个 admin key 创建。
- 挑战会发现：ACP（Agent Communication Protocol）是编辑器集成协议包，**不是** admin 引导 token 机制。
  - `internal/acp/` 包实现了 editor ↔ agent 的 JSON-RPC 协议
  - `HERMES_ACP_TOKEN` 是静态 bearer，由 `StaticTokenExtractor` 在 auth chain 中处理，赋予 `system:acp-admin` identity
  - 因此，Bootstrap 流程可以复用 `HERMES_ACP_TOKEN` 作为一次性授权凭证，但需要独立的 Bootstrap 端点
- 当前无 `GET /admin/v1/tenants/{id}/api-keys` 接口（已在 intake 识别为 backend gap）。

## 设计决策

### Bootstrap 端点规格

#### GET /admin/v1/bootstrap/status
- 无需认证（公开端点）
- 响应：`{"bootstrap_required": true}` — 当 DB 中 admin role 的 API key 数量为 0 时返回 true
- 前端用途：页面加载时检查，决定展示"登录"还是"Bootstrap 引导"

#### POST /admin/v1/bootstrap
- 认证：`Authorization: Bearer <HERMES_ACP_TOKEN>`（静态 token，仅在初始化阶段已知）
- 请求体：`{"name": "initial-admin-key", "expires_at": "2027-01-01T00:00:00Z"}`
- 逻辑：
  1. 验证 ACP token（通过现有 StaticTokenExtractor chain）
  2. **原子检查**：若 admin API key 数量 > 0，返回 `403 Forbidden {"error": "bootstrap already completed"}`
  3. 创建 API key，roles: `["admin"]`，scopes: `["admin", "chat", "read"]`
  4. 返回明文 key（**唯一一次返回明文**）：`{"api_key": "hx-...", "key_id": "...", "name": "..."}`
- 安全门：端点在 bootstrap_required=false 时返回 403，防止重复调用

### GET /admin/v1/tenants/{id}/api-keys

新增接口，列出租户下所有 API keys（掩码展示）：
- 认证：admin API key（`RequireScope("admin")`）
- 响应：`{"api_keys": [{"id": "...", "name": "...", "prefix": "hx-...", "roles": [...], "scopes": [...], "expires_at": "...", "revoked_at": null, "created_at": "..."}]}`
- 实现：查询 `api_keys` 表，按 `tenant_id` 过滤，不返回 `key_hash`

### 前端 Bootstrap 流程

```
GET /admin/v1/bootstrap/status
  → {bootstrap_required: true}  → BootstrapPage.vue
  → {bootstrap_required: false} → AdminLoginPage.vue

BootstrapPage.vue:
  1. 用户输入 HERMES_ACP_TOKEN（运维人员提供）
  2. 输入新 admin key 名称和过期时间
  3. POST /admin/v1/bootstrap → 显示一次性明文 key（复制后确认关闭）
  4. 跳转 AdminLoginPage，用新 key 登录
```

### 安全说明

- HERMES_ACP_TOKEN 不存储在前端，仅用于一次性 POST 请求，不放 sessionStorage
- Bootstrap 端点在正式运行后永久返回 403（因为 admin key 已存在）
- 明文 key 只在 POST /admin/v1/bootstrap 响应中出现一次，后端只存 SHA-256 hash

## 后续动作

| 动作 | Owner | 完成条件 |
|------|-------|---------|
| 实现 GET /admin/v1/bootstrap/status | backend-engineer | Phase 0 |
| 实现 POST /admin/v1/bootstrap | backend-engineer | Phase 0 |
| 实现 GET /admin/v1/tenants/{id}/api-keys | backend-engineer | Phase 0 |
| 前端 BootstrapPage.vue | frontend-engineer | Phase 1 |
| 前端 Bootstrap 状态检查集成到 AdminApp 启动 | frontend-engineer | Phase 1 |
