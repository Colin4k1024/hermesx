# Delivery Plan: v2.3.0 Security Integration Sprint

> **版本目标**: v2.3.0  
> **Owner**: tech-lead  
> **状态**: draft → handoff-ready（挑战会 + 设计收口完成后更新）  
> **日期**: 2026-05-18  
> **估计工期**: P1+P2 约 50h（单人），P3 条件进入 +17h

---

## 版本目标

将 IronClaw 安全子系统（safety/egress/secrets）从"已构建"推进到"生产可用"：完成三条核心防线的接入（prompt injection 拦截、出站 HTTP 管控、凭证泄漏防护），Admin API 统一可配置。

**放行标准**：
- P1 三项全部完成，`go test ./... -race` 绿，新增失败用例 ≤ 5
- Safety interceptor 以 audit 模式上线
- Admin API safety/egress/secrets 三组端点通过 `RequireScope("admin")` 注册

---

## 需求挑战会结论

**会议日期**: 2026-05-18  
**参与角色**: product-manager, project-manager, architect

### product-manager 质疑

- **质疑**：Safety audit 模式上线，若无日志消费方，等于"安全外观"而非真实防护
- **替代路径**：仅对高风险 10 工具完成端到端闭环，其余 40 工具顺延 v2.4.0
- **阻断条件**：若无明确 audit 日志消费方 + enforce 升级 SLA，SafetyInterceptor 接入应缩小为仅 LeakScanner
- **结论（接受）**：audit 模式接入为有价值的第一步；同时在 delivery-plan 中明确"audit → enforce 升级由 Admin API 配置触发，文档需说明升级标准"

### project-manager 质疑

- **质疑**：#36+#37 共享 http.Client 改造路径，分两次提交存在 redirect 裸窗口
- **替代路径**：轨道 A（#36+#37 原子 PR）+ 轨道 B（#38 并行开发）
- **阻断条件**：P1 回归 > 5 个新失败用例，P3 不进本 sprint
- **结论（接受）**：#36+#37 强制原子提交为同一 PR；#38 并行开发；P3 准入以回归结果为触发条件

### architect 质疑

- **质疑**：SafetyInterceptor 流式响应语义（chunk 级 vs 事后扫描）；Transport 逐工具替换侵入面过广；CheckRedirect 破坏 OAuth 工具
- **替代路径**：ToolContext 统一注入（Registry-Level）；redirect-depth 分级 + per-tool 覆盖
- **阻断条件**：如有工具正常业务路径依赖 redirect chain 且目标域不在 allowlist，必须先完成 per-tool policy 设计
- **结论（接受）**：D1 采用事后审计；D2 采用 ToolContext 注入；D3 采用 redirect-depth 分级

---

## Story Slice（可独立执行单元）

### Story A — SafetyInterceptor 接入 RunConversation（#38）

**目标**：在 `agent.go` RunConversation 循环中挂载 SafetyInterceptor，audit 模式不阻塞对话。

**涉及文件**：
- `internal/agent/agent.go`（接入点：streamingAPICall 返回后）
- `internal/safety/interceptor.go`（接口，只读）

**验收标准**：
- 完整响应后触发 CheckOutput；audit 模式下违规仅记录日志
- 单元测试覆盖：命中/未命中/interceptor error 三条路径

**估时**：8h  
**依赖**：无（可与 Story B 并行）  
**Owner**：backend-engineer

---

### Story B — SecureTransport + CheckRedirect 原子改造（#36 + #37）

**目标**：两项合并单 PR，消除 redirect 裸窗口。扩展 `ToolContext.HTTPClient` 字段，在 `executeSingleTool` 中注入 SecureTransport；同时实现 CheckRedirect hook（redirect-depth 分级）。

**涉及文件**：
- `internal/agent/agent.go`（executeSingleTool，注入 HTTPClient）
- `internal/tools/registry.go`（ToolContext 扩展）
- `internal/egress/transport.go`（CheckRedirect hook，MaxRedirects 支持）
- `internal/tools/*.go`（分批替换 http.DefaultClient → tctx.HTTPClient；先高风险 10 个，后 40 个）

**验收标准**：
- `go vet` + lint 零 bare `&http.Client{}` 出现
- redirect loop 测试：301→302→目标，hook 截断非白名单域
- P1 回归新增失败用例 ≤ 5

**估时**：16h（含分批 + 集成测试）  
**依赖**：必须原子提交，#36 和 #37 不可拆分  
**Owner**：backend-engineer

---

### Story C — 高风险 10 工具迁移 SecretResolver（#39）

**目标**：将 10 个高风险工具的凭证读取从 `os.Getenv` 替换为 `secrets.SecretResolver`。

**高风险工具清单**：
1. `discord_tool.go` — `DISCORD_BOT_TOKEN`
2. `web.go` — `EXA_API_KEY`
3. `web.go` — `FIRECRAWL_API_KEY`
4. `homeassistant.go` — `HASS_TOKEN`
5. `vision.go` — `FAL_KEY`
6. `vision.go` — `AUXILIARY_VISION_API_KEY`
7. `vision.go` — `OPENROUTER_API_KEY`
8. `mcp_sse.go` — MCP SSE 端点鉴权 token
9. `messaging.go` — Gateway bearer token
10. `tts.go` — TTS API key（`TTS_API_KEY`）

**涉及文件**：上述 10 个工具文件 + `internal/secrets/resolver.go`

**验收标准**：
- `grep -r "os.Getenv" internal/tools/` 仅余非凭据类 env（`PATH`、`HOME`、`TMPDIR`、`TERMINAL_CWD`）
- 集成测试：SecretResolver 注入后工具功能正常

**估时**：10h  
**依赖**：Story B 完成（ToolContext 已扩展）  
**Owner**：backend-engineer

---

### Story D — Admin API 统一注册（#40）

**目标**：在 `internal/api/admin/handler.go` 按现有 `/admin/v1/` 模式注册 safety、egress、secrets 管理端点，均包裹 `RequireScope("admin")`。

**端点清单**：

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/admin/v1/safety/rules` | 列出当前 safety policy 规则 |
| PUT | `/admin/v1/safety/rules/{id}` | 更新规则（audit ↔ block 切换） |
| POST | `/admin/v1/safety/scan` | 手动触发扫描（调试） |
| GET | `/admin/v1/egress/allowlist` | 列出域名白名单 |
| POST | `/admin/v1/egress/allowlist` | 添加白名单条目 |
| DELETE | `/admin/v1/egress/allowlist/{id}` | 删除条目 |
| GET | `/admin/v1/egress/blocked-log` | 查询拦截记录（分页） |
| GET | `/admin/v1/secrets/patterns` | 列出 secret patterns（已有） |
| POST | `/admin/v1/secrets/patterns` | 添加 pattern（已有） |
| GET | `/admin/v1/secrets/canary-tokens` | 列出活跃 canary token |
| DELETE | `/admin/v1/secrets/canary-tokens/{id}` | 手动清除 canary token |

**涉及文件**：
- `internal/api/admin/handler.go`（注册路由）
- 新增 `internal/api/admin/safety.go`（safety handler）
- 新增 `internal/api/admin/egress_rules.go`（egress handler）
- `internal/api/admin/secrets.go`（已有，扩展 canary 端点）

**估时**：10h  
**依赖**：三子系统接口稳定（已满足）  
**Owner**：backend-engineer

---

### Story E — Canary Token TTL 清理（#41）

**目标**：在 `internal/secrets` 注册后台 goroutine，按 TTL 定期清除过期 canary token，防止长期运行内存增长。

**涉及文件**：
- `internal/secrets/canary.go`（新增 `StartCleanupLoop`）
- `cmd/hermesx/main.go`（注入清理 goroutine）

**验收标准**：benchmark 测试：插入 10k token 并等待 2× TTL，确认内存回收

**估时**：6h  
**依赖**：无（可与 Story A 并行）  
**Owner**：backend-engineer

---

### Story F — P3 条件批（#42、#43、#44、#45）

**准入条件**：Story B 完成后，P1 回归新增失败用例 ≤ 5

| # | 事项 | 文件 | 估时 |
|---|------|------|------|
| 42 | ResolvedValues 接口限制 | `internal/secrets/resolver.go` | 4h |
| 43 | Unicode NFKC normalization for input_guard | `internal/safety/input_guard.go` | 4h |
| 44 | CI linter rule 禁止工具层 os.Getenv | `.golangci.yml` | 3h |
| 45 | Redis 缓存 egress rules | `internal/egress/cache.go`（新增） | 6h |

**Story F 总估时**：17h（条件进入）

---

## 工期与依赖总览

```
Week 1
  Day 1-2:  Story A + Story E (并行，~14h)
  Day 2-4:  Story B (原子 PR，16h，关键路径)

Week 2
  Day 1-2:  Story C (依赖 B，10h)
  Day 1-2:  Story D (可与 C 并行，10h)

  [P1 回归检查] → 若 ≤ 5 失败 → Story F 进入
  Day 3-4:  Story F (17h，条件)
```

**P1+P2 关键路径**：B(16h) → C(10h) = 26h 串行，加 A+D+E 并行，总 wall-clock 约 3 工作日

---

## 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| #36+#37 原子 PR 引入多处回归 | P1 放行延迟 | 分批：先完成 ToolContext 注入骨架，后分 2 batch 迁移工具 |
| Safety audit 日志无消费方 | 安全价值无法验证 | 在 Story D Admin API 中提供 safety scan 端点，作为临时消费入口 |
| OAuth 工具 redirect 目标未在 allowlist | 工具功能中断 | Story B 前审查所有持有 http.Client 工具，标记需要 MaxRedirects 覆盖的工具 |
| Canary token map 无 TTL，内存增长 | 长期运行 OOM 风险 | Story E 必须进本 sprint（P2） |
| DB migrations 未在集成测试前执行 | 测试失败（safety_policies 表不存在） | 在 CI 集成测试 setup 阶段加入 migration 步骤 |

---

## 节点检查

| 检查点 | 条件 | 负责角色 |
|--------|------|----------|
| Story A + B 完成后 | `go test ./... -race` 绿，回归 ≤ 5 | backend-engineer → qa-engineer |
| Story D 完成后 | Admin API 所有端点通过 Postman 冒烟 | backend-engineer → tech-lead |
| P3 准入决策 | 基于 B 回归结果 | tech-lead |
| v2.3.0 放行 | 全部 P1+P2 完成，CI 绿，audit 日志验证 | qa-engineer → tech-lead |

---

## 技能装配

- `golang-patterns`（工具层批量改造）
- `golang-testing`（race + bench 测试）
- `security-review`（admin API 鉴权验证）

---

## Handoff 状态

- **当前阶段**: plan  
- **目标阶段**: handoff-ready（Design Review Board 通过后更新）  
- **就绪状态**: not-ready（待 tech-lead 主持 Design Review Board 收口）  
- **阻塞项**: 无硬阻塞；D1/D2/D3 三项架构决策已在 arch-design.md 中收口
- **accepted_by**: backend-engineer（设计收口后接手）
