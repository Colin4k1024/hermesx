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

**更新时间**: 2026-05-18 | **更新角色**: devops-engineer

| # | 事项 | 优先级 | 来源 | Owner | 状态 |
|---|------|--------|------|-------|------|
| 36 | 工具层 HTTP client 迁移到 SecureTransport (50 tools 逐个迁移) | P1 | C3 CRITICAL | backend-engineer | 待定 |
| 37 | HTTP redirect 绕过防护 — CheckRedirect hook | P1 | C4 CRITICAL | backend-engineer | 待定 |
| 38 | Agent loop interceptor 集成 (safety interceptor 接入 agent.go) | P1 | S1.6 | backend-engineer | 待定 |
| 39 | 高风险 10 个工具迁移 SecretResolver | P2 | S2.4 | backend-engineer | 待定 |
| 40 | Admin API 统一交付 (Safety + Egress + Secret patterns) | P2 | S1.5/S2.5 | backend-engineer | 待定 |
| 41 | Canary token TTL 清理 + RemoveToken 集成 | P2 | H5 | backend-engineer | 待定 |
| 42 | ResolvedValues 接口限制 | P3 | H6 | backend-engineer | 待定 |
| 43 | Unicode NFKC normalization for input guard | P3 | M3 | backend-engineer | 待定 |
| 44 | Linter rule 禁止 os.Getenv (CI 集成) | P3 | S2.6 | devops-engineer | 待定 |
| 45 | Redis 缓存 egress rules (性能优化) | P3 | S3.5 | backend-engineer | 待定 |

## 技术债

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 27 | RLS SELECT policies 评估 | P3 | 读隔离需求确认 | architect | 待定 |
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
- [ ] **[P3] RLS SELECT policies 评估**: 对已部署的 RLS SELECT 策略进行系统性评估，确认读隔离边界是否正确覆盖所有查询路径。 (Owner: architect, Label: security)
- [x] **[P3] pgxmock 引入**: 已完成 v2.2.0。新增 apikey_test.go：接口断言 + bootstrap 幂等逻辑单元测试 + SQL 形状验证（scopes COALESCE, ON CONFLICT idempotency）。 (Owner: backend-engineer, Label: testing)
- [ ] **[P3] CORS 动态管理**: 将 CORS 配置从环境变量扩展为可从数据库或配置中心动态加载，支持多域名按租户配置。 (Owner: backend-engineer, Label: enhancement)
- [ ] **[P3] LocalDualLimiter 多副本精确性优化**: 在 Redis 频繁故障时，多副本部署下的 LocalDualLimiter 精确性不足，需设计更优的分布式协调方案。 (Owner: backend-engineer, Label: performance)
- [x] **[P3] HasScope empty scopes 策略**: 关闭 v2.2.0。接受兼容策略：空 scopes = legacy 兼容访问（非 admin）。新建 key 已携带显式 scopes。文档已更新到 internal/auth/context.go。 (Owner: backend-engineer, Label: bug→accepted)
- [x] **[P3] GHA actions digest-pin**: 已完成 v2.2.0。release.yml 中 actions/checkout, actions/setup-go, softprops/action-gh-release 均已 pin 到 commit SHA。 (Owner: devops-engineer, Label: security)

### P4 - 长远规划

- [ ] **[P4] Unicode bidi chars sanitization**: 在 LLM 安全要求升级后，对所有进入 prompt 的文本执行 Unicode Bidi 字符清理，防止 Unicode 文本混淆攻击。 (Owner: backend-engineer, Label: security)
- [ ] **[P4] Admin UI for pricing rules**: 产品需求确认后，开发定价规则管理的前端界面。 (Owner: frontend-engineer, Label: feature)
- [ ] **[P4] 用量 dashboard / 计费报告**: 产品需求确认后，开发用量统计和计费报告前端界面。 (Owner: frontend-engineer, Label: feature)
- [ ] **[P4] GDPR 自助数据导出 UI**: 合规需求明确后，开发 GDPR 自助数据导出前端界面，支持租户管理员自主导出数据。 (Owner: frontend-engineer, Label: feature)

### v2.3.0 Security Enhancement - 下一迭代 (来源: 2026-05-18)

- [ ] **[P1] SecureTransport 工具层迁移 (C3)**: 50 个工具的 HTTP client 逐步迁移到 SecureTransport（含 DialContext IP 验证 + DNS rebinding 防护）。批次策略：先高风险 web/http 类工具，再 API 类工具。 (Owner: backend-engineer, Label: security)
- [ ] **[P1] HTTP redirect 绕过防护 (C4)**: 为 SecureTransport 添加 CheckRedirect hook，验证 redirect 目标不指向内网/CGNAT/localhost。当前 DialContext 仅对直接请求有效，redirect 可绕过。 (Owner: backend-engineer, Label: security)
- [ ] **[P1] Agent loop interceptor 集成 (S1.6)**: 将 SafetyInterceptor 接入 agent.go 的对话循环，在 LLM 调用前后执行 input_guard + output_guard + canary 检测。接口已就绪，需集成测试验证。 (Owner: backend-engineer, Label: security)
- [ ] **[P2] 高风险工具迁移 SecretResolver (S2.4)**: 10 个高风险工具（web_search, http_request, email_send 等）迁移到 SecretResolver 模式，禁止直接 os.Getenv 读取 secret。 (Owner: backend-engineer, Label: security)
- [ ] **[P2] Admin API 统一交付 (S1.5/S2.5)**: Safety policy + Egress rules + Secret patterns 三个 Admin handler 统一接入主 server，必须包裹 RequireScope("admin") 中间件。 (Owner: backend-engineer, Label: feature)
- [ ] **[P2] Canary token TTL 清理 (H5)**: 为 canary token map 增加 TTL 过期清理机制，防止长期运行后内存持续增长。Token 对象 ~60 bytes，短期无 OOM 风险但需限制上界。 (Owner: backend-engineer, Label: reliability)
- [ ] **[P3] ResolvedValues 接口限制 (H6)**: 限制 ToolContext.SecretResolver 可解析的 key 范围，防止工具 handler 访问非授权 secret。当前工具在 Docker sandbox 中隔离运行，风险可控。 (Owner: backend-engineer, Label: security)
- [ ] **[P3] Unicode NFKC normalization (M3)**: 对 input_guard 输入做 Unicode NFKC 规范化，防止通过等价字符绕过注入检测规则。 (Owner: backend-engineer, Label: security)
- [ ] **[P3] Linter rule 禁止 os.Getenv (S2.6)**: CI 集成静态检查，标记工具代码中直接调用 os.Getenv 的位置，推动迁移到 SecretResolver。 (Owner: devops-engineer, Label: tooling)
- [ ] **[P3] Redis 缓存 egress rules (S3.5)**: AllowlistPolicy 当前每次请求查 DB，高流量场景需引入 Redis 缓存 + TTL 失效策略。interface 已预留。 (Owner: backend-engineer, Label: performance)

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
- [ ] **[P2] 共享 Transport 连接池生产验证**：per-call `http.Client{Transport: sharedTransport}` 在开发/staging 负载下经 -race 验证安全，需在生产流量下确认无连接池泄漏。(Owner: backend-engineer, Label: reliability)
- [x] **[P2] Canary goroutine 双实例统一**：✅ 完成 2026-05-27。API server 内 Admin token 管理、API chat、workflow agent executor 共享同一个 `CanaryDetector` / `SafetyInterceptor`。
- [ ] **[P3] Admin DI 完整重构**：AdminHandler struct 字段注入已完成，但 main.go 侧仍为 singleton 初始化。下一 sprint 完整迁移到 server-level DI 容器，消除测试竞态风险。(Owner: backend-engineer, Label: maintainability)
- [ ] **[P3] WASM sandbox（ADR-006）**：工具隔离沙箱，原定 v2.3.0 但 ADR-006 推迟，待安全需求明确后重新规划。(Owner: architect, Label: security)

### 2026-05-27 Batch 1 已完成

- [x] **API/Workflow SafetyInterceptor 注入**：API chat 与 workflow agent executor 共用 server-level `SafetyInterceptor`、`LeakScanner` 和 `CanaryDetector`，避免 admin 控制面与运行时检测分叉。
- [x] **Tool HTTP tenant/path 上下文注入**：Eino tool adapter 通过 tenant-aware RoundTripper 自动把 tenant 与 URL path 写入 SecureTransport context，避免工具用 `http.NewRequest` 时丢失治理上下文。
- [x] **Evolution policy 多实例刷新**：`GeneStore.RefreshSharingPolicies` 支持主动刷新，`StartSharingPolicyWatcher` 支持周期 watcher，API server `Shutdown` 会停止 watcher。
