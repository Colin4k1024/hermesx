# Delivery Plan: SaaS 定时调度功能（Distributed Cron Scheduler）

> **状态**: draft | **阶段**: plan | **角色**: tech-lead  
> **日期**: 2026-05-19 | **slug**: saas-cron-scheduler

---

## 1. 版本目标

| 项目 | 说明 |
|------|------|
| 版本 | v2.4.0 cron-scheduler |
| 范围 | gocron v2 + Redis Locker SaaS 调度器，PG 持久化，执行历史，tools/cronjob.go 改写 |
| 放行标准 | 3 Pod 模拟下同一 job 一个周期内仅执行 1 次；租户隔离通过；执行历史可查询 |

---

## 2. 需求挑战会结论

### 核心假设挑战与收口

| # | 质疑 | 来源 | 结论 | 处理 |
|---|------|------|------|------|
| C1 | Redis locker 失败时无保护，退化为多 Pod 同时执行 | product-manager | **接受风险，加幂等层** | cron_job_runs 加 UNIQUE(cron_job_id, scheduled_at) 约束，DB 层防双写 |
| C2 | deliver=local 时用户无感知执行结果 | product-manager | **本期用 API 查询 cron_job_runs** | 补 GET /cron-jobs/{id}/runs 端点，deliver=local 写 DB，用户可查 |
| C3 | gocron-redis-lock 不支持 lock 续租，LLM 超 TTL 导致双执行 | architect | **TTL 加倍 + 幂等约束** | exec_timeout=5min，lock TTL=**12min**（2x buffer）；cron_job_runs 唯一约束兜底 |
| C4 | RunConversation 非幂等，双执行意味着重复投递 | architect | **本期 deliver=local 可接受**；DB 唯一约束防重复写入 | cron_job_runs UNIQUE 约束 INSERT ON CONFLICT DO NOTHING |
| C5 | PG ListDue 多 Pod 重复全表扫描 | architect | **Pod 数 < 10 可接受**，不加分片 | 记录为已知限制，Pod > 10 时评估 SKIP LOCKED 方案 |
| C6 | Story A→B 串行假设拉长关键路径 | project-manager | **B 先用接口 stub 并行开发** | 定义 SchedulerStore interface + 内存实现，migration 完成后替换 |
| C7 | RunConversation 是否支持 context deadline 传入 | project-manager | **已确认**：factory.go 接受 ctx，可传 5min deadline | Story C 实现时用 context.WithTimeout |

### 阻断条件核查

| # | 条件 | 状态 |
|---|------|------|
| B1 | Redis HA 部署（单点宕机全停调度） | **已确认**：SaaS Redis 为 Redis Sentinel，v2.3.0 已验证 |
| B2 | gocron-redis-lock 续租 API | **不依赖续租**：采用 TTL=12min + DB 幂等约束代替，不要求 lock extension |
| B3 | Go 版本 ≥ 1.21（gocron v2 要求） | **待验证**：Story B 开始前检查 go.mod |

---

## 3. Story 拆解

### Story A — DB Migration（独立，可最先并行）

**目标**：建立 cron_job_runs 表 + 补充 cron_jobs 字段  
**Owner**: backend-engineer  
**依赖**: 无  
**验收标准**:
- `cron_job_runs` 表创建，含 UNIQUE(cron_job_id, scheduled_at) 约束和 RLS policy
- `cron_jobs` 表补 `last_run_success boolean`、`last_run_error text` 字段
- migration 文件命名规范，幂等可重复执行

### Story B — SaasScheduler 核心（并行，用 stub 解除对 A 的依赖）

**目标**：实现 `internal/scheduler/` 包，gocron v2 + Redis locker + PG 轮询同步  
**Owner**: backend-engineer  
**依赖**: Story A（仅最终集成时替换 stub）  
**验收标准**:
- `SaasScheduler.Start(ctx)` / `Stop()` 正确管理 gocron lifecycle
- 30s 轮询从 PG 同步 job：新增注册、变更重调度、禁用删除
- Redis locker 竞争成功才执行，失败 skip（可通过日志观察）
- `gocron.WithSingletonMode(LimitModeReschedule)` 防同 Pod 并发

### Story C — executor.go（依赖 B 接口）

**目标**：实现 job 执行逻辑，集成 agent/factory.RunConversation，写 cron_job_runs  
**Owner**: backend-engineer  
**依赖**: Story B 接口定义（不依赖 B 实现完成）  
**验收标准**:
- `context.WithTimeout(ctx, 5*time.Minute)` 强制超时
- 执行前 INSERT cron_job_runs（ON CONFLICT DO NOTHING 幂等）
- 执行后 UPDATE status/result/duration
- 启动时扫描 status=running AND started_at < now()-12min，标记为 failed（stale lock 清理）

### Story D — tools/cronjob.go 改写

**目标**：SaaS 模式下写 PG，CLI 模式保留文件系统  
**Owner**: backend-engineer  
**依赖**: 无（Story A migration 完成后可集成测试）  
**验收标准**:
- 通过 `config.IsSaasMode()` 或依赖注入区分两条路径
- SaaS 路径调用 `store.CronJobs().Create/Update/Delete`
- CLI 路径行为不变，现有 CLI 测试通过

### Story E — saas.go 启动集成

**目标**：在 SaaS 启动路径初始化并启动 SaasScheduler  
**Owner**: backend-engineer  
**依赖**: Story B、C、D  
**验收标准**:
- `saas.go` 启动时 `scheduler.New(cfg).Start(ctx)`
- graceful shutdown：`server.Shutdown` 前先 `scheduler.Stop()`
- 环境变量配置：`CRON_POLL_INTERVAL`（默认 30s）、`CRON_EXEC_TIMEOUT`（默认 5m）、`CRON_LOCK_TTL`（默认 12m）

### Story F — 测试（分批，按依赖链）

**F1 — 多 Pod 去重**（依赖 B）  
- 用 3 个 goroutine 模拟 3 个 Pod 的 SaasScheduler，同一 job 触发，断言 cron_job_runs 只有 1 条记录

**F2 — 执行历史**（依赖 C/D）  
- job 执行后 cron_job_runs 有正确的 status/result/duration/pod_id

**F3 — 租户隔离**（依赖 A）  
- 租户 A 的 job 不出现在租户 B 的 ListRuns 结果中；RLS 验证

---

## 4. 关键路径（并行化后）

```
A（migration）─────────────────────────────────────┐
B（stub → PG 实现，依赖 A 仅集成替换）─┐            │
C（executor，依赖 B 接口）────────────┤──► E（saas.go 集成）
D（cronjob.go 改写）──────────────────┘            │
F1（← B） F2（← C/D） F3（← A）─────────────────────┘
```

**估算**：并行后关键路径 7-8 天（vs 串行 12+ 天）

---

## 5. 角色分工

| 角色 | 职责 | Stories |
|------|------|---------|
| tech-lead | 方案收口、风险仲裁、arch-design review | 全程 |
| architect | 接口设计、SaasScheduler 骨架设计 | B 接口 |
| backend-engineer | 全部实现 | A B C D E |
| qa-engineer | 测试计划、F1/F2/F3 验证 | F |

---

## 6. 风险与缓解

| 风险 | 影响 | 缓解 | Owner |
|------|------|------|-------|
| gocron v2 版本与 Go 版本不兼容 | B 阻塞 | Story B 前先验证 go.mod 兼容性 | backend-engineer |
| Redis lock TTL 不足（极端慢 LLM） | 双执行 | TTL=12min + DB 幂等约束双保险 | architect |
| ListDue 全表扫描性能（中长期） | PG 负载 | 当前 Pod < 10 可接受；未来加 SKIP LOCKED | tech-lead |
| cron_jobs stale lock 残留（Pod 崩溃）| 任务永久卡 running | executor 启动时清理 stale records | backend-engineer |

---

## 7. 节点检查

| 节点 | 条件 | Owner |
|------|------|-------|
| 方案评审 | arch-design.md 完成，challenge 收口 | ✅ 本文档完成 |
| 开发完成 | Story A-E 合并，go test ./... 全通 | backend-engineer |
| 测试完成 | F1/F2/F3 全通，多 Pod 去重验证 | qa-engineer |
| 发布准备 | deployment-context.md + launch-acceptance.md 完成 | devops-engineer |

---

## 8. 技能装配

| 技能 | 触发原因 | 主责 |
|------|---------|------|
| golang-patterns | Go 分布式调度器实现 | backend-engineer |
| golang-testing | 多 goroutine 并发测试、race detector | qa-engineer |
| database-migrations | cron_job_runs 表 + RLS policy | backend-engineer |
| security-review | 租户隔离、RLS 正确性 | qa-engineer |

---

## 9. implementation-readiness 结论

| 项目 | 状态 |
|------|------|
| PRD 存在，待确认项已收口 | ✅ |
| Requirement Challenge Session 完成（7条质疑收口） | ✅ |
| arch-design.md 完成 | ✅ |
| 阻断条件 B1 核查通过（Redis HA 确认） | ✅ |
| 阻断条件 B2 处理（不依赖续租，TTL+幂等） | ✅ |
| 阻断条件 B3（Go 版本）| ⚠️ Story B 开始前验证 |
| Story slice 边界清晰，可独立交接 | ✅ |

**就绪状态**: `handoff-ready`（B3 为低风险验证项，不阻塞启动）
