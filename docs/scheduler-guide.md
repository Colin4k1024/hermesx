# 分布式调度部署与测试指南

> 如何部署 Cron Scheduler 所需的中间件、启动调度服务、以及验证分布式执行正确性。

## 前置条件

分布式 Cron Scheduler 依赖以下中间件：

| 组件 | 版本要求 | 用途 |
|------|----------|------|
| PostgreSQL | 16+ | 存储 cron_jobs、cron_job_runs 表，RLS 租户隔离 |
| Redis | 7+ | 分布式锁（多 Pod 互斥执行） |

## 中间件部署

### Docker Compose（推荐开发环境）

使用 `docker-compose.saas.yml` 一键启动完整基础设施：

```bash
# 创建 .env 文件
cat > .env <<'EOF'
POSTGRES_DB=hermesx
POSTGRES_USER=hermesx
POSTGRES_PASSWORD=hermesx
REDIS_URL=redis://redis:6379
MINIO_ACCESS_KEY=hermesx
MINIO_SECRET_KEY=hermesxpass
MINIO_BUCKET=hermes-skills
SAAS_ALLOWED_ORIGINS=*
HERMES_ACP_TOKEN=my-admin-token
HERMES_API_KEY=my-api-key
LLM_API_URL=http://host.docker.internal:11434/v1
LLM_API_KEY=ollama
LLM_MODEL=qwen3:30b
EOF

# 启动所有服务
docker compose -f docker-compose.saas.yml up -d
```

服务启动后，Scheduler 所需的 PostgreSQL 和 Redis 自动可用：

```
PostgreSQL  → localhost:5432
Redis       → localhost:6379
SaaS API    → localhost:18080
```

### 仅启动中间件

如需单独运行中间件（本地开发调试）：

```bash
docker compose -f docker-compose.saas.yml up -d postgres redis
```

验证中间件就绪：

```bash
# PostgreSQL
docker exec hermesx-pg pg_isready -U hermesx

# Redis
docker exec hermesx-redis redis-cli ping
```

### Kubernetes / Helm

使用 Helm Chart 部署到集群：

```bash
cd deploy/helm/hermesx

# 安装（自动创建 PG + Redis 依赖）
helm install hermesx . \
  --set env.DATABASE_URL="postgres://hermesx:password@pg-host:5432/hermesx?sslmode=require" \
  --set env.REDIS_URL="redis://redis-host:6379"
```

Helm `values.yaml` 中 Scheduler 相关默认值：

```yaml
env:
  REDIS_URL: "redis://redis:6379"
  # SCHEDULER_POLL_INTERVAL: "30s"   # 可选覆盖
  # SCHEDULER_EXEC_TIMEOUT: "5m"     # 可选覆盖
  # SCHEDULER_LOCK_TTL: "12m"        # 可选覆盖
```

### 数据库表自动迁移

Scheduler 依赖的表在 `hermes saas-api` 启动时自动创建（migration 100-106）：

- `cron_jobs` — 定时任务定义
- `cron_job_runs` — 执行记录（含幂等唯一约束）
- RLS 读写策略（SELECT / INSERT / UPDATE / DELETE）
- `scheduler_cleanup_stale_runs()` SECURITY DEFINER 函数

无需手动执行 DDL。

## 启动与配置

### 环境变量

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `REDIS_URL` | 是（启用调度） | - | Redis 连接字符串 |
| `DATABASE_URL` | 是 | - | PostgreSQL 连接字符串 |
| `SCHEDULER_POLL_INTERVAL` | 否 | `30s` | 从 PG 同步任务的轮询间隔 |
| `SCHEDULER_EXEC_TIMEOUT` | 否 | `5m` | 单次任务执行超时 |
| `SCHEDULER_LOCK_TTL` | 否 | `12m` | Redis 分布式锁 TTL |

**关键约束**：`SCHEDULER_LOCK_TTL` 必须大于最长任务的执行时间，否则锁提前释放可能导致重复执行。

### 启动流程

```bash
# 方式 1：二进制直接启动
export DATABASE_URL="postgres://hermesx:hermesx@localhost:5432/hermesx?sslmode=disable"
export REDIS_URL="redis://localhost:6379"
export HERMES_ACP_TOKEN="my-admin-token"
./hermesx saas-api

# 方式 2：Docker Compose
docker compose -f docker-compose.saas.yml up -d
```

启动日志中应看到：

```
level=INFO msg="scheduler started" poll_interval=30s
```

如果 Redis 不可用，日志会输出警告但服务仍正常启动（Scheduler 功能不可用）：

```
level=WARN msg="scheduler disabled: redis unavailable"
```

### 健康检查确认

```bash
# 就绪探针（包含 Redis 连通性检查）
curl -s http://localhost:18080/health/ready | jq .

# 期望输出
{
  "status": "ok",
  "checks": {
    "database": "ok",
    "redis": "ok"
  }
}
```

如果 `redis` 状态为 `"error"`，Scheduler 未能启动。

### 多副本部署

Scheduler 天然支持多 Pod 部署，无需额外配置：

```bash
# 使用 multi-replica compose 验证（3 副本 + Nginx LB）
cd deploy
docker compose -f docker-compose.multi-replica.yml up -d
```

每个 Pod 独立运行 pollLoop，竞争 Redis 锁：
- 获得锁的 Pod 执行任务
- 未获得锁的 Pod 跳过（`redislock.WithTries(1)` 无重试）
- 幂等约束 `UNIQUE(cron_job_id, scheduled_at)` 兜底防重复

## 测试

### 单元测试

```bash
# 运行 scheduler 包测试
go test ./internal/scheduler/ -v -count=1

# 期望输出
=== RUN   TestSchedulerNew
=== RUN   TestSchedulerSync
...
--- PASS: ...
PASS
ok      github.com/hermesx/internal/scheduler   0.XXXs
```

测试使用内存 mock（`mockCronJobStore`、`mockAgentRunner`），无需外部依赖。

### 集成测试

集成测试需要真实的 PostgreSQL 和 Redis：

```bash
# 方式 1：一键运行（启动基础设施 → 测试 → 清理）
make test-integration

# 方式 2：分步操作
make test-infra-up                                    # 启动 PG:5433 + Redis:6380 + MinIO:9002
go test -tags=integration ./tests/integration/...     # 运行测试
make test-infra-down                                  # 清理
```

`docker-compose.test.yml` 使用隔离端口和 tmpfs 卷，不影响开发环境：

| 服务 | 测试端口 |
|------|----------|
| PostgreSQL | 5433 |
| Redis | 6380 |
| MinIO | 9002 |

### 手动功能验证

通过 Agent 的 `cronjob` 工具创建定时任务并观察执行：

```bash
# 1. 启动服务
docker compose -f docker-compose.saas.yml up -d

# 2. 通过 Chat API 创建一个每分钟执行的测试任务
curl -X POST http://localhost:18080/v1/chat/completions \
  -H "Authorization: Bearer my-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3:30b",
    "messages": [{"role": "user", "content": "帮我创建一个每分钟执行的定时任务，内容是：回复当前时间"}]
  }'

# 3. 等待 1-2 分钟，检查执行记录
docker exec hermesx-pg psql -U hermesx -c \
  "SELECT id, status, scheduled_at, duration_ms FROM cron_job_runs ORDER BY created_at DESC LIMIT 5;"

# 4. 检查 Scheduler 日志
docker logs hermesx-saas 2>&1 | grep -i "cron\|scheduler" | tail -10
```

### 多 Pod 互斥验证

验证分布式锁正确工作：

```bash
# 1. 启动 3 副本
cd deploy
docker compose -f docker-compose.multi-replica.yml up -d

# 2. 创建一个短间隔任务（通过任一副本）
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer test-secret-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3:30b",
    "messages": [{"role": "user", "content": "创建定时任务：每分钟报告一次当前时间"}]
  }'

# 3. 观察各 Pod 日志，确认只有一个 Pod 执行
docker logs hermesx-mr-1 2>&1 | grep "cron job executed" | wc -l
docker logs hermesx-mr-2 2>&1 | grep "cron job executed" | wc -l
docker logs hermesx-mr-3 2>&1 | grep "cron job executed" | wc -l

# 4. 确认 cron_job_runs 无重复记录
docker exec hermesx-mr-pg psql -U hermesx -c \
  "SELECT cron_job_id, scheduled_at, COUNT(*) FROM cron_job_runs GROUP BY 1,2 HAVING COUNT(*) > 1;"
# 期望：0 行（无重复）

# 5. 清理
docker compose -f docker-compose.multi-replica.yml down -v
```

### 故障恢复验证

```bash
# 模拟 Redis 故障
docker stop hermesx-redis

# 观察服务日志（Scheduler 停止调度，但 API 仍可用）
curl -s http://localhost:18080/health/ready | jq .
# redis: "error", 但 HTTP 200（服务不退出）

# 恢复 Redis
docker start hermesx-redis

# Scheduler 在下一个 poll 周期（≤30s）自动恢复
docker logs hermesx-saas 2>&1 | grep "scheduler" | tail -5
```

## 运维要点

| 场景 | 处理方式 |
|------|----------|
| Redis 临时不可用 | Scheduler 暂停，API 正常，Redis 恢复后自动恢复 |
| Pod 异常退出 | 其他 Pod 通过锁 TTL 超时后自动接管；`scheduler_cleanup_stale_runs()` 标记超时记录为 failed |
| 任务堆积 | 调大 `SCHEDULER_EXEC_TIMEOUT`，或拆分为更小粒度的 prompt |
| 锁 TTL 过短 | 调大 `SCHEDULER_LOCK_TTL`（必须大于最长任务时长） |
| PG 与 gocron 不同步 | 等待下一个 poll 周期（30s）自动对齐 |

## 相关文档

- [架构概览](architecture.md) — Scheduler 系统设计
- [配置指南](configuration.md) — 完整环境变量参考
- [数据库](database.md) — cron_jobs / cron_job_runs 表结构
- [部署指南](deployment.md) — Docker / Helm / Kind 部署
