# ADR-002: Dual-Layer Rate Limiter Interface

## 决策信息

| 字段 | 值 |
|------|-----|
| 编号 | ADR-002 |
| 标题 | 引入 DualLayerLimiter 接口替代单层 RateLimiter |
| 状态 | Accepted |
| 日期 | 2026-05-07 |
| Owner | tech-lead |
| 关联需求 | US-09 (User-Level Rate Limiting), P2-S3 |

---

## 背景与约束

### 当前问题

现有 `RateLimiter` 接口签名为：

```go
type RateLimiter interface {
    Allow(key string, limit int) (bool, int, error)
}
```

该接口只支持单 key 单 limit 的检查。US-09 要求同时检查 tenant 级和 user/key 级两层限流，且两层检查必须原子执行——否则会出现 tenant 配额耗尽但 user 配额显示充裕的不一致状态。

### 业务目标

- 企业租户需要保障公平使用：单个用户不能耗尽整个租户的 API 配额
- 两层限流必须原子判定：一个 Lua 脚本检查两个 key
- 返回两层剩余量，HTTP 响应头展示 `min(tenant_remaining, user_remaining)`

### 约束条件

- 现有 `RateLimiter` 接口已有 3 个实现（Redis、local fallback、test mock）
- 现有 `RateLimitMiddleware` 依赖 `Allow(key, limit)` 签名
- Redis Cluster 模式下多 key 操作需 hash tag 保证同 slot
- Local fallback 在 Redis 不可用时必须提供降级

### 非目标

- 不改变 tenant 级限流的语义（保持 ZSET sliding window）
- 不支持 burst 模式（延后到 v1.3.0）
- 不引入外部限流服务（如 envoy ratelimit gRPC）

---

## 备选方案

### 方案 A：两次 Allow 调用（非原子）

调用两次 `Allow()`：先检查 tenant，再检查 user。

- **适用条件**：能接受非原子性的场景
- **优点**：零接口变更，实现最简
- **风险**：race condition — tenant allow + user deny 会消耗 tenant 配额但拒绝请求；tenant deny + user allow 无法正确递减 user 计数
- **不选原因**：在高并发下 rate limit 不准确，违背 US-09 验收标准

### 方案 B：修改现有 Allow 签名为多 key

```go
Allow(keys []string, limits []int) (bool, []int, error)
```

- **适用条件**：愿意承受所有调用方的 breaking change
- **优点**：单一接口，无接口碎片
- **风险**：所有现有调用方必须适配；local fallback 复杂度上升；单 key 场景变得啰嗦
- **不选原因**：破坏性太大，且单层调用方（匿名流量）不需要多 key 语义

### 方案 C：新建 DualLayerLimiter 接口（采用）

```go
type DualLayerLimiter interface {
    AllowDual(tenantKey string, tenantLimit int, userKey string, userLimit int) (allowed bool, tenantRemaining int, userRemaining int, err error)
}
```

- **适用条件**：需要原子双层检查的认证请求
- **优点**：不破坏现有接口；新旧可并存；Lua 脚本封装原子逻辑
- **风险**：两个接口并存增加认知负担；需要确保 middleware 在有/无 DualLayerLimiter 时都能正常工作
- **选中原因**：向后兼容 + 原子性 + 清晰语义

---

## 决策结果

**采用方案 C：新建 `DualLayerLimiter` 接口。**

### 具体设计

```go
// DualLayerLimiter checks tenant and user/key rate limits atomically.
type DualLayerLimiter interface {
    AllowDual(tenantKey string, tenantLimit int, userKey string, userLimit int) (allowed bool, tenantRemaining int, userRemaining int, err error)
}
```

### 实现策略

1. **Redis 实现** (`RedisDualLimiter`)：单 Lua 脚本原子检查两个 ZSET key
   - Key 格式：`rl:{tenantID}` + `rl:{tenantID}:user:{userID}` 或 `rl:{tenantID}:key:{keyID}`
   - Hash tag `{tenantID}` 保证 Redis Cluster 同 slot
   - 返回 `[allowed(0/1), tenant_remaining, user_remaining]`

2. **Local fallback** (`LocalDualLimiter`)：两个独立的 LRU 滑动窗口
   - 非严格原子（进程内无 race），但提供尽力而为的降级

3. **Middleware 兼容**：
   - 认证请求：若 `DualLayerLimiter` 可用 → 使用 `AllowDual`
   - 匿名请求 / 无 UserID：退回到旧 `RateLimiter.Allow` 单层检查
   - `X-RateLimit-Remaining` = `min(tenantRemaining, userRemaining)`

### 迁移路径

```
Phase 2-S3:
  1. 新增 DualLayerLimiter 接口 + Redis 实现
  2. 新增 LocalDualLimiter fallback
  3. RateLimitMiddleware 增加 DualLayerLimiter 可选字段
  4. 认证请求走 AllowDual，匿名请求走 Allow
  5. 旧 RateLimiter 接口不删除、不修改
```

### 影响范围

- `internal/middleware/ratelimit.go` — 新增 DualLayerLimiter 路径
- `internal/middleware/redis_ratelimiter.go` — 新增 RedisDualLimiter
- `internal/middleware/ratelimit_test.go` — 新增双层测试
- Prometheus metric：新增 `hermes_rate_limit_rejected_total{tenant_id, layer}` label

### 兼容性

- 所有现有 `RateLimiter.Allow` 调用方不受影响
- 匿名请求行为不变
- 新 middleware 配置为 optional——不设 DualLayerLimiter 则退回单层

### 失败 / 回退思路

- 若 Lua 脚本在 Redis Cluster 模式下不工作：回退到方案 A（两次调用 + 文档说明非原子性）
- 若 local fallback 精度不足：接受为"尽力而为"，不阻塞发布

---

## 后续动作

| 动作 | Owner | 完成条件 |
|------|-------|---------|
| 实现 DualLayerLimiter + Redis Lua | backend-engineer | P2-S3 完成 |
| 更新 RateLimitMiddleware | backend-engineer | 认证请求走双层路径 |
| 表驱动测试覆盖 | backend-engineer | ≥8 个 scenario |
| Redis Cluster hash tag 验证 | backend-engineer | CI 中 Redis Cluster 模式测试 |
