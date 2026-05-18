# Test Plan: v2.3.0 Security Integration Sprint

> **角色**: qa-engineer  
> **状态**: review  
> **日期**: 2026-05-18  
> **关联任务**: 2026-05-18-v230-security-integration

---

## 评审输入

- `execute-log.md`（全部 Story A/B/C/D/E/F 完成）
- `arch-design.md`（D1/D2/D3 架构决策）
- `delivery-plan.md`（验收标准与 P1/P2/P3 准入条件）
- code-reviewer 评审结论（2026-05-18）
- security-reviewer 评审结论（2026-05-18）
- git diff：24 files changed, 596 insertions(+), 100 deletions(-)

---

## 测试范围

### 功能范围

| Story | 功能 | 覆盖状态 |
|-------|------|----------|
| A (#38) | SafetyInterceptor → RunConversation，CheckInput/CheckOutput，audit 模式 | 需补 F-2 修复后重测 |
| B (#36+#37) | SecureTransport 注入 ToolContext，CheckRedirect per-tool | 需补 F-5 修复后重测 |
| C (#39) | 高风险 10 工具 SecretResolver 替换 os.Getenv | **F-3 阻断：SecretResolver 未注入，当前完全无效** |
| D (#40) | Admin API 11 端点注册，RequireScope("admin") | 通过，路由鉴权正确 |
| E (#41) | Canary TTL 清理 goroutine | 通过，单元测试 6 个 |
| F-#42 | WithAllowedKeys 接口限制 | 部分通过，ResolvedValues bypass 需修复 |
| F-#43 | NFKC normalization | 通过，位置正确 |
| F-#44 | CI forbidigo warn-only | 通过，按计划 warn-only |
| F-#45 | CachedEgressPolicy TTL 60s | 通过，7 个测试覆盖 |

### 非功能范围

- `-race` 回归：26/26 包通过（execute-log 记录）
- P99 性能：safety 扫描 < 50ms（bench_test.go 基线满足）
- 内存：canary 10k token benchmark 待验证（Story E 验收标准）

### 不覆盖项

- WASM sandbox（ADR-006 推迟）
- WebUI 安全配置界面（backlog）
- 非 HTTP 出站协议

---

## 发现的阻塞问题

### BLOCKER-1 — SecretResolver 未注入 ToolContext（F-3）

**严重级别**: HIGH / 阻断  
**位置**: `internal/agent/agent.go:783-791`

`executeSingleTool` 构建 `ToolContext` 时没有注入 `SecretResolver`。`toolCtx.SecretResolver` 为 nil，工具调用时所有 `.Resolve()` 路径均走 `os.Getenv` fallback。**Story C 的 SecretResolver 迁移在运行时完全无效。** `AIAgent` struct 也没有 `secretResolver` 字段，注入链路尚未建立。

**修复要求**:
1. `AIAgent` struct 新增 `secretResolver secrets.SecretResolver` 字段
2. `WithSecretResolver(r SecretResolver) AgentOption` 注入
3. `executeSingleTool` 将 `a.secretResolver` 赋给 `toolCtx.SecretResolver`

---

### BLOCKER-2 — ListTokens 暴露原始 Canary Token 值（F-1）

**严重级别**: HIGH / 阻断  
**位置**: `internal/safety/canary.go:86-98`, `internal/api/admin/secrets.go:100-109`

`GET /admin/v1/secrets/canary-tokens` 将完整 `CANARY-<hex>-CANARY` 字符串返回给所有 admin 用户。Canary 的安全价值依赖其不可预测性；一旦 admin 凭证被攻击者获取，可枚举所有活跃 token，指令 LLM 在输出中避开这些字符串，彻底绕过 Canary 检测。

**修复要求**:
- `TokenInfo` 只暴露 opaque handle（如 token SHA-256 的前 8 hex 字节）
- `RemoveToken` 接受 handle，内部维护 handle→token 映射
- DELETE 端点使用 handle，不接受原始 token 作为路径参数

---

### BLOCKER-3 — restrictedResolver.ResolvedValues 泄漏非 allowed 密钥（Code HIGH-3）

**严重级别**: HIGH / 阻断  
**位置**: `internal/secrets/resolver.go:114-116`

`restrictedResolver.ResolvedValues()` 直接委托给 inner resolver，返回 inner 解析过的**所有**密钥，包括不在 allowed set 内的密钥。WithAllowedKeys 的隔离保证被绕过。

**修复要求**:
```go
func (r *restrictedResolver) ResolvedValues() map[string]string {
    all := r.inner.ResolvedValues()
    out := make(map[string]string, len(r.allowed))
    for k := range r.allowed {
        if v, ok := all[k]; ok { out[k] = v }
    }
    return out
}
```

---

### BLOCKER-4 — 13+ 工具仍使用裸 http.Client，完全绕过 SecureTransport（F-5）

**严重级别**: HIGH / 阻断  
**位置**: `internal/tools/` 多处（web.go 内 fallbackSearch 等，以及 osv_check 等）

`tctx.HTTPClient` 仅被传入显式接受它的工具函数。在工具函数体内自行构造 `http.Client{}` 或 `&http.Client{}` 的处理器完全绕过 SecureTransport，没有 IP 封锁、没有 policy 检查、没有 egress 审计。

**修复要求**: 全面审计 `internal/tools/` 内所有 `http.Client{}` 和 `&http.Client{}` 实例化，替换为 `tctx.HTTPClient`。同时在 `.golangci.yml` 中为 `internal/tools/` 添加 forbidigo 规则禁止 `http.Client{`。

---

### BLOCKER-5 — ModeEnforce 下安全超时降级为 Allow（F-2）

**严重级别**: HIGH / 阻断  
**位置**: `internal/agent/agent.go:322-334`, `424-437`

`CheckInput`/`CheckOutput` 超时统一降级为 allow+log，不区分 tenant 的 policy 模式。ModeEnforce 租户的 safety 检查可通过耗尽 safety 服务响应时间来绕过。

**修复要求**: 在 `inputErr != nil` 分支中检查 tenant policy；若为 `ModeEnforce` 则 fail-closed（block + interrupt），若为 `ModeLogOnly` 则 allow + warn。

---

## 非阻塞风险

### MEDIUM 级别

| ID | 位置 | 描述 | 建议 |
|----|------|------|------|
| M-1 | agent.go:761 | per-call `NewSecureTransport` 每次 executeSingleTool 创建新 `*http.Transport`（含 idle pool），高并发下连接数爆炸 | 将 SecureTransport 提升为 agent 实例共享，per-call 仅 override CheckRedirect |
| M-2 | agent.go:766-780 | `CheckRedirect` limit exceeded 返回 `ErrUseLastResponse` 而非 `ErrNotAllowed`，语义为"静默接受最后一跳" | 限制超出时改返回 `egress.ErrNotAllowed` |
| M-3 | egress/policy.go | `NewAllowAllPolicy()` 无 CI 守卫，v2.4.0 替换风险被遗忘 | 添加 TODO(v2.4.0) 注释 + grep CI 检查 |
| M-4 | tools/ + resolver.go | resolve 失败时 fallback 到 os.Getenv，错误被 `_` 丢弃，无 warn 日志 | 将 `resolveErr` 记录到 `slog.Warn`，fallback 可见 |
| M-5 | egress/cache.go:90-98 | `InvalidateTenant` O(n) 持写锁遍历 map | 低优先级，文档注明 O(n) 行为 |
| M-6 | secrets/resolver.go:80 | `WithAllowedKeys(r, nil)` 不限制，缺省 = 全开；新注册工具默认获得所有密钥访问 | 推荐反转为"默认拒绝"或添加 warn 日志 |
| M-7 | api/admin/safety.go | `POST /admin/v1/safety/scan` 无 `http.MaxBytesReader`，admin 可触发大输入 DoS | 添加 64KB body 限制 |
| M-8 | api/admin/*.go | globalPolicyStore/globalCanaryDetector 无并发初始化守卫，测试存在竞态 | 迁移到 DI struct 字段 或使用 `atomic.Pointer[T]` |

### LOW 级别

| ID | 描述 |
|----|------|
| L-1 | SafetyInterceptor 使用 `safetyCtx, safetyCancel := ...` 后立即调用，若 panic 则 cancel 泄漏；改用 `defer safetyCancel()` |
| L-2 | Canary cleanup goroutine 与 InterceptorChain 内部 Canary 是两个独立实例，v2.4.0 统一接入 runner |
| L-3 | forbidigo warn-only 不阻断 CI；Story C 修复前无法升级为 error |

---

## 测试矩阵

| 场景 | 类型 | 前置条件 | 期望结果 |
|------|------|----------|----------|
| CheckInput 检测 prompt injection | 单元 | ModeLogOnly tenant | 日志记录，对话继续 |
| CheckInput 超时 + ModeEnforce | 单元 | Mock safety 服务超时 | fail-closed，interrupt=true |
| CheckOutput block 流式响应后 | 集成 | ModeLogOnly，响应含 violation | interrupt=true，OnError 回调触发 |
| SecretResolver Resolve 成功，无 fallback | 单元 | tctx.Secrets 已注入，key 在 allowed | 返回 secret 值，不调用 os.Getenv |
| SecretResolver Resolve 失败，fallback 有 warn 日志 | 单元 | tctx.Secrets 返回 ErrKeyNotAllowed | slog.Warn 触发，fallback 值返回 |
| SecureTransport 阻断私有 IP 直连 | 单元 | 目标 host = 192.168.1.1 | dial 失败，ErrNotAllowed |
| CheckRedirect deny-all（maxRedirects=0） | 单元 | 目标返回 301 | ErrUseLastResponse，不跟随 |
| CheckRedirect 超出 maxRedirects | 单元 | maxRedirects=1，两跳 redirect | ErrNotAllowed |
| ListTokens 仅返回 opaque handle | 单元 | 2 个活跃 canary token | 响应中无原始 token 字符串 |
| WithAllowedKeys 阻断非 allowed key | 单元 | allowed=["EXA_API_KEY"]，访问 FIRECRAWL | ErrKeyNotAllowed |
| ResolvedValues 只返回 allowed 内密钥 | 单元 | restricted，allowed=["A"]，inner 含 A+B | 仅返回 A |
| CachedEgressPolicy TTL 命中 | 单元 | 两次相同 tenant+host 查询 | 第二次不调用 inner.IsAllowed |
| Admin API RequireScope | 集成 | 无 admin scope token | 403 |
| Admin API safety scan 大输入 | 集成 | body > 64KB | 413 Request Entity Too Large |
| Canary TTL 清理 | 单元 | 插入 10k token，等 2×TTL | 内存回收，ActiveTokenCount=0 |
| -race 全量回归 | 集成 | `go test ./... -race` | 无新增失败 |

---

## 放行建议

**当前状态：NOT READY — 存在 5 个阻塞问题，不建议合并到 main / 生产部署。**

### 阻塞项（必须修复）

1. BLOCKER-1：注入 SecretResolver 到 ToolContext（Story C 运行时完全无效）
2. BLOCKER-2：ListTokens 改用 opaque handle（Canary 安全语义被破坏）
3. BLOCKER-3：restrictedResolver.ResolvedValues 过滤 allowed set
4. BLOCKER-4：审计 13+ 工具裸 http.Client，替换为 tctx.HTTPClient
5. BLOCKER-5：ModeEnforce 下 safety timeout fail-closed

### 可接受风险（文档存档，不阻断修复后合并）

- AllowAllPolicy 过渡期（F-4）：SSRF 防护仅覆盖 RFC-1918 直连；per-tenant allowlist v2.4.0 替换，已记录
- per-call transport pool（M-1）：开发环境可接受，v2.4.0 前修复
- Canary goroutine 双实例（L-2）：v2.4.0 统一接入 runner

---

## 验证依据

- execute-log.md：`go test ./... -race` 26/26 通过 ✅
- code-reviewer：2026-05-18，3 HIGH + 4 MEDIUM + 3 LOW
- security-reviewer：2026-05-18，4 BLOCKER HIGH + 1 documented HIGH + 3 MEDIUM + 3 LOW
- 新增失败：0（满足 P3 准入条件 ≤ 5）

---

## 节点检查

| 检查点 | 状态 |
|--------|------|
| P1 回归 ≤ 5 新增失败 | ✅ 0 新增失败 |
| `go test ./... -race` 绿 | ✅ 26/26 |
| Admin API RequireScope | ✅ 全部 11 端点已注册 |
| SafetyInterceptor audit 模式接入 | ⚠️ 接入但 ModeEnforce timeout 需修复 |
| SecretResolver 高风险工具迁移 | ❌ 运行时 tctx.Secrets 为 nil，迁移无效 |
| Canary admin API 安全 | ❌ 暴露原始 token 值 |
