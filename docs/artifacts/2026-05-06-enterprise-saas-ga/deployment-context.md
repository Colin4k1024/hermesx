# Deployment Context: enterprise-saas-ga v1.2.0

| 字段 | 值 |
|------|-----|
| 任务 | 2026-05-06-enterprise-saas-ga |
| 版本 | v1.2.0 (Phase 1 + Phase 2) |
| 主责 | devops-engineer |
| 状态 | released |
| 日期 | 2026-05-07 |

---

## 环境清单

| 环境 | 用途 | 入口 | 部署目标 |
|------|------|------|----------|
| dev | 开发自测 | docker-compose.dev.yml | 本地 Docker |
| test | CI 自动化测试 | docker-compose.test.yml | GitHub Actions |
| saas-staging | SaaS 预发 | docker-compose.saas.yml | K8s (kind/staging) |
| saas-production | SaaS 生产 | Helm chart (deploy/helm/hermes-agent) | K8s (production) |

---

## 部署入口

### 主入口 (Helm)

```bash
helm upgrade --install hermes-saas deploy/helm/hermes-agent/ \
  -f deploy/helm/hermes-agent/values.yaml \
  -f values.production.yaml \
  --set image.tag=v1.2.0 \
  --namespace hermes --create-namespace
```

### Docker Compose (Staging)

```bash
docker compose -f docker-compose.saas.yml up -d
```

### 回退入口

```bash
# Helm rollback
helm rollback hermes-saas 1 --namespace hermes

# Docker Compose
docker compose -f docker-compose.saas.yml down
git checkout v1.1.0 -- docker-compose.saas.yml
docker compose -f docker-compose.saas.yml up -d
```

---

## 配置与密钥

| 配置项 | 来源 | 说明 |
|--------|------|------|
| DATABASE_URL | K8s Secret / .env | PostgreSQL 连接串 |
| HERMES_ACP_TOKEN | K8s Secret / .env | Admin bootstrap token (>=32 chars) |
| HERMES_API_KEY | K8s Secret / .env | Application API key (>=32 chars) |
| REDIS_URL | K8s Secret / .env | Redis 连接串 (DualLimiter 依赖) |
| MINIO_ENDPOINT | K8s ConfigMap / .env | MinIO endpoint |
| MINIO_ACCESS_KEY | K8s Secret / .env | MinIO access key |
| MINIO_SECRET_KEY | K8s Secret / .env | MinIO secret key |
| SAAS_ALLOWED_ORIGINS | K8s ConfigMap / .env | CORS 白名单 (必须显式配置) |
| LLM_API_URL | K8s Secret / .env | LLM provider base URL |
| LLM_API_KEY | K8s Secret / .env | LLM API key |
| LLM_MODEL | K8s ConfigMap / .env | 默认模型 |

### Phase 2 新增配置

| 配置项 | 来源 | 说明 |
|--------|------|------|
| OIDC_ISSUER_URL | K8s ConfigMap | OIDC IdP issuer (Phase 2 代码就绪, wiring 需运维配置) |
| OIDC_CLIENT_ID | K8s Secret | OIDC audience/client ID |
| OIDC_CLAIM_MAPPING | K8s ConfigMap | JSON claim path 映射 (可选) |

> Note: OIDC extractor 代码已交付但未 wire 到 server.go auth chain。生产启用需运维显式配置上述变量并修改启动参数。

---

## 运行保障

### 高可用

| 机制 | 配置 | 说明 |
|------|------|------|
| PDB | minAvailable: 1 | 滚动更新期间至少保留 1 Pod |
| HPA | 2-10 replicas, CPU 70% / Memory 80% | 自动弹缩 |
| Scale-down stabilization | 300s | 防止频繁缩容 |
| Liveness probe | /health/live, 5s init, 10s period | 探活 |
| Readiness probe | /health/ready, 5s init, 10s period | 就绪检查 |

### 限流 (Phase 2 新增)

| 层级 | 机制 | 降级策略 |
|------|------|----------|
| Tenant | Redis ZSET sliding window (Lua atomic) | Redis 故障自动 fallback 到 LocalDualLimiter |
| User | Redis ZSET sliding window (同 Lua 脚本) | 同上 |
| Anonymous | 本地 LRU sliding window | 无外部依赖 |

### 监控

| 指标 | 类型 | 说明 |
|------|------|------|
| hermes_rate_limit_rejected_total | Counter (tenant_id) | 限流拒绝计数 |
| hermes_llm_request_duration_seconds | Histogram | LLM 调用延迟 |
| hermes_llm_tokens_total | Counter | Token 消耗 |
| hermes_metering_cost_total | Counter | 计费金额 |

### 告警建议

| 条件 | 严重度 | 响应 |
|------|--------|------|
| rate_limit_rejected_total > 100/min | WARN | 检查是否正常流量还是攻击 |
| Redis 连接失败 | WARN | 限流降级到本地，检查 Redis 集群 |
| LLM p99 latency > 30s | WARN | 检查 provider 状态，触发断路器 |
| Pod restarts > 3 in 5min | CRITICAL | 检查 OOM/panic，考虑回滚 |

---

## 恢复能力

### 回滚触发条件

- Pod CrashLoopBackOff 持续 > 3 min
- 错误率 > 5% 持续 2 min
- 限流拒绝率异常飙升 (非正常流量模式)
- RLS 写入报错 (SET LOCAL 路径异常)

### 回滚路径

1. `helm rollback hermes-saas <revision> --namespace hermes`
2. 验证 `/health/ready` 恢复 200
3. 验证 `hermes_rate_limit_rejected_total` 恢复正常
4. 通知 on-call 团队

### 数据库回滚

- Migration 65-70 均为 additive (ADD COLUMN IF NOT EXISTS, CREATE POLICY IF NOT EXISTS)
- 无需回滚 migration；应用回滚到旧版本时 RLS policy 不生效但不阻塞读写
- 若必须回滚 migration: 使用 pitr-drill.sh 恢复到指定时间点

---

## 观察窗口

| 阶段 | 时长 | 关注指标 |
|------|------|----------|
| 金丝雀 (10% 流量) | 30 min | 错误率、延迟 p99、Pod 健康 |
| 灰度 (50% 流量) | 1 hour | 限流命中率、Redis Lua 延迟、PricingStore 缓存命中 |
| 全量 | 2 hours | HPA 弹缩行为、GDPR 删除性能、审计写入完整性 |
| 稳定运行 | 24 hours | 整体错误率、资源利用率趋势 |

---

## Phase 2 特殊注意事项

1. **DualLayerLimiter Redis 依赖**: 首次部署确保 Redis 可达；若 Redis 未配置，系统自动使用 LocalDualLimiter (单副本精确，多副本按比例放大)
2. **PricingStore 缓存预热**: 首次请求会触发 DB 查询并缓存 30s；若 pricing_rules 表为空则使用硬编码 fallback
3. **OIDC 未激活**: 代码已就绪但未 wire，不影响现有 JWT auth chain
4. **store.ErrNotFound**: 新增 sentinel error 对现有调用无影响（仅 pricing handler 消费）
