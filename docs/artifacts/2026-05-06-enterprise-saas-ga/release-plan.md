# Release Plan: enterprise-saas-ga v1.2.0

| 字段 | 值 |
|------|-----|
| 版本 | v1.2.0 |
| 发布范围 | Phase 1 (Security Hardening) + Phase 2 (OIDC + Pricing + Rate Limiting) |
| 发布角色 | devops-engineer |
| 放行来源 | QA GO verdict (launch-acceptance.md) |
| 发布日期 | 2026-05-07 |
| 值守人 | devops-engineer + backend-engineer |
| 观察窗口 | 24 hours post-deploy |

---

## 发布信息

### 变更概要

| Phase | 内容 | 风险等级 |
|-------|------|----------|
| Phase 1 | RLS WITH CHECK 写策略, 审计日志不可变, GDPR MinIO 清理, PDB/HPA, Session owner check, CORS 修复, credential 治理, IDOR 修复 | HIGH (已修复所有 CRITICAL/HIGH) |
| Phase 2 | OIDCExtractor, DualLayerLimiter (Redis Lua), Dynamic PricingStore, Admin Pricing API, store.ErrNotFound | MEDIUM (代码就绪, OIDC 未激活) |

### Release Notes

```
## v1.2.0 — Enterprise SaaS GA

### Security Hardening (Phase 1)
- Multi-tenant RLS write policies with SET LOCAL transaction wrapper
- Immutable audit logs (REVOKE DELETE + SECURITY DEFINER purge function)
- GDPR right-to-erasure: MinIO object cleanup with 207 Multi-Status
- Session ownership verification on message access
- IDOR fix: removed X-Hermes-User-Id header fallback in memory API
- CORS: removed wildcard default, requires explicit origin configuration
- Credential hygiene: .env.example with CHANGEME pattern

### Platform Features (Phase 2)
- OIDC ID token extractor with JWKS rotation and configurable claim mapping
- Atomic dual-layer rate limiting (tenant + user) via Redis Lua script
- Automatic fallback to local limiter on Redis failure
- Dynamic pricing rules: Admin CRUD API with DB-first cost calculation
- 30s TTL pricing cache with background refresh

### Infrastructure
- PodDisruptionBudget (minAvailable: 1)
- HorizontalPodAutoscaler (2-10 replicas, CPU/Memory targets)
- Scale-down stabilization window (300s)
- Health probes: /health/live, /health/ready
```

---

## 风险与缓解

| 风险 | 等级 | 缓解措施 |
|------|------|----------|
| RLS SET LOCAL 性能开销 | LOW | 每事务 1 次 SET LOCAL, 无索引影响 |
| Redis Lua 脚本延迟 | LOW | 单 EVALSHA < 1ms; 故障自动 fallback |
| GDPR 大批量 MinIO 删除 | MEDIUM | best-effort 全量删除 + 207 报告; 后台执行 |
| LocalDualLimiter 多副本倍增 | MEDIUM | 文档化已知限制; 仅 Redis 故障时触发 |
| OIDC wiring 配置错误 | LOW | 未激活, 不影响现有 auth chain |
| HPA 弹缩抖动 | LOW | scaleDown stabilization 300s |

---

## 前置条件检查

| # | 检查项 | 状态 | 说明 |
|---|--------|------|------|
| 1 | QA 放行结论 | ✅ | Phase 1 + Phase 2 均 GO |
| 2 | go build 编译通过 | ✅ | clean |
| 3 | go vet 无警告 | ✅ | clean |
| 4 | 全量测试通过 | ✅ | 1469/1469 pass |
| 5 | Race detector 无竞态 | ✅ | -race clean |
| 6 | 0 CRITICAL / 0 HIGH 未修复 | ✅ | all resolved |
| 7 | Docker image 构建成功 | ⏳ | 发布时验证 |
| 8 | Helm template 渲染正确 | ⏳ | 发布时验证 |
| 9 | 密钥配置就绪 | ⏳ | 运维确认 |
| 10 | Redis 集群可达 | ⏳ | 运维确认 |

---

## 执行步骤

### Step 1: Build & Push Image

```bash
# Build
docker build -t hermes-agent/hermes-saas:v1.2.0 .

# Tag & push
docker tag hermes-agent/hermes-saas:v1.2.0 registry.example.com/hermes-agent/hermes-saas:v1.2.0
docker push registry.example.com/hermes-agent/hermes-saas:v1.2.0
```

### Step 2: Helm Template Validation

```bash
helm template hermes-saas deploy/helm/hermes-agent/ \
  -f deploy/helm/hermes-agent/values.yaml \
  --set image.tag=v1.2.0 \
  --namespace hermes | kubectl apply --dry-run=client -f -
```

### Step 3: Database Migration (Automatic)

- Migrations 65-70 在应用启动时自动执行 (embedded migrate)
- 所有 migration 使用 `IF NOT EXISTS` / `DO $$ IF EXISTS` 幂等保护
- 无破坏性 schema 变更

### Step 4: Deploy (Canary 10%)

```bash
helm upgrade --install hermes-saas deploy/helm/hermes-agent/ \
  -f values.production.yaml \
  --set image.tag=v1.2.0 \
  --set replicaCount=1 \
  --namespace hermes
```

**暂停点 — 30 min 观察:**
- `/health/ready` 返回 200
- 错误率 < 0.1%
- `hermes_rate_limit_rejected_total` 无异常飙升
- Pod 无 restart

### Step 5: Scale to 50%

```bash
helm upgrade hermes-saas deploy/helm/hermes-agent/ \
  -f values.production.yaml \
  --set image.tag=v1.2.0 \
  --set replicaCount=2 \
  --namespace hermes
```

**暂停点 — 1 hour 观察:**
- Redis Lua 执行延迟 p99 < 5ms
- PricingStore 缓存命中率 > 95%
- DualLimiter denied 计数符合预期

### Step 6: Full Rollout

```bash
helm upgrade hermes-saas deploy/helm/hermes-agent/ \
  -f values.production.yaml \
  --set image.tag=v1.2.0 \
  --namespace hermes
```

HPA 接管后 minReplicas=2, maxReplicas=10。

---

## 验证与监控

### 发布后即时验证 (T+5min)

| 检查项 | 命令 / 方式 | 预期 |
|--------|-------------|------|
| Pod 健康 | `kubectl get pods -n hermes` | All Running/Ready |
| Health endpoint | `curl /health/ready` | 200 |
| RLS 写入 | 创建 session → 成功 | 非 superuser 写入通过 |
| Rate limit header | 任意 API 调用 | X-RateLimit-Remaining 头存在 |
| Pricing API | `GET /admin/v1/pricing-rules` | 200 (空列表或已有规则) |
| Audit log | 执行写操作 → 查 audit_logs | 记录存在且 tenant_id 正确 |

### 持续监控 (T+24h)

| 指标 | 阈值 | 动作 |
|------|------|------|
| Error rate | > 1% 持续 5min | 告警 + 评估回滚 |
| Pod restarts | > 3/5min | 告警 + 立即回滚 |
| Redis connection errors | > 10/min | 告警 + 检查 Redis 集群 |
| p99 latency | > 10s | 告警 + 检查 LLM provider |
| Memory usage | > 80% limit | 告警 + 检查泄漏 |

---

## 回滚方案

### 触发条件

- 错误率 > 5% 持续 2 min
- Pod CrashLoopBackOff > 3 min
- RLS 写入全量失败
- 安全漏洞复现

### 执行

```bash
# 1. 回滚到上一版本
helm rollback hermes-saas 0 --namespace hermes

# 2. 验证
kubectl rollout status deployment/hermes-saas -n hermes
curl -s https://api.example.com/health/ready

# 3. 通知
# 通知 on-call 团队，记录回滚原因
```

### 数据库回滚 (极端情况)

- Migration 65-70 均为 additive，应用回滚不需要 DB 回滚
- 若需 PITR: `./deploy/pitr/pitr-drill.sh restore <timestamp>`

---

## 放行结论

**结论: GO — 允许发布**

### 放行依据

1. Phase 1 + Phase 2 QA 验收均为 GO
2. 0 CRITICAL, 0 HIGH 未修复问题
3. 1469/1469 全量测试通过, race detector clean
4. 所有 migration 幂等保护, 无破坏性变更
5. 回滚路径明确且已验证 (helm rollback)
6. 观察窗口和告警策略已定义

### 后续观察项

- Redis Lua EVALSHA 延迟 (p99 < 5ms baseline)
- PricingStore 缓存命中率 (预期 >95%)
- HPA 弹缩频率 (stabilization 300s 是否足够)
- RLS SET LOCAL 对高并发写入的性能影响
- GDPR 大批量删除的超时和重试行为

### 确认记录

- QA: GO (Phase 1 + Phase 2)
- DevOps: GO
- 发布负责人: devops-engineer
- 值守人: devops-engineer + backend-engineer
- 观察窗口: 24 hours (2026-05-07 ~ 2026-05-08)
