# Backlog Snapshot

**来源任务**: 2026-05-22-platform-governance-remediation
**更新时间**: 2026-05-27
**更新角色**: tech-lead

---

## 2026-05-22 Platform Governance Remediation 残余项

| # | 事项 | 优先级 | 触发条件 | Owner | 状态 |
|---|------|--------|----------|-------|------|
| 1 | 多实例 evolution sharing policy 主动刷新 / watcher | P1 | 多副本部署或跨实例治理回滚上线前 | backend-engineer | ✅ 完成 2026-05-27（`RefreshSharingPolicies` + server lifecycle watcher） |
| 2 | 平台治理中心 Web UI（policy history / rollback / revoke 控制面） | P1 | 内部平台治理进入运营阶段 | frontend-engineer | 待定 |
| 3 | OIDC / API key / RoleStore 统一权限 evaluator 收敛 | P1 | 下一轮权限治理整改启动时 | backend-engineer | 待定 |
| 4 | 企业 release gate 与恢复演练 artifact 补齐 | P2 | 发布演练或 release-ready 评审前 | devops-engineer | 待定 |
| 5 | delivery-plan.md 历史 markdown lint 清理 | P3 | 文档治理窗口开启时 | writer | 待定 |

---

## v0.12 Absorption 残余项

| # | 事项 | 优先级 | 触发条件 | Owner | 状态 |
|---|------|--------|----------|-------|------|
| 1 | LifecycleHooks 接入 Gateway Runner | P1 | — | backend-engineer | ✅ 完成 v1.4.0 |
| 2 | SelfImprover 接入 Agent 循环 | P2 | — | backend-engineer | ✅ 完成 v1.4.0 |
| 3 | compress.go / curator.go prompt sanitization 一致性 | P2 | — | backend-engineer | ✅ 完成 v2.0.0 |
| 4 | payload.URL 路径遍历检查扩展 | P2 | — | backend-engineer | ✅ 完成 v2.0.0 |
| 5 | Curator O(n²) dedup 优化 | P3 | MaxMemories > 100 需求 | backend-engineer | ✅ 完成 v2.2.0 |
| 6 | Unicode bidi chars sanitization | P4 | LLM 安全要求升级 | backend-engineer | ✅ 完成 v2.2.1（sanitizeForPrompt 扩展覆盖 U+061C, U+200E-F, U+202A-E, U+2066-9；测试用例已补充） |

## Phase 3 已完成项

| # | 事项 | 状态 | 完成版本 |
|---|------|------|----------|
| 7 | OIDC wiring 到 server.go auth chain | ✅ 完成 | v1.3.0 |
| 8 | 断路器 Prometheus metrics + failure recording | ✅ 完成 | v1.3.0 |
| 9 | CI/CD Pipeline (GitHub Actions + ghcr.io) | ✅ 完成 | v1.3.0 |

## v1.4.0 已完成项

| # | 事项 | 状态 | 完成版本 |
|---|------|------|----------|
| 10 | ExecutionReceipt API + store + ReceiptRecorder | ✅ 完成 | v1.4.0 |
| 11 | Auditor RBAC 角色 + execution-receipts 路由守卫 | ✅ 完成 | v1.4.0 |
| 12 | Prometheus Business Metrics (tool/chat/store) | ✅ 完成 | v1.4.0 |
| 13 | OpenAPI 规范扩展（22 端点，完整 schema） | ✅ 完成 | v1.4.0 |
| 14 | Production Compose (PostgreSQL 16 + Redis 7 + MinIO) | ✅ 完成 | v1.4.0 |
| 15 | OTel + Jaeger 链路追踪 | ✅ 完成 | v1.4.0 |
| 16 | 备份/恢复脚本 (pg_dump) | ✅ 完成 | v1.4.0 |
| 17 | Model Catalog hot-reload + CJK trigram search | ✅ 完成 | v1.4.0 |
| 18 | Gateway platform registry refactor | ✅ 完成 | v1.4.0 |
| 19 | MultimodalRouter (image/audio/video dispatch) | ✅ 完成 | v1.4.0 |
| 20 | Autonomous Memory Curator | ✅ 完成 | v1.4.0 |
| 21 | Self-improvement Loop | ✅ 完成 | v1.4.0 |
| 22 | Gateway Media Parity + Lifecycle Hooks | ✅ 完成 | v1.4.0 |

## v2.0.0 待完成项

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 23 | HermesX 品牌文档同步（所有文档 branding 更新） | P1 | v2.0.0 release 前 | writer | ✅ 完成 v2.2.0 |
| 24 | v2.0.0 Release Notes 编写 | P1 | v2.0.0 release 前 | writer | ✅ 完成 v2.2.0 |
| 25 | ExecutionReceipt API 文档完善（集成示例 + 幂等性说明） | P2 | 文档同步 sprint | writer | ✅ 完成 v2.2.0 |
| 26 | API Reference 文档与代码端点完整对齐（见 api-reference.md） | P2 | — | writer | ✅ 完成 v2.2.0（embedded OpenAPI spec 已更新：title/version/contact + 全部路由对齐） |

## v2.3.0 候选（来源: 2026-05-18 security-enhancement-ironclaw）

**更新时间**: 2026-06-01 | **更新角色**: backend-engineer

| # | 事项 | 优先级 | 来源 | Owner | 状态 |
|---|------|--------|------|-------|------|
| 36 | 工具层 HTTP client 迁移到 SecureTransport (50 tools 逐个迁移) | P1 | C3 CRITICAL | backend-engineer | ✅ 完成 v2.3.0（主路径：agent.go sharedTransport + tctx.HTTPClient；次级工具 browser_impl.go/mcp_sse.go/osv_check.go 存在技术债，见 #46） |
| 37 | HTTP redirect 绕过防护 — CheckRedirect hook | P1 | C4 CRITICAL | backend-engineer | ✅ 完成 v2.3.1（agent.go CheckRedirect 加 egress.ValidateRedirectTarget；对齐 tooladapter.go）|
| 38 | Agent loop interceptor 集成 (safety interceptor 接入 agent.go) | P1 | S1.6 | backend-engineer | ✅ 完成 v2.3.0 |
| 39 | 高风险 10 个工具迁移 SecretResolver | P2 | S2.4 | backend-engineer | ✅ 完成 v2.3.0（web.go/vision.go/discord_tool.go 等；browser_impl.go 为全局 singleton，独立计入 #46） |
| 40 | Admin API 统一交付 (Safety + Egress + Secret patterns) | P2 | S1.5/S2.5 | backend-engineer | ✅ 完成 v2.3.0 |
| 41 | Canary token TTL 清理 + RemoveToken 集成 | P2 | H5 | backend-engineer | ✅ 完成 v2.3.1（server.go 调用 canaryDetector.StartCleanupLoop；TTL=24h，随 backgroundCtx 停止）|
| 42 | ResolvedValues 接口限制 | P3 | H6 | backend-engineer | ✅ 完成 v2.3.0（WithAllowedKeys wrapper in internal/secrets/resolver.go:82；ResolvedValues 已按 allowed set 过滤；ErrKeyNotAllowed；6 单元测试） |
| 43 | Unicode NFKC normalization for input guard | P3 | M3 | backend-engineer | ✅ 完成 v2.3.0 |
| 44 | Linter rule 禁止 os.Getenv (CI 集成) | P3 | S2.6 | devops-engineer | ✅ 完成 v2.3.0（.golangci.yml forbidigo pattern '^os\.Getenv$'；warn-only；// fallback 行排除；允许 osv_check.go init() 的端点配置） |
| 45 | Redis 缓存 egress rules (性能优化) | P3 | S3.5 | backend-engineer | ✅ 完成 v2.3.0（internal/egress/cache.go CachedEgressPolicy；TTL 60s；wired in server.go:120；InvalidateTenant + Reload；7 单元测试） |
| 46 | browser_impl.go SecureTransport + SecretResolver 迁移 | P3 | C3 残留 | backend-engineer | ✅ 完成 v2.3.2（BrowserBackend.Connect() 接口已改为 `Connect(ctx context.Context, tctx *ToolContext) error`；BrowserbaseBackend.Connect 通过 SecretResolver 解析 BROWSERBASE_API_KEY/PROJECT_ID 并降级到 os.Getenv；LocalBrowserBackend.Connect 签名对齐；getOrCreateBackend/selectBackend 全部接受 ctx/tctx；所有 20 个调用点已更新；TODO(#46) 注释已移除；browser_impl.go 中 os 包依赖已消除；编译 + 单测通过；Admin DI：APIServerConfig 三个 safety 字段已添加 DI 入口）|
| 47 | MCP SamplingHandler safety 集成（pre-existing 测试缺失实现） | P3 | 测试债务修复 | backend-engineer | ✅ 完成 2026-06-01（NewSamplingHandlerWithSafety 已实现；input check 在 LLM 调用前，output check 在调用后；TestSamplingHandlerSafetyBlocksInputBeforeLLM + TestSamplingHandlerSafetyBlocksOutput 通过） |

## 技术债

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 27 | RLS SELECT policies 评估 | P3 | 读隔离需求确认 | architect | ✅ 完成（审计结论：所有主租户表 SELECT USING 覆盖完整；发现 2 个 FORCE 缺口已补：migration 109 为 execution_receipts 加 FORCE；migration 110 为 egress_rules 补 ENABLE+FORCE+policy；bootstrap_state 为平台单例，无需 RLS） |
| 28 | pgxmock 引入 (store 层 mock 测试) | P3 | — | backend-engineer | ✅ 完成 v2.2.0（SQL 形状测试 + 接口断言） |
| 29 | CORS 动态管理 (DB/config 加载) | P3 | 多域名需求出现 | backend-engineer | 待定 |
| 30 | 多副本 LocalDualLimiter 精确性优化 | P3 | 生产 Redis 频繁故障 | backend-engineer | 待定 |
| 31 | HasScope empty scopes 放行修复 | P3 | — | backend-engineer | ✅ 关闭：接受兼容策略。Legacy 空 scopes 允许非 admin 访问，新建 key 已携带显式 scopes。见 internal/auth/context.go 策略注释。 |
| 32 | GHA actions digest-pin | P3 | — | devops-engineer | ✅ 完成 v2.2.0 |

## 产品需求候选

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 33 | Admin UI for pricing rules | P3 | 产品需求确认 | frontend-engineer |
| 34 | 用量 dashboard / 计费报告 | P3 | 产品需求确认 | frontend-engineer |
| 35 | GDPR 自助数据导出 UI | P4 | 合规需求 | frontend-engineer |

---

## GitHub Issues 草稿

### P1 - 必须完成

- [x] **[P1] LifecycleHooks 接入 Gateway Runner**: ✅ 完成 v1.4.0。Gateway Runner 已集成 LifecycleHooks 执行链路。
- [x] **[P1] HermesX 品牌文档同步**: ✅ 完成 v2.2.0。所有文档、OpenAPI spec（title/contact）、英文文档页面已同步 rebrand。
- [x] **[P1] v2.0.0 Release Notes 编写**: ✅ 完成 v2.2.0。docs/CHANGELOG.en.md 已包含 v2.x 变更记录。

### P2 - 应该做

- [x] **[P2] SelfImprover 接入 Agent 循环**: ✅ 完成 v1.4.0。SelfImprover 已集成到 Agent 对话循环。
- [x] **[P2] prompt sanitization 一致性**: ✅ 完成 v2.0.0。sanitizeForPrompt 已从 self_improve.go 提取为 internal/agent/sanitize.go，compress.go 和 curator.go 均统一调用。
- [x] **[P2] payload.URL 路径遍历检查扩展**: ✅ 完成 v2.0.0。URL 路径遍历检查已扩展覆盖更多协议。
- [x] **[P2] ExecutionReceipt API 文档完善**: ✅ 完成 v2.2.0。docs/api-reference.en.md ExecutionReceipts 章节已扩展：receipt 对象字段说明、status 枚举语义、idempotency_id 使用方式与幂等行为、与审计日志的对比关系、带安全重试的 curl 集成示例。 (Owner: writer, Label: documentation)
- [x] **[P2] API Reference 与代码端点对齐**: ✅ 完成 v2.2.0。embedded OpenAPI spec 已对齐所有路由；TestOpenAPISpec_AllPathsPresent 和 TestOpenAPISpec_InfoBranding 作为回归保护。

### P3 - 可以做

- [x] **[P3] Curator O(n²) dedup 优化**: 已完成 v2.2.0。Phase 1 用 map 精确 key 去重 O(n)，Phase 2 仅对 key-unique 集合做内容相似度比较，MaxMemories=100 时性能显著改善。 (Owner: backend-engineer, Label: performance)
- [x] **[P3] RLS SELECT policies 评估**: ✅ 完成 v2.3.2。已对全部主租户表 SELECT USING 覆盖进行确认；御甩 2 个 FORCE 缺口：migration 109 为 execution_receipts 加 FORCE，migration 110 为 egress_rules 补 ENABLE+FORCE+policy。 (Owner: architect, Label: security)
- [x] **[P3] pgxmock 引入**: 已完成 v2.2.0。新增 apikey_test.go：接口断言 + bootstrap 幂等逻辑单元测试 + SQL 形状验证（scopes COALESCE, ON CONFLICT idempotency）。 (Owner: backend-engineer, Label: testing)
- [ ] **[P3] CORS 动态管理**: 将 CORS 配置从环境变量扩展为可从数据库或配置中心动态加载，支持多域名按租户配置。 (Owner: backend-engineer, Label: enhancement)
- [ ] **[P3] LocalDualLimiter 多副本精确性优化**: 在 Redis 频繁故障时，多副本部署下的 LocalDualLimiter 精确性不足，需设计更优的分布式协调方案。 (Owner: backend-engineer, Label: performance)
- [x] **[P3] HasScope empty scopes 策略**: 关闭 v2.2.0。接受兼容策略：空 scopes = legacy 兼容访问（非 admin）。新建 key 已携带显式 scopes。文档已更新到 internal/auth/context.go。 (Owner: backend-engineer, Label: bug→accepted)
- [x] **[P3] GHA actions digest-pin**: 已完成 v2.2.0。release.yml 中 actions/checkout, actions/setup-go, softprops/action-gh-release 均已 pin 到 commit SHA。 (Owner: devops-engineer, Label: security)

### P4 - 长远规划

- [x] **[P4] Unicode bidi chars sanitization**: ✅ 完成 v2.2.1。sanitizeForPrompt 已扩展覆盖 U+061C, U+200E-F, U+202A-E, U+2066-9；测试用例已补充。 (Owner: backend-engineer, Label: security)
- [ ] **[P4] Admin UI for pricing rules**: 产品需求确认后，开发定价规则管理的前端界面。 (Owner: frontend-engineer, Label: feature)
- [ ] **[P4] 用量 dashboard / 计费报告**: 产品需求确认后，开发用量统计和计费报告前端界面。 (Owner: frontend-engineer, Label: feature)
- [ ] **[P4] GDPR 自助数据导出 UI**: 合规需求明确后，开发 GDPR 自助数据导出前端界面，支持租户管理员自主导出数据。 (Owner: frontend-engineer, Label: feature)

### v2.3.0 Security Enhancement (来源: 2026-05-18) — ✅ 全部完成

- [x] **[P1] SecureTransport 工具层迁移 (C3)**: ✅ 完成 v2.3.0 + v2.3.2。主路径：agent.go sharedTransport + tctx.HTTPClient；browser_impl.go BrowserBackend.Connect() 已迁移 SecureTransport + SecretResolver（#46）。 (Owner: backend-engineer, Label: security)
- [x] **[P1] HTTP redirect 绕过防护 (C4)**: ✅ 完成 v2.3.1。agent.go CheckRedirect 加 egress.ValidateRedirectTarget；抳归 loopback/private/CGNAT/link-local。 (Owner: backend-engineer, Label: security)
- [x] **[P1] Agent loop interceptor 集成 (S1.6)**: ✅ 完成 v2.3.0。SafetyInterceptor 已接入 agent.go 对话循环。 (Owner: backend-engineer, Label: security)
- [x] **[P2] 高风险工具迁移 SecretResolver (S2.4)**: ✅ 完成 v2.3.0。web.go/vision.go/discord_tool.go 等 10 个高风险工具已迁移到 SecretResolver 模式。 (Owner: backend-engineer, Label: security)
- [x] **[P2] Admin API 统一交付 (S1.5/S2.5)**: ✅ 完成 v2.3.0。Safety + Egress + Secret 三个 Admin handler 已统一接入主 server，包裹 RequireScope("admin")。 (Owner: backend-engineer, Label: feature)
- [x] **[P2] Canary token TTL 清理 (H5)**: ✅ 完成 v2.3.1。server.go 调用 canaryDetector.StartCleanupLoop；TTL=24h，随 backgroundCtx 停止。 (Owner: backend-engineer, Label: reliability)
- [x] **[P3] ResolvedValues 接口限制 (H6)**: ✅ 完成 v2.3.0。WithAllowedKeys wrapper 已实现；ErrKeyNotAllowed；6 个单元测试。 (Owner: backend-engineer, Label: security)
- [x] **[P3] Unicode NFKC normalization (M3)**: ✅ 完成 v2.3.0。input_guard 已对输入做 NFKC 规范化。 (Owner: backend-engineer, Label: security)
- [x] **[P3] Linter rule 禁止 os.Getenv (S2.6)**: ✅ 完成 v2.3.0。.golangci.yml forbidigo pattern '^os\.Getenv$'；osv_check.go init() 的绝对路径配置已保留。 (Owner: devops-engineer, Label: tooling)
- [x] **[P3] Redis 缓存 egress rules (S3.5)**: ✅ 完成 v2.3.0。CachedEgressPolicy TTL=60s；InvalidateTenant + Reload；7 单元测试。 (Owner: backend-engineer, Label: performance)

---

## v2.1.0-webui 遗留项（来源: 2026-05-08-hermesx-webui closeout）

**更新时间**: 2026-05-08 | **更新角色**: devops-engineer

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 36 | Bootstrap 端点 IP 速率限制（应用层 + Nginx） | ✅ 完成 | v2.2.0-stabilization | backend-engineer |
| 37 | useSse 401/403 响应自动登出 | ✅ 已完成 | v2.1.1+ | frontend-engineer |
| 38 | Bootstrap 跨实例 TOCTOU → `bootstrap_state` 原子 claim | ✅ 完成 | v2.2.0-stabilization | backend-engineer |

### P1 - 必须完成

- [x] **[P1] Bootstrap 端点 IP 速率限制**: POST /admin/v1/bootstrap 已增加应用层 IP 限流，并在 WebUI Nginx 与生产 Nginx LB 配置叠加 `limit_req`。(Owner: backend-engineer, Label: security)

### P2 - 应该做

- [x] **[P2] useSse 401/403 auto-logout**: useSse.ts 已在 401/403 时调用 `disconnectUser()` 并返回 Session expired。(Owner: frontend-engineer, Label: security)
- [x] **[P2] Bootstrap DB unique constraint**: 改为 `bootstrap_state` 原子 claim，覆盖不同 bootstrap name 的跨实例竞态，比 `(tenant_id, name)` 约束更准确。(Owner: backend-engineer, Label: reliability)

---

## v2.3.0 Security Integration Sprint 遗留项（来源: 2026-05-18-v230-security-integration closeout）

**更新时间**: 2026-05-18 | **更新角色**: tech-lead

### 已完成（v2.3.0 原 backlog 候选项）

| # | 事项 | 完成版本 |
|---|------|---------|
| C4 (partial) | SecureTransport redirect count 限制 | ✅ v2.3.0 |
| S1.6 | SafetyInterceptor 接入 agent.go 对话循环 | ✅ v2.3.0 |
| S2.4 | 高风险工具 SecretResolver 迁移 | ✅ v2.3.0 |
| S1.5/S2.5 | Admin API 三 handler 统一注册 + RequireScope | ✅ v2.3.0 |
| H5 | Canary token TTL 清理 goroutine | ✅ v2.3.0 |
| H6 | ResolvedValues 接口限制（WithAllowedKeys） | ✅ v2.3.0 |
| M3 | Unicode NFKC normalization（input_guard） | ✅ v2.3.0 |
| S2.6 | forbidigo linter（warn-only）| ✅ v2.3.0 |
| S3.5 | CachedEgressPolicy（TTL 60s）| ✅ v2.3.0 |

### 遗留项（v2.4.0 目标）

- [x] **[P1] per-tenant EgressPolicy**：✅ 完成 v2.4.0-dev。运行时通过 `NewAllowlistPolicyFromEnv` 使用租户 allowlist，生产环境默认 `deny-all`，开发环境保留显式 override。
- [x] **[P2] redirect 目标 IP 验证**：✅ 完成 2026-05-27。tool HTTP client 的 CheckRedirect 现在拒绝 loopback/private/CGNAT IP literal，后续实际连接仍由 SecureTransport DialContext 做 DNS/IP 校验。
- [x] **[P2] 共享 Transport 连接池生产验证**：✅ 已验证 2026-06-xx。单一 `*http.Transport` 在 `NewAPIServer` 创建一次，通过 `http.Client{Transport: egressTransport}` 共享给所有 tool call；连接池由 transport 统一管理，符合 Go net/http 设计；per-call `http.Client` 只复用 transport 引用，不会泄漏独立连接池。源码审查确认无连接池泄漏风险，无需代码修改。(Owner: backend-engineer, Label: reliability)
- [x] **[P2] Canary goroutine 双实例统一**：✅ 完成 2026-05-27。API server 内 Admin token 管理、API chat、workflow agent executor 共享同一个 `CanaryDetector` / `SafetyInterceptor`。
- [ ] **[P3] Admin DI 完整重构**：AdminHandler struct 字段注入已完成，但 main.go 侧仍为 singleton 初始化。下一 sprint 完整迁移到 server-level DI 容器，消除测试竞态风险。(Owner: backend-engineer, Label: maintainability)
- [ ] **[P3] WASM sandbox（ADR-006）**：工具隔离沙箱，原定 v2.3.0 但 ADR-006 推迟，待安全需求明确后重新规划。(Owner: architect, Label: security)

### 2026-05-27 Batch 1 已完成

- [x] **API/Workflow SafetyInterceptor 注入**：API chat 与 workflow agent executor 共用 server-level `SafetyInterceptor`、`LeakScanner` 和 `CanaryDetector`，避免 admin 控制面与运行时检测分叉。
- [x] **Tool HTTP tenant/path 上下文注入**：Eino tool adapter 通过 tenant-aware RoundTripper 自动把 tenant 与 URL path 写入 SecureTransport context，避免工具用 `http.NewRequest` 时丢失治理上下文。
- [x] **Evolution policy 多实例刷新**：`GeneStore.RefreshSharingPolicies` 支持主动刷新，`StartSharingPolicyWatcher` 支持周期 watcher，API server `Shutdown` 会停止 watcher。
