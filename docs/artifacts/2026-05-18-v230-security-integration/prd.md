# PRD: v2.3.0 Security Integration Sprint

> **Slug:** v230-security-integration  
> **State:** intake  
> **Date:** 2026-05-18  
> **Owner:** tech-lead  
> **Status:** draft

---

## 背景

IronClaw 安全增强（2026-05-18 合并，19c986a → e6dd299）为 HermesX 引入了三个独立安全子系统：

- `internal/safety` — SafetyInterceptor：LLM 输入/输出拦截，含 prompt injection 防护、canary token
- `internal/egress` — SecureTransport：出站 HTTP 管控，IP allowlist + DNS rebinding 防护  
- `internal/secrets` — LeakScanner + SecretResolver：凭证泄漏检测（Aho-Corasick）+ 工具层凭证隔离

**关键问题：三个包均已构建并通过测试，但均未接入主链路**：

| 子系统 | 当前状态 | 缺失集成 |
|--------|----------|----------|
| SafetyInterceptor | 独立包，有测试 | 未挂入 agent.go 对话循环 |
| SecureTransport | 独立包，有测试 | 未替换 50 个工具的 http.Client |
| SecretResolver | 独立包，有测试 | 未在高风险工具中替换 os.Getenv |
| Admin API | 部分存在 | safety/egress/secrets 三个 handler 未注册到 server |

**触发原因：** IronClaw PR 合并后安全能力形同虚设，需要一个专项 sprint 将安全层真正接入生产路径。

**当前约束：**
- HermesX v2.2.x，Go 1.25，单二进制 + Helm 部署
- 现有测试 1585+，不能引入回归
- 工具数量 50+，需分批迁移策略
- 已有 Docker sandbox 隔离（代码执行级），本次补齐工具调用级

---

## 目标与成功标准

### 业务目标

将 IronClaw 安全能力从"已构建"推进到"生产可用"，完成三条核心防线的实际接入：prompt injection 拦截、出站 HTTP 管控、凭证泄漏防护。

### 用户价值

- 企业管理员：可通过 Admin API 配置 safety policy / egress allowlist / secret patterns
- 安全团队：agent 对话循环中自动检测注入，工具出站流量受白名单约束
- 租户：工具处理结果中的凭证不会泄漏到 LLM 响应

### 成功标准

| 指标 | 目标 |
|------|------|
| P1 集成项全部完成 | SafetyInterceptor + SecureTransport + redirect 防护均接入 |
| 零回归 | 现有测试全部通过，`go test ./... -race` 绿 |
| Admin API 可用 | safety/egress/secrets 三个 handler 通过 `RequireScope("admin")` 注册 |
| P99 性能开销 | safety 层 < 50ms（来自 ironclaw bench_test.go 基线） |
| 高风险工具迁移 | 10 个高风险工具使用 SecretResolver 而非 os.Getenv |

---

## 用户故事

### US-1: Agent Loop 安全拦截 (P1)

**作为** 企业安全管理员  
**我希望** agent 每轮对话在发送给 LLM 前后自动执行 safety 检测  
**以便** prompt injection 攻击被实时拦截并记录  
**验收标准：** SafetyInterceptor.ProcessInput / ProcessOutput 在 agent.go RunConversation 循环中被调用；拦截事件写入审计日志

### US-2: 工具出站 HTTP 管控 (P1)

**作为** 企业合规团队  
**我希望** 所有工具发出的 HTTP 请求经过 SecureTransport 过滤  
**以便** 工具不能访问未授权的外部地址  
**验收标准：** 50 个工具的 http.Client 替换为 SecureTransport；redirect 目标校验防止内网绕过；未在 allowlist 的域名返回 403

### US-3: 凭证泄漏防护 (P1/P2)

**作为** 租户管理员  
**我希望** 工具运行时的凭证通过 SecretResolver 注入而非环境变量  
**以便** 凭证不会出现在工具调用日志或 LLM 响应中  
**验收标准：** 10 个高风险工具（web_search/http_request/email_send 等）迁移到 SecretResolver；LeakScanner 在 output_guard 中启用

### US-4: Admin API 统一配置 (P2)

**作为** 管理员  
**我希望** 通过 REST API 管理 safety policy / egress rules / secret patterns  
**以便** 安全策略可在运行时动态更新  
**验收标准：** 三个 admin handler 注册到 `/admin/v1/` 路径，均包裹 `RequireScope("admin")`；OpenAPI spec 同步更新

---

## 范围

### In Scope

- SafetyInterceptor 接入 agent.go RunConversation 循环 (#38)
- SecureTransport 替换 50 工具的 http.Client，分批迁移（#36）
- CheckRedirect hook 防止 redirect 绕过 IP 白名单 (#37)
- 10 个高风险工具迁移 SecretResolver (#39)
- Admin API 三个 handler 统一注册 (#40)
- Canary token TTL 清理（防内存增长）(#41)
- ResolvedValues 接口限制（可访问 key 范围约束）(#42)
- Unicode NFKC normalization for input_guard (#43)
- CI linter rule 禁止工具层 os.Getenv (#44)
- Redis 缓存 egress rules（性能优化）(#45)

### Out of Scope

- WASM sandbox（已推迟，见 ADR-006）
- 新增 safety pattern 内容（使用现有 patterns.go 中的规则）
- WebUI 安全配置界面（产品需求待定，见 backlog #33/34）
- 非 HTTP 协议的出站管控（仅覆盖 HTTP/HTTPS）

---

## 风险与依赖

| 风险 | 影响 | 缓解 |
|------|------|------|
| 50 工具迁移引入回归 | 高 | 分批：先高风险 web/http 类 10 个，再 API 类 40 个；每批跑完整测试 |
| SafetyInterceptor 性能影响流式响应 | 中 | output_guard 在流式完成后集中扫描，不在 token 级别拦截 |
| Admin API 与现有 server 注册冲突 | 低 | 已有 admin handler 模式（receipts/pricing 等），复用同路由注册结构 |
| os.Getenv linter 误报 | 低 | 白名单非工具层路径（main.go/config.go 等） |

### 关键依赖

- `internal/safety`, `internal/egress`, `internal/secrets` 包已通过测试（IronClaw PR 已合并）
- DB migrations 000001/000002 需在集成前执行
- `RequireScope("admin")` 中间件已存在（v1.4.0 RBAC 引入）

### 待确认项

1. **工具迁移优先级**：50 个工具中，除明确的 10 个高风险工具外，剩余 40 个的迁移顺序是否需要按 `net/http` 调用静态分析排序？
2. **Safety policy 默认值**：生产部署时 safety interceptor 默认是 `enforce` 还是 `audit` 模式？（影响是否阻塞现有对话）
3. **Redis 缓存时机**：egress rules Redis 缓存（#45）是否进入本 sprint 还是后续单独迭代？
4. **测试策略**：agent loop interceptor 集成是否需要新增 E2E 测试用例，还是单元测试覆盖已足够？

---

## 企业治理

- **应用等级**：继承 HermesX v2.2.x，T2 级（多实例，高可用）
- **数据/合规风险**：safety policy 存储在 PG，含租户隔离；需确认 RLS 策略覆盖 safety_policies 表
- **ADR 关联**：ADR-006（WASM 推迟）；本次可能新增 ADR 描述 SecureTransport 迁移策略

---

## 参与角色

| 角色 | 职责 |
|------|------|
| tech-lead | 范围确认、风险仲裁、设计收口 |
| architect | SecureTransport 迁移架构、safety 集成设计 |
| backend-engineer | 所有 #36-#45 实现 |
| qa-engineer | 集成测试验证、E2E 安全场景 |
| devops-engineer | CI linter rule (#44)、DB migration 执行验证 |
