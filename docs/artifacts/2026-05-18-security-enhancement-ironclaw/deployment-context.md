# Deployment Context: HermesX Security Enhancement

> **Slug:** security-enhancement-ironclaw  
> **State:** released  
> **Date:** 2026-05-18  
> **Owner:** devops-engineer  
> **Status:** draft

---

## 环境清单

| 环境 | 用途 | 部署目标 |
|------|------|----------|
| dev | 开发验证 | single binary (local / Docker) |
| staging | 集成测试 + 性能验证 | Kubernetes single pod |
| production | 线上服务 | Kubernetes multi-replica |

---

## 部署入口

| 入口 | 说明 |
|------|------|
| 主入口 | `go build -o hermesx ./cmd/server` → 替换现有 binary |
| 镜像构建 | Dockerfile multi-stage build (现有流程不变) |
| 回退入口 | 回滚至上一版本 binary / image tag |
| 前置条件 | DB migrations 执行 (safety_policies + egress_rules 表) |

---

## 配置与密钥

### 环境变量 (新增)

| 变量 | 用途 | 默认值 | 来源 |
|------|------|--------|------|
| `HERMESX_SAFETY_MODE` | 全局安全策略默认模式 | `log_only` | ConfigMap |
| `HERMESX_EGRESS_DEFAULT_POLICY` | 出口默认策略 | `deny_all` | ConfigMap |

### 配置项

| 项目 | 说明 |
|------|------|
| Per-tenant safety policy | DB: `safety_policies` 表, 通过 Admin API 管理 |
| Per-tenant egress rules | DB: `egress_rules` 表, 通过 Admin API 管理 |
| Leak scanner patterns | 50+ builtin + DB 动态加载 (PatternWatcher) |
| Secret resolver | 读取进程环境变量, 不引入新外部依赖 |

### 密钥

| 密钥 | 来源 | 访问方式 |
|------|------|----------|
| PostgreSQL connection | 现有 `DATABASE_URL` | 不变 |
| 无新增外部密钥 | — | — |

---

## 运行保障

### Feature Flag

| Flag | 功能 | 初始值 |
|------|------|--------|
| safety_policy.mode (per-tenant) | enforce/log_only/disabled | log_only (新租户) |
| egress_default_policy (global) | allow_all/deny_all/log_only | deny_all |

### 灰度控制

- Phase 1: 所有租户 `log_only` 模式 (仅监控, 不阻断)
- Phase 2: 高风险租户切换 `enforce` (观察 72h)
- Phase 3: 全量 `enforce`

### 监控

| 指标 | 类型 | 告警阈值 |
|------|------|----------|
| `safety_input_guard_matches_total` | Counter | > 100/min (异常注入流量) |
| `safety_canary_leaks_total` | Counter | > 0 (立即告警) |
| `secrets_leak_detected_total` | Counter | > 0 (立即告警, severity=critical) |
| `egress_blocked_total` | Counter | 基线 +3σ |
| `safety_check_duration_seconds` | Histogram | P99 > 50ms |

### 告警

| 告警 | 条件 | 动作 |
|------|------|------|
| Canary leak detected | count > 0 | PagerDuty → On-call |
| Critical secret leak | severity=critical, count > 0 | PagerDuty → On-call |
| Safety check latency | P99 > 50ms 持续 5min | Slack → #ops |
| Egress deny spike | 10x 基线 | Slack → #security |

### 值守安排

- 发布日: backend-engineer + devops-engineer
- 观察窗口: 72h on-call (现有轮值)

### 观察窗口

- T+0~1h: 验证 safety interceptor 不影响请求延迟
- T+1~24h: 观察 log_only 模式下检测量和误报率
- T+24~72h: 评估是否可切换 enforce 模式

---

## 恢复能力

### 回滚触发条件

| 条件 | 动作 |
|------|------|
| P99 延迟 > 100ms 持续 10min | 回滚 binary |
| 核心 API 错误率 > 1% | 回滚 binary |
| 安全模块 panic | 回滚 binary |
| 误报导致大量正常请求被拒 | 切换 safety mode → disabled |

### 回滚路径

1. **快速缓解:** 切换 per-tenant policy → `disabled` (无需重启)
2. **代码回滚:** 部署上一版本 image (DB schema 向后兼容, 新表不影响旧代码)
3. **DB 回滚:** 不需要 — 新增表不影响旧 binary 运行

### 验证方法

- 回滚后: 确认 `/healthz` 200, 核心 API 响应时间恢复基线
- 确认 safety/egress 相关 metrics 不再上报 (证明模块未加载)
