# Arch Design: SaaS Distributed Cron Scheduler

> **状态**: draft | **阶段**: plan | **角色**: architect  
> **日期**: 2026-05-19 | **slug**: saas-cron-scheduler

---

## 1. 系统边界

### 外部依赖
- **PostgreSQL**：cron_jobs 表（真相源）、cron_job_runs 执行历史，RLS 租户隔离
- **Redis**（Sentinel HA）：gocron-redis-lock 分布式锁，key = `cron:lock:{job_id}`
- **agent/factory.go**：RunConversation — SaaS agent 执行入口

### 边界内
- `internal/scheduler/` — 新包，SaaS 调度器全部逻辑
- `internal/tools/cronjob.go` — 改写：SaaS 模式写 PG，CLI 模式保留文件系统
- DB migration — cron_job_runs 表 + cron_jobs 字段补充

### 边界外（本期不改）
- `internal/cron/` — CLI/standalone 模式调度器，保持不变
- UI 层 — 无前端改动
- deliver 渠道（Discord 等）— 本期只支持 local（写 DB）

---

## 2. 组件拆分

```
internal/scheduler/
├── scheduler.go    SaasScheduler struct、New()、Start()、Stop()
├── sync.go         PG 轮询 + gocron job 增删改同步
├── executor.go     单 job 执行：超时控制、RunConversation、cron_job_runs 写入
└── schema.go       CronJobRun model、stale lock 清理 SQL
```

### SaasScheduler 核心结构

```go
type SaasScheduler struct {
    gocronSched  gocron.Scheduler
    store        store.CronJobStore
    agentFactory AgentRunner           // 接口，可 stub
    redisClient  redis.UniversalClient
    pollInterval time.Duration         // 默认 30s，CRON_POLL_INTERVAL
    execTimeout  time.Duration         // 默认 5min，CRON_EXEC_TIMEOUT
    lockTTL      time.Duration         // 默认 12min，CRON_LOCK_TTL
    jobs         map[string]gocron.Job // cronJobID → gocron Job 句柄
    mu           sync.Mutex
}

// AgentRunner 接口（便于测试 stub）
type AgentRunner interface {
    RunConversation(ctx context.Context, tenantID, sessionID, prompt string) (string, error)
}
```

---

## 3. 关键数据流

### 启动流程
```
saas.go
  └─ scheduler.New(cfg)
       ├─ gocron.NewScheduler(WithDistributedLocker(redisLocker))
       ├─ cleanupStaleRuns()           // 清理 status=running AND started_at < now()-lockTTL
       └─ Start(ctx)
            ├─ syncOnce(ctx)           // 立即全量同步一次
            └─ ticker(30s) → syncOnce  // 定期轮询
```

### 同步流程（syncOnce）
```
PG.ListAllEnabled()
  └─ 对比 s.jobs map（以 PG 为真相源）
       ├─ 新增：sched.NewJob(CronJob expr, execute fn, WithName(id), WithSingletonMode)
       ├─ 变更：RemoveJob + 重新 NewJob
       └─ 删除/禁用：sched.RemoveJob
```

### 执行流程（单次触发）
```
gocron 内部到达 cron 触发时间
  └─ DistributedLocker.Lock("cron:lock:{job_id}", TTL=12min)
       ├─ 失败 → skip（其他 Pod 持锁）
       └─ 成功 → executor.execute(ctx, job)
                   ├─ INSERT cron_job_runs (status=running, scheduled_at=trigger_time)
                   │     └─ ON CONFLICT (cron_job_id, scheduled_at) DO NOTHING
                   │          └─ 影响行=0 → 幂等保护，直接 return
                   ├─ context.WithTimeout(ctx, 5min)
                   ├─ AgentRunner.RunConversation(ctx, tenantID, sessionID, prompt)
                   ├─ UPDATE cron_job_runs (status, result, finished_at, duration_ms)
                   ├─ UPDATE cron_jobs (last_run_at, run_count++, last_run_success, next_run_at)
                   └─ Lock 自动过期释放（无需显式 Unlock）
```

---

## 4. 接口约定

### gocron v2 核心 API

```go
import (
    gocron "github.com/go-co-op/gocron/v2"
    redislock "github.com/go-co-op/gocron-redis-lock/v2"
)

// 初始化 locker
locker, err := redislock.NewRedisLocker(
    redisClient,
    redislock.WithTries(1),          // 不重试，失败即 skip
    redislock.WithExpiry(s.lockTTL), // 12min
)

// 初始化 scheduler
sched, err := gocron.NewScheduler(
    gocron.WithDistributedLocker(locker),
)

// 注册 job
j, err := sched.NewJob(
    gocron.CronJob(cronExpr, false), // withSeconds=false
    gocron.NewTask(s.execute, ctx, cronJob),
    gocron.WithName(cronJob.ID),
    gocron.WithSingletonMode(gocron.LimitModeReschedule), // 同 Pod 防并发
)
```

### 新增 HTTP 端点（骨架）

```
GET /api/v1/cron-jobs/{id}/runs?limit=20&offset=0
  → []CronJobRun{id, status, started_at, finished_at, duration_ms, result}
  → RLS 由 PG 层隔离（withTenantTx）
```

---

## 5. DB Schema

### cron_job_runs（新表）

```sql
CREATE TABLE cron_job_runs (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    cron_job_id    UUID        NOT NULL REFERENCES cron_jobs(id) ON DELETE CASCADE,
    tenant_id      VARCHAR(64) NOT NULL,
    status         VARCHAR(16) NOT NULL DEFAULT 'pending',
    scheduled_at   TIMESTAMPTZ NOT NULL,   -- gocron 触发时间，幂等 key 之一
    started_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at    TIMESTAMPTZ,
    duration_ms    INT,
    result         TEXT,                   -- 截断至 4096 字符
    error          TEXT,
    pod_id         VARCHAR(128),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_cron_job_runs_job_scheduled UNIQUE (cron_job_id, scheduled_at)
);

CREATE INDEX idx_cron_job_runs_job_id    ON cron_job_runs(cron_job_id);
CREATE INDEX idx_cron_job_runs_tenant    ON cron_job_runs(tenant_id, started_at DESC);

-- RLS
ALTER TABLE cron_job_runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON cron_job_runs
    USING (tenant_id = current_setting('app.tenant_id'));
```

### cron_jobs 补充字段（migration）

```sql
ALTER TABLE cron_jobs
    ADD COLUMN IF NOT EXISTS last_run_success boolean,
    ADD COLUMN IF NOT EXISTS last_run_error   text;
```

---

## 6. 技术选型

| 项目 | 选择 | 原因 |
|------|------|------|
| 调度引擎 | gocron v2 | 内置 DistributedLocker 接口；兼容 robfig/cron 表达式语法；维护活跃 |
| 分布式锁 | gocron-redis-lock v2 | gocron 官方配套；Redis SET NX + TTL 语义清晰 |
| 幂等保护 | DB UNIQUE 约束 | 锁失效后最后一道防线；无需外部依赖 |
| 真相源 | PostgreSQL | 与现有 SaaS 架构一致；ListDue 接口已存在 |
| 单 Pod 并发控制 | WithSingletonMode(LimitModeReschedule) | 防止同一 job 在同一 Pod 上并发执行 |

**未采用方案**：
- `pg_try_advisory_lock` 方案：连接池占用问题（长时间 LLM 调用锁定连接），不适合 5min+ 执行
- `SELECT FOR UPDATE SKIP LOCKED` 方案：需要改 cron_jobs 状态机，增加 stale lock reaper 复杂度；在 Pod < 10 场景下 Redis locker 更简洁

---

## 7. 风险与约束

| 风险 | 缓解 | 剩余风险 |
|------|------|----------|
| LLM 执行超 5min（锁 TTL 12min 兜不住）| DB UNIQUE 约束幂等保护 | 极端情况（>12min）double-write 被约束拦截，不影响结果正确性 |
| Redis 不可用（Sentinel 故障切换期间）| scheduler 降级：Redis 不可用时 gocron 不执行任何 job | 降级窗口内 job 漏执行；恢复后下次触发正常 |
| Pod 崩溃遗留 status=running 记录 | 启动时清理 started_at < now()-lockTTL 的 running 记录 | 无 |
| PG ListDue 全量扫描（Pod 数增长）| 当前可接受；Pod > 10 时评估 SKIP LOCKED 分片 | 中长期可扩展性风险 |
| cron job 过多（内存压力）| 单租户限制 50 个 job（API 层拒绝）| 无 |

---

## 8. 与现有代码的关系

| 现有组件 | 改动 | 说明 |
|---------|------|------|
| `internal/cron/` | **不变** | CLI/standalone 模式继续使用 |
| `internal/tools/cronjob.go` | **改写** | SaaS 模式写 PG，CLI 模式保留文件系统；通过 AgentOption 注入 store |
| `internal/store/pg/cronjobs.go` | **新增字段扫描** | Scan last_run_success/error 字段 |
| `internal/agent/factory.go` | **不变** | 通过 AgentRunner 接口调用，无直接依赖 |
| `cmd/hermesx/saas.go` | **新增初始化** | scheduler.New(cfg).Start(ctx) |
