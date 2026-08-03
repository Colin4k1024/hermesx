# Delivery Plan — Go Architecture Remediation

**slug:** go-arch-remediation  
**状态:** plan  
**主责角色:** tech-lead  
**日期:** 2026-07-31  
**关联 PRD:** [prd.md](prd.md)

---

## 版本目标

**里程碑:** Go 架构审查修复 v1  
**范围:** 修复 2 个严重 bug + 5 个高优先级问题（挑战会后调整）  
**放行标准:** 全部修复通过 `go test ./... -race`，无新增 CRITICAL/HIGH 问题

### 挑战会后范围调整

| 变更 | 原因 |
|------|------|
| B2（LLM 无超时）升级为 A5 | 生产中有 goroutine 泄漏风险，不亚于 A 级 |
| B1（错误格式统一）推迟 | 涉及 API 行为变更，需先确认前端兼容性 |
| B3（types.go 拆分）保持 B 级 | 代码组织改进，不影响运行时 |
| B5（MaxBytesReader）保持 B 级 | 安全加固，不影响运行时 |
| B6（启动校验）保持 B 级 | 防御性改进，不影响运行时 |

**最终范围（8 项）:**

| 级别 | 编号 | 问题 | 修复方案 |
|------|------|------|---------|
| 🚨 S | S1 | WaitGroup 未初始化，shutdown 不完整 | 加 `wg.Add(1)` + `defer wg.Done()` |
| 🚨 S | S2 | os.Chdir 并发不安全 | 审计 job 依赖后选 exec.Command 或 per-goroutine workdir |
| 🔴 A | A1 | os.Exit 在库代码 | 改为 return error |
| 🔴 A | A2 | context.Background() 在中间件 | 使用 `context.WithTimeout(context.Background(), 5s)` |
| 🔴 A | A3 | 错误链断裂（7 处） | `%v` → `%w` |
| 🔴 A | A4 | Config.Save() 密钥持久化 | Save 前脱敏敏感字段 |
| 🔴 A | A5 | LLM 调用无超时 | 添加 context.WithTimeout |
| 🟡 B | B4 | mcpcatalog 静默忽略错误 | 添加 slog.Warn |

---

## 工作拆解（Story Slices）

### PR1: 并发安全修复（Wave 1）

| Story | 目标 | 验收标准 | Owner | 依赖 | 估时 |
|-------|------|---------|-------|------|------|
| **S1-fix** | 修复 scheduler WaitGroup 泄漏 | `Stop()` 在 pollLoop 退出后正常返回；新增 `TestSaasSchedulerStop` 验证 | backend-engineer | 无 | 0.5d |
| **S2-fix** | 修复 cron os.Chdir 并发问题 | 并发 job 不互相干扰 CWD；新增 `TestConcurrentJobsWithWorkdir` | backend-engineer | S2-audit 完成 | 1d |
| **S2-audit** | 审计 cron job 的实际依赖 | 明确哪些 job 依赖进程内状态，确定 S2 修复方案 | backend-engineer | 无 | 0.5d |

**PR1 估时:** 2 天  
**分支策略:** 从 main 切 `fix/concurrent-safety`

### PR2: 错误处理与上下文修复（Wave 1，与 PR1 并行）

| Story | 目标 | 验收标准 | Owner | 依赖 | 估时 |
|-------|------|---------|-------|------|------|
| **A1-fix** | 移除库代码中的 os.Exit | `server.go` 返回错误；`saas.go` 正确处理退出 | backend-engineer | 无 | 0.5d |
| **A2-fix** | 修复 Redis 限流器 context | `Allow()` 使用请求 ctx | backend-engineer | 无 | 0.5d |
| **A3-fix** | 修复 7 处错误包装 | 所有 `fmt.Errorf` 使用 `%w`；`errors.Is` 可穿透 | backend-engineer | 无 | 0.5d |
| **A5-fix** | 添加 LLM 调用超时 | batch/cron 的 LLM 调用有 120s 超时 | backend-engineer | 无 | 0.5d |
| **B4-fix** | mcpcatalog 错误日志 | upsert 失败记录 warn 日志 | backend-engineer | 无 | 0.25d |

**PR2 估时:** 2.25 天  
**分支策略:** 从 main 切 `fix/error-handling-context`

### PR3: 安全与配置修复（Wave 2，依赖 PR2 合并）

| Story | 目标 | 验收标准 | Owner | 依赖 | 估时 |
|-------|------|---------|-------|------|------|
| **A4-fix** | Config.Save() 脱敏 | Save 输出的 YAML 不含明文 APIKey/AccessKey/SecretKey | backend-engineer | 无 | 0.5d |
| **A4-test** | 补充 config 保存测试 | `TestSaveRedactsSensitiveFields` 验证脱敏 | backend-engineer | A4-fix | 0.5d |

**PR3 估时:** 1 天  
**分支策略:** 从 main 切 `fix/config-save-redaction`

---

## 风险与缓解

| 风险 | 影响 | 缓解措施 | Owner |
|------|------|---------|-------|
| S2 exec.Command 改变 job 语义 | 依赖进程内状态的 job 失败 | 先完成 S2-audit，确认无进程内依赖后再选方案 | backend-engineer |
| A1 移除 os.Exit 后退出码变化 | 运维脚本依赖 exit code | 在 saas.go 的 RunE 中显式 `os.Exit(1)` 保持一致 | backend-engineer |
| A4 脱敏破坏配置迁移 | 依赖 Save 完整输出的场景 | 保留 `SaveFull()` 方法，脱敏版为默认 | backend-engineer |
| 测试覆盖不足导致回归 | 修复引入新 bug | 每个 PR 必须通过 `go test -race` | backend-engineer |
| PR 合并冲突 | 3 个 PR 改动不同包，冲突概率低 | 无额外缓解 | — |

---

## 节点检查

| 节点 | 时间 | 检查内容 | 主责角色 |
|------|------|---------|---------|
| 方案评审 | Day 0 | 修复方案确认、分支策略确认 | tech-lead |
| S2-audit 完成 | Day 1 | cron job 依赖审计结论 | backend-engineer |
| PR1 提交 | Day 3 | 并发安全修复 + 测试 | backend-engineer |
| PR2 提交 | Day 3 | 错误处理修复 + 测试 | backend-engineer |
| PR3 提交 | Day 4 | 配置脱敏修复 + 测试 | backend-engineer |
| Code Review | Day 4-5 | 全部 PR review | tech-lead |
| QA 回归 | Day 5-6 | `go test -race` + 手动验证 | qa-engineer |
| 合并 & 发布 | Day 6 | 合并到 main，观察 | devops-engineer |

---

## 角色分工

| 角色 | 职责 | 交接顺序 |
|------|------|---------|
| **tech-lead** | 方案评审、PR review、冲突仲裁 | 全程 |
| **backend-engineer** | 实施全部修复、编写测试 | Day 1-4 |
| **qa-engineer** | 回归验证、race detector | Day 5-6 |
| **devops-engineer** | 合并后部署观察 | Day 6 |

---

## 是否需要 ADR

| 决策点 | 需要 ADR | 说明 |
|--------|---------|------|
| S2 修复方案选型 | ✅ 是 | exec.Command vs per-goroutine workdir，影响 cron job 执行语义 |
| A4 Save 脱敏策略 | ❌ 否 | 实现细节，不涉及架构决策 |
| 其他修复项 | ❌ 否 | 都是明确的 bug fix，无架构决策 |

**ADR 编号:** ADR-010（在 `/team-execute` 阶段产出）

---

## 技能装配清单

| 技能 | 启用原因 | 主责角色 |
|------|---------|---------|
| `golang-patterns` | 惯用 Go 模式 | backend-engineer |
| `golang-testing` | 测试编写 | backend-engineer |
| `karpathy-guidelines` | 最小化改动范围 | 全员 |

---

## 前端影响评估

**本轮无前端变更。** B1（错误格式统一）已推迟，不影响 webui。

---

## Implementation-Readiness 结论

| 维度 | 状态 | 说明 |
|------|------|------|
| 需求挑战会 | ✅ 完成 | 3 个核心假设已挑战，范围已调整 |
| 设计收口 | ✅ 完成 | 8 项修复方案已确定 |
| 角色分工 | ✅ 完成 | 4 角色职责明确 |
| 风险清单 | ✅ 完成 | 5 项风险已识别并有缓解 |
| 分支策略 | ✅ 完成 | 3 个 PR 各自分支 |
| ADR 需求 | ✅ 识别 | S2 需要 ADR-010 |
| 前端影响 | ✅ 评估 | 无前端变更 |

**就绪状态:** `handoff-ready`  
**可进入 `/team-execute`:** ✅ 是（S2-audit 需在 execute 阶段首先完成）
