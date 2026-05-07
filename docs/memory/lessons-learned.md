# Lessons Learned

---

## 2026-05-07 — Phase 3: 认证链 passthrough 模式需要显式文档化

**场景：** OIDC extractor 的 `return nil, nil` passthrough 在安全评审中被标记为 HIGH — JWT 验证失败时静默跳过，审计无法追踪。

**问题：**
1. ExtractorChain 设计中，`return nil, nil` 表示"不是我的 token"，但 JWT-shaped token 验证失败也走同一路径，导致攻击者可绕过 OIDC 验证。
2. 空 tenant_id claim 被默认映射为 "default" tenant，形成权限提升路径。
3. OIDC provider discovery 无超时上限，startup 阶段 IdP 不可达时服务永久阻塞。

**建议：**
1. 认证链中"不是我的"决策必须区分"token 格式不匹配"和"token 格式匹配但验证失败"— 后者必须返回 error 而非 nil。
2. 多租户系统中，任何 claim 缺失都应拒绝认证，不能用默认值兜底 — 尤其是 tenant_id。
3. 外部依赖的初始化必须有 bounded timeout — 15s 上限足以覆盖网络抖动，但不会无限阻塞 startup。

---

## 2026-05-07 — Phase 3: 流式接口的断路器保护需要独立机制

**场景：** ChatStream 走 `breaker.Execute` wrapper 导致 double-count — 流式接口的生命周期与 request/response 模型不匹配。

**问题：**
1. `gobreaker.Execute` 设计为同步 request/response，流式接口强行适配导致成功也被计为一次 Execute 调用，双倍统计。
2. 流式 goroutine 无 context cancellation 退出路径，调用者放弃后 goroutine 泄漏。
3. 并行 `/team-execute` 对 Go 后端极其高效 — 三个 slice 完全独立（不同包），零冲突交付。

**建议：**
1. 流式接口的断路器保护应使用独立的 failure recording 机制（如手动 `RecordFailure`），不强行走 Execute wrapper。
2. 所有 goroutine 必须有 `select { case <-ctx.Done() }` 退出路径 — 流式场景尤为关键。
3. Go 的包级隔离天然支持并行 slice 执行 — 独立文件/独立包的 story 可以放心并行，不需要串行等待。

---

## 2026-05-07 — Enterprise SaaS GA v1.2.0: 接口设计要预留 context 和 sentinel error

**场景：** Phase 2 DualLayerLimiter 接口初版无 context 参数，store 层 Delete 泄漏 pgx.ErrNoRows 到 handler。

**问题：**
1. 接口发布后补 context 参数，导致所有实现和调用者级联编译失败 — 5 个文件需要同步修改。
2. handler 直接依赖 pgx-specific 错误类型，导致所有 DB 错误都映射为 404，运维监控盲区。
3. Admin API 无输入验证，NaN/Inf/负数价格和注入字符可直接写入数据库。

**建议：**
1. Go 接口第一版就要带 `context.Context` 参数 — 即使当前实现不需要，未来任何 IO 操作都需要。
2. 引入 `store.ErrNotFound` sentinel error 解耦 handler 和驱动层 — handler 用 `errors.Is()` 分类，驱动细节留在 store 内部。
3. HTTP handler 入口做完整输入验证 (regex + math.IsNaN/IsInf + 非负) — 不让无效数据进入 store 层。
4. Redis 多 key Lua 脚本必须用 hash tag `{tenantID}` 保证 Cluster 同 slot 路由。

---

## 2026-05-07 — 全链路 /team-* 主链协作模式验证

**场景：** enterprise-saas-ga 从 intake → plan → execute → review → release → closeout 完整走完两个 Phase。

**问题：**
1. Phase 1 首次 review 暴露 5 个 CRITICAL/HIGH blocking items — 说明 execute 阶段自测覆盖不足。
2. Phase 2 review 一次 GO — 说明 execute 阶段吸取了 Phase 1 教训，同步补了验证。
3. closeout 阶段容易遗漏 backlog 回写，导致后续 Phase 没有明确候选项来源。

**建议：**
1. execute 阶段对安全关键代码 (auth, RBAC, tenant isolation) 同步写单元测试 — 不留到 review 再补。
2. 每个 Phase 完成后立即回写 backlog，不等 closeout — 减少遗忘和上下文丢失。
3. sentinel error、接口签名等架构性决策在 plan 阶段 ADR 中预判，不在 execute/review 中临时发现。

---

## 2026-04-28 — SaaS Readiness: 安全审查驱动的批量修复

**场景：** P0-P5 一次性交付 23 个新文件后，并行 code-reviewer + security-reviewer 发现 29 个安全问题（5 CRITICAL + 10 HIGH + 9 MEDIUM + 5 LOW）。

**问题：**
1. 批量交付后的安全审查修复成本高 — 5 CRITICAL + 6 HIGH 需要跨 7 个文件的协调修改。
2. Pre-existing 安全问题（CRIT-1 ACP auth bypass）在新代码审查中被发现，但修复涉及 out-of-scope 代码。
3. 新增代码虽然编译和集成测试通过，但缺少专门的单元测试，导致安全修复缺乏回归保护。

**建议：**
1. 每个 Phase 交付后立即运行安全审查，不要等全量完成 — 修复成本随积累指数增长。
2. Pre-existing 安全问题应在 intake 阶段显式列入 backlog 并评估优先级，不要等到新代码审查时才发现。
3. 新增安全关键代码（auth、RBAC、tenant isolation）应在实现阶段同步补充单元测试，不要作为"后续补充"。
4. Store interface 扩展（如 `GetByID`）应在设计阶段预判，避免安全修复时才发现缺少必要的数据访问方法。

---

## 2026-04-30 — Enterprise Hardening: Requirement Challenge 的价值

**场景：** Phase 1-5 企业级加固，6 轮需求挑战将原始计划从"高风险并行"调整为"分阶段可控交付"。

**问题：**
1. 原计划 Phase 1 并行 OIDC + RBAC + RLS + 无状态化 — 四个高风险改造同期，任一阻塞则整个 Phase 停摆。
2. 外部依赖（企业 IdP）未确认就排进 Phase 1，导致关键交付路径不可控。
3. 组件依赖顺序错误 — Lifecycle Manager 排在 Phase 4，但 Phase 2 引入的 OTel/断路器已需要生命周期管理。

**建议：**
1. Requirement Challenge Session 优先识别外部依赖 — 将依赖外部 IdP 的工作推后，优先交付"内部可控"slice。
2. 依赖分析要前置 — 新组件引入基础设施依赖（如后台 goroutine 需 Lifecycle Manager），必须在设计阶段发现。
3. 安全机制分阶段激活 — RLS 可延后，先用集成测试覆盖应用层路径，降低并行风险。
4. 用户体验关键路径（SSE 真实流式）不应排到最后版本 — 感知延迟是第一印象，应在中期交付。

---

## 2026-04-30 — gofmt CI 门禁的价值

**场景：** Phase 1-5 全量代码推送后，CI gofmt 检查发现 8 个文件格式不合规，需要额外修复 commit。

**问题：**
1. 多文件并行编辑时，部分文件在工具调用中产生格式漂移（多余空行、缩进不一致）。
2. gofmt 未在本地预提交检查中执行，问题在 CI 层才暴露，增加了一个 fix commit。

**建议：**
1. Go 代码每次 commit 前执行 `gofmt -l .` — 零容忍格式问题，不留到 CI 阶段。
2. 多文件并行写入时，完成后统一运行 `gofmt -w .` 再检查，不依赖 IDE 自动格式化。
3. Makefile 的 `make lint` target 应包含 gofmt 检查，作为本地门禁。

---
