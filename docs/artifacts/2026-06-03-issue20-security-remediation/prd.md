# PRD: Issue #20 安全与合规问题清单修复

> 状态：Draft | 日期：2026-06-03 | 主责：tech-lead | 来源：GitHub Issue #20

---

## 背景

GitHub Issue #20 于 2026-06-03 提出 18 条问题（6 致命 + 2 高危 + 3 中危 + 7 低危），覆盖安全、合规、可观测性和灾备领域。

**关键发现：** 经代码库实际验证，18 条中有 **7 条已在当前代码中修复或不准确**，实际需要处理的问题为 **11 条**（3 致命 + 1 高危 + 1 中危 + 6 低危）。

---

## Issue #20 逐条验证结论

### 已修复 / 不准确（7 条，建议关闭或标记已解决）

| # | 原始描述 | 验证结论 | 证据 |
|---|----------|----------|------|
| 1 | `rand.Read` 错误未处理 | **已修复** | `internal/api/apikeys.go:generateRawKey()` 正确 `if _, err := rand.Read(b); err != nil { return "", ... }` |
| 2 | API Key 创建时 tenant_id 可从 body 指定 | **不准确** | `internal/api/admin/apikeys.go` 中 tenant_id 从 URL path 取值 `chi.URLParam(r, "tenant_id")`，且验证 tenant 存在 |
| 3 | ExecutionReceipt 完全缺失 | **已实现** | `internal/tools/receipt_recorder.go` + `internal/api/execution_receipts.go` 存在完整实现 |
| 4 | 无 SAST/DAST/依赖漏洞扫描 | **已配置** | `.github/workflows/security.yml` 包含 govulncheck + gosec + trivy + CodeQL |
| 5 | 缺少跨租户自动化渗透测试 | **已实现** | `tests/integration/cross_tenant_attack_test.go` 387 行，含 API Key 边界测试和内存泄漏测试 |
| 6 | OpenAPI 规范缺失 | **已实现** | `internal/api/openapi.go` + `internal/api/openapi_test.go` |
| 7 | 无 Grafana Dashboard 和 Alert Rules | **已实现** | `deploy/prometheus/alerts.yml` + `deploy/grafana/` 目录存在 |

### 真实待修复问题（11 条）

#### 致命（3 条）

| ID | 问题 | 风险 | 当前状态 |
|----|------|------|----------|
| C1 | AI Agent 自主写入公开仓库 / 代码投毒风险 | 供应链攻击 | `user_notes.md` + `.claude/` + `.deepseek/` 并存，需清理 + .gitignore 规则 |
| C2 | RLS 在 superuser 连接池下完全失效 | 多租户隔离崩溃 | `set_config('app.current_tenant', ...)` 存在但无 superuser 验证机制 |
| C3 | K8s 下 Sandbox 依赖 DinD，无 gVisor/Firecracker 隔离 | 容器逃逸 | `internal/tools/environments/k8sjob.go` 无任何安全容器运行时集成 |

#### 高危（1 条）

| ID | 问题 | 风险 | 当前状态 |
|----|------|------|----------|
| H1 | GitHub Actions 未 digest-pin | 供应链注入 | 所有 `uses:` 均为 tag 引用 (`@v4`, `@v5`, `@master`) |

#### 中危（1 条）

| ID | 问题 | 风险 | 当前状态 |
|----|------|------|----------|
| M1 | 计费无用量告警 | 产品缺失 | 聚合 API 已存在 (`usage_v2.go`)，但缺少阈值告警触发 |

#### 低危（6 条）

| ID | 问题 | 风险 |
|----|------|------|
| L1 | 多副本下 LocalDualLimiter 限流精确性下降 | Redis 故障时限流失效 |
| L2 | Redis / MinIO 无独立备份方案 | 灾备不完整 |
| L3 | 审计日志无归档策略，无 SIEM 集成 | 合规风险 + 表膨胀 |
| L4 | MinIO 对象 GDPR 清理为异步，可能残留 | 数据删除权违规 |
| L5 | GDPR 删除无软删除 / 宽限期机制 | 误删不可恢复 |
| L6 | PITR 恢复 RTO 未在生产数据量下验证 | SLA 承诺无支撑 |

---

## 目标与成功标准

### 业务目标

1. 消除 3 条致命安全风险，使 HermesX 满足企业级多租户安全基线
2. 修复供应链安全问题，CI/CD 达到 SLSA Level 2 标准
3. 补齐合规缺口（审计归档、GDPR 宽限期、灾备验证）

### 成功指标

- [ ] RLS 隔离验证工具可自动化执行，覆盖 superuser 场景
- [ ] K8s sandbox 至少提供一种安全运行时方案（gVisor RuntimeClass）
- [ ] Actions 100% digest-pinned，无 tag-only 引用
- [ ] 审计日志有 90 天归档策略 + 外部存储导出接口
- [ ] GDPR 删除支持 30 天软删除窗口
- [ ] 用量告警可按租户配置阈值

---

## 用户故事

1. **作为平台运维**，我需要确认 RLS 在任何连接方式下都有效，以保证多租户数据隔离不依赖连接凭据类型。
2. **作为安全工程师**，我需要 CI/CD 中所有第三方 Actions 通过 SHA digest 固定，以防止供应链注入攻击。
3. **作为合规审计员**，我需要审计日志有自动归档和外部导出能力，以满足合规框架的日志外存要求。
4. **作为企业客户**，我需要 GDPR 删除有宽限期和确认机制，以防止误操作导致不可逆数据丢失。
5. **作为 SRE**，我需要 K8s 环境中的代码执行沙箱具备安全容器运行时隔离，以防止容器逃逸影响集群。

---

## 范围

### In Scope

- RLS superuser 验证机制 + 自动化检测
- K8s sandbox gVisor RuntimeClass 集成
- GitHub Actions digest pinning
- 审计日志归档策略 + S3 导出
- GDPR 软删除宽限期机制
- 用量阈值告警
- 供应链卫生（`.gitignore` 清理 AI 残留文件）
- 多副本限流 Redis 故障降级改进
- Redis/MinIO 备份方案
- PITR 恢复演练

### Out of Scope

- 已验证修复的 7 条问题（标记关闭即可）
- 全面 SIEM 集成（本轮只做导出接口，SIEM 对接为下一阶段）
- Firecracker/Kata 级隔离（本轮以 gVisor 为目标）

---

## 风险与依赖

| 风险 | 影响 | 缓解 |
|------|------|------|
| gVisor RuntimeClass 需要 K8s 节点支持 | 部署环境约束 | 先在 GKE/EKS 验证，提供 fallback DinD + 文档告警 |
| RLS superuser 检测可能影响连接池性能 | 延迟增加 | 使用 PgBouncer `auth_query` + 连接时 `SET ROLE` |
| digest pinning 后 dependabot 无法自动升级 | 维护负担 | 配合 Renovate 自动化 digest 更新 PR |

---

## 批次规划（建议）

### Sprint 1 — 致命修复（预估 3 天）

| 任务 | 主责角色 |
|------|----------|
| C1: 清理 AI 残留文件 + .gitignore 规则 | backend-engineer |
| C2: RLS superuser 验证 + `SET ROLE` 连接池改造 | backend-engineer + database-reviewer |
| C3: K8s sandbox gVisor RuntimeClass 集成 | devops-engineer |
| H1: Actions digest pinning | devops-engineer |

### Sprint 2 — 合规补齐（预估 2 天）

| 任务 | 主责角色 |
|------|----------|
| L3: 审计日志归档策略 + S3 导出 | backend-engineer |
| L4+L5: GDPR 软删除宽限期 + 异步清理可靠性 | backend-engineer |
| M1: 用量阈值告警 | backend-engineer |

### Sprint 3 — 灾备加固（预估 2 天）

| 任务 | 主责角色 |
|------|----------|
| L1: 多副本限流降级方案 | backend-engineer |
| L2: Redis/MinIO 备份 runbook + 自动化 | devops-engineer |
| L6: PITR 生产级数据量演练 | devops-engineer |

---

## 待确认项

1. **RLS 改造方案选择**：`SET ROLE` vs. 专用非 superuser 连接池 — 需 DBA 评估性能影响
2. **gVisor 目标环境**：GKE Autopilot 原生支持 vs. 自建节点 RuntimeClass 安装
3. **审计归档目标**：S3 冷存储 vs. 专用日志服务（CloudWatch Logs / Loki）
4. **GDPR 宽限期时长**：30 天 vs. 配置化（需法务确认）
5. **Issue #20 关闭策略**：是否逐条 comment 验证结论后批量关闭已修复项

---

## 参与角色

| 角色 | 职责 |
|------|------|
| tech-lead | 优先级仲裁、方案收口 |
| backend-engineer | RLS 改造、GDPR 机制、审计归档、告警 |
| devops-engineer | gVisor 集成、Actions 固定、备份方案、PITR 演练 |
| security-reviewer | 方案安全评审、渗透测试补充 |
| database-reviewer | RLS + 连接池方案审查 |
| architect | sandbox 架构决策 |

---

## 需求挑战会候选分组

### 分组 A：多租户隔离（C2 + 跨租户测试增强）

- 参与：architect, backend-engineer, database-reviewer, security-reviewer
- 焦点：superuser RLS 绕过的根本解法、连接池改造范围、回归验证策略

### 分组 B：沙箱安全（C3）

- 参与：architect, devops-engineer, security-reviewer
- 焦点：gVisor vs Firecracker 选型、K8s RuntimeClass 部署路径、性能影响评估

### 分组 C：合规与灾备（L3 + L4 + L5 + L6）

- 参与：backend-engineer, devops-engineer, tech-lead
- 焦点：归档策略、GDPR 法务确认、PITR 演练标准

---

## 领域技能包启用建议

| 技能 | 触发原因 |
|------|----------|
| `security-reviewer` agent | 致命安全问题修复需安全评审 |
| `database-reviewer` agent | RLS + 连接池改造需 DB 专家 |
| `devops-engineer` agent | CI/CD + K8s + 备份方案 |
| `codegraph` MCP | 影响面分析（RLS 相关代码路径追踪） |

---

## 企业治理待确认项

- 应用等级：T2（多租户 SaaS，非关键业务主链路）
- 数据风险：涉及多租户隔离和 GDPR 合规，需按更严口径处理
- 技术架构等级：T2（多副本 K8s 部署，PostgreSQL 集群）
- 集团组件约束：无（自主项目，非集团统一架构管辖）
