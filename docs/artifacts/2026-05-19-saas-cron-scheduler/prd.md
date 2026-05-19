# PRD: SaaS 版本定时调度功能（Distributed Cron Scheduler）

> **状态**: draft | **阶段**: intake | **角色**: tech-lead  
> **日期**: 2026-05-19 | **slug**: saas-cron-scheduler

---

## 1. 背景

### 业务问题

Hermes Agent 在单机 CLI/gateway 模式下已具备完整的定时调度能力（`internal/cron/`，基于 `robfig/cron v3`，JSON 文件持久化）。用户可以创建 cron job，让 agent 按计划执行 prompt 并通过 Discord / 本地等渠道投递结果。

SaaS 化（多租户、无状态 Pod 部署）后，该能力完全断裂：
- `cron_jobs` PG 表已存在（migration v14–v16），API 层可以 CRUD 任务
- 但**没有任何 background scheduler 读取并执行这些任务**
- 多 Pod 部署时若直接迁移，会导致同一 job 在所有 Pod 上重复执行

### 当前约束

- SaaS 运行时：多租户 PostgreSQL（PG advisory lock 已用于 tenant_cleanup）+ Redis（session、metering 已在用）
- 已有 PG 端 `CronJobStore`（CRUD + `ListDue`）接口，但无消费者
- 已有 `agent/factory.go` 的 `RunConversation`，SaaS agent 执行入口
- 现有 `internal/cron/scheduler.go` 不能在 SaaS 模式下直接复用（文件系统状态、无租户隔离、无分布式锁）

---

## 2. 目标与成功标准

### 业务目标
- SaaS 模式下每个租户可以创建/管理定时 agent 任务，任务按 cron 表达式触发，agent 执行 prompt 并投递结果

### 用户价值
- 租户无需手动触发 agent，依赖调度自动执行，支持日报、监控告警、定期数据处理等场景

### 成功标准
| 指标 | 目标值 |
|------|--------|
| 多 Pod 下同一 job 重复执行率 | 0% |
| 调度延迟（cron 触发到 agent 开始执行）| ≤ 5s |
| Job CRUD → 调度生效时间 | ≤ 30s |
| 单 Pod 崩溃后任务恢复时间 | ≤ 下一个 cron 周期 |
| 租户数据隔离 | 严格，每个 job 只能访问所属 tenant 数据 |

---

## 3. 用户故事

### US-1：创建定时任务
**作为**租户用户，**我希望**通过 agent tool 或 API 创建一个每天 9am 运行的 job，**以便**自动获取日报摘要。
- 验收：job 写入 PG，30s 内被调度器感知，到达触发时间后 agent 执行，结果投递到指定渠道

### US-2：多 Pod 下去重保证
**作为**平台运维，**我希望**在 3 个 Pod 水平扩展时同一 job 只执行一次，**以便**避免重复消费和重复投递。
- 验收：通过压测模拟 3 Pod 并发，同一 job_id 在同一触发周期内只有 1 条执行记录

### US-3：Job 暂停 / 恢复
**作为**租户用户，**我希望**可以暂停和恢复某个 job，**以便**临时停止执行而不删除配置。
- 验收：pause 后下一个 cron 周期不触发；resume 后下一个 cron 周期恢复触发

### US-4：执行历史查询
**作为**租户用户，**我希望**查看某个 job 的最近 N 次执行记录（时间、结果、输出摘要），**以便**排查异常。
- 验收：API 返回 `last_run_at`、`run_count`、`last_run_success`，详细日志可选

---

## 4. 功能范围

### In Scope
- SaaS 模式下基于 gocron v2 的分布式调度器（`internal/scheduler/`，新包）
- 分布式锁：gocron v2 `DistributedLocker` + Redis（`go-co-op/gocron-redis-lock`）
- 从 PG `cron_jobs` 表加载 job，定期同步（30s 轮询）
- 与 `agent/factory.go` 的 `RunConversation` 集成执行
- Job CRUD 变更事件通知调度器热更新（via Redis pub/sub 或 polling）
- DB schema 补充：`last_run_success`、`last_run_error` 字段（migration）
- API：Job CRUD（已有）+ 执行历史端点（新增）
- `saas.go` 启动时初始化分布式调度器

### Out of Scope
- 重写现有单机 `internal/cron/`（保留，CLI 模式继续使用）
- Workflow 步骤类调度（属于 workflow feature）
- UI 前端（无 UI 变更）
- 跨租户 job 依赖
- Job 优先级队列 / 权重

---

## 5. 技术方案概述

### 核心选型：gocron v2 + Redis Locker

```
SaaS Pod A                    SaaS Pod B
┌────────────────┐            ┌────────────────┐
│ SaasScheduler  │            │ SaasScheduler  │
│  (gocron v2)   │            │  (gocron v2)   │
│  ┌──────────┐  │  Redis     │  ┌──────────┐  │
│  │Redis Lock│◄─┼────────────┼─►│Redis Lock│  │
│  └──────────┘  │  Pub/Sub   │  └──────────┘  │
│  ┌──────────┐  │            │  ┌──────────┐  │
│  │ PG Poll  │◄─┼────────────┼──│ PG Poll  │  │
│  └──────────┘  │            │  └──────────┘  │
└────────┬───────┘            └───────┬────────┘
         │ execute                    │ skipped (lock held)
         ▼                            ▼
  agent/factory.go              (no-op)
  RunConversation(ctx,
    tenantID, prompt)
```

**为什么选 gocron v2 而非直接用 `pg_try_advisory_lock` + goroutine：**
- gocron v2 提供成熟的 cron 表达式解析（兼容 `robfig/cron` 语法）
- 内置 `DistributedLocker` 接口，Redis 实现开箱即用
- `WithStartImmediately` / `WithSingletonMode` 等并发控制原生支持
- 同一代码库仍可通过切换 Locker 实现来支持不同部署模式

### 新包结构
```
internal/scheduler/
  saas_scheduler.go    # SaasScheduler 结构体，gocron v2 封装
  sync.go              # PG → gocron 同步逻辑（加载/更新/删除）
  executor.go          # job 执行：调用 agent/factory.go RunConversation
  scheduler_test.go
```

### DB Migration（新增字段）
```sql
ALTER TABLE cron_jobs
  ADD COLUMN IF NOT EXISTS last_run_success boolean,
  ADD COLUMN IF NOT EXISTS last_run_error   text;
```

---

## 6. 风险与依赖

| 风险 | 影响 | 缓解 |
|------|------|------|
| Redis 抖动导致锁误释放 | job 重复执行 | gocron-redis-lock 默认 TTL > job 最大执行时间；`WithSingletonMode` 防并发 |
| PG 轮询 30s 延迟 | 新建 job 最多延迟 30s 生效 | 补充 Redis pub/sub 通知（可选优化） |
| agent 执行超时 | job 阻塞调度线程 | 每个 job 独立 goroutine，设置执行超时（默认 5min）|
| SaaS 模式下 deliver 渠道不可用 | 输出丢失 | 执行结果写入 `cron_job_runs` 表作为 fallback |
| `internal/cron/` 与新 `internal/scheduler/` 接口不统一 | 维护复杂 | 定义共同 `Scheduler` interface，两者都实现 |

### 关键依赖
- `go-co-op/gocron/v2` — 尚未在 go.mod 中（需新增）
- `go-co-op/gocron-redis-lock` — 尚未在 go.mod 中（需新增）
- Redis 已在 SaaS 模式下可用

---

## 7. 待确认项（已收口）

| # | 问题 | 决策 |
|---|------|------|
| Q1 | agent 执行超时上限？ | **5min**，Redis lock TTL 设为 6min |
| Q2 | Discord deliver 路由？ | **暂不支持**，本期 deliver 仅支持 `local`（写 DB） |
| Q3 | 执行历史存储方式？ | **新增 `cron_job_runs` 表**，记录每次执行记录 |
| Q4 | 调度去重策略？ | **所有 Pod 跑 scheduler + Redis locker 去重**，无 leader election |
| Q5 | `tools/cronjob.go` SaaS 适配？ | **改写**，SaaS 模式下写 PG，CLI 模式保留文件系统 |

---

## 8. 已知假设（Karpathy Guidelines）

1. **最小可行范围**：先实现调度器 + 分布式锁 + 执行，deliver 渠道先支持 `local`（写入 DB）
2. **非目标**：不重写 CLI 单机调度器，不做 UI，不做 job 依赖图
3. **成功标准**：3 Pod 下同一 job 周期内只执行 1 次，结果可查询
4. **假设 Redis 可用**：SaaS 模式下 Redis 已稳定运行（v2.3.0 已验证）
5. **假设 gocron v2 兼容 robfig/cron 表达式**：现有 job.Schedule 字段无需迁移
