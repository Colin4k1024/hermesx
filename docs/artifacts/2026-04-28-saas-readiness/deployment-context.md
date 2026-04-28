# Deployment Context: SaaS Readiness P0-P5

| 字段 | 值 |
|------|-----|
| Slug | `saas-readiness` |
| 日期 | 2026-04-28 |
| 主责 | devops-engineer |
| 状态 | draft |
| 阶段 | released |

---

## 环境清单

| 环境 | 用途 | 访问入口 | 部署目标 |
|------|------|---------|---------|
| local | 开发调试 | localhost:3000 | 直接 `go run` / SQLite |
| staging | 集成验证 | staging.hermes.internal:3000 | Kubernetes (Helm) + PostgreSQL |
| production | 线上服务 | hermes.internal:3000 | Kubernetes (Helm) + PostgreSQL |

---

## 部署入口

| 入口 | 方式 | 前置条件 |
|------|------|---------|
| 主入口 | `helm upgrade --install hermes-agent deploy/helm/hermes-agent/ -f values-{env}.yaml` | K8s context 切换到目标集群 |
| 手工入口 | `kubectl apply -f deploy/helm/hermes-agent/templates/` | 仅 emergency，需 values 手动渲染 |
| 回退入口 | `helm rollback hermes-agent {revision}` | 保留至少 3 个 revision history |
| 本地入口 | `go build -o hermes-agent ./cmd/hermes-agent && ./hermes-agent` | SQLite 模式，无需外部依赖 |

---

## 配置与密钥

### 环境变量

| 变量 | 用途 | 来源 | 必填 |
|------|------|------|------|
| `DATABASE_URL` | PostgreSQL 连接串 | K8s Secret / env | 是（PG 模式） |
| `HERMES_ACP_TOKEN` | ACP server 认证 token | K8s Secret / env | **是（CRIT-1 修复后强制）** |
| `REDIS_URL` | Redis 连接（分布式限流） | K8s Secret / env | 否（降级到 local limiter） |
| `TLS_ENABLED` | 启用 TLS | ConfigMap / env | 否（默认 false） |
| `TLS_CERT_FILE` | TLS 证书路径 | Volume mount | TLS 启用时必填 |
| `TLS_KEY_FILE` | TLS 私钥路径 | Volume mount | TLS 启用时必填 |
| `JWT_PUBLIC_KEY_PATH` | JWT RS256 公钥路径 | Volume mount | JWT 认证时必填 |

### 密钥管理

| 密钥 | 当前方式 | 生产建议 |
|------|---------|---------|
| `DATABASE_URL` | Helm values 明文 | ExternalSecrets → K8s Secret |
| `HERMES_ACP_TOKEN` | Helm values 明文 | ExternalSecrets → K8s Secret |
| `REDIS_URL` | Helm values 明文 | ExternalSecrets → K8s Secret |

> **HIGH-10**：当前 Helm chart 将 secrets 以明文 env 注入。生产部署前必须迁移到 ExternalSecrets 或 SealedSecrets。

---

## 运行保障

### 健康检查

| 探针 | 路径 | 行为 |
|------|------|------|
| liveness | `/health/live` | 返回 200，仅检查进程存活 |
| readiness | `/health/ready` | 检查 DB 连接 (`Ping()`)，失败返回 503 |

### Prometheus 指标

| 指标 | 类型 | 说明 |
|------|------|------|
| `hermes_http_requests_total` | Counter | 按 method, path, status 分组 |
| `hermes_http_request_duration_seconds` | Histogram | 请求延迟分布 |
| `hermes_http_requests_in_flight` | Gauge | 当前并发请求数 |

### 监控与告警建议

| 告警项 | 条件 | 建议阈值 |
|--------|------|---------|
| 高错误率 | `rate(hermes_http_requests_total{status=~"5.."}[5m]) > 0.05` | 5% |
| 高延迟 | `histogram_quantile(0.99, hermes_http_request_duration_seconds) > 2` | P99 > 2s |
| Pod 不健康 | readiness probe 连续失败 | 3 次 |
| 速率限制触发 | `rate(hermes_http_requests_total{status="429"}[5m]) > 0.1` | 10% |

### 观察窗口

| 阶段 | 时长 | 关注项 |
|------|------|--------|
| 发布后即时 | 15 min | Pod 启动、readiness probe、DB 连接 |
| 短期观察 | 2 hours | 错误率、延迟、内存使用 |
| 中期观察 | 24 hours | 速率限制触发、审计日志写入、配额执行 |

---

## 恢复能力

### 回滚触发条件

- readiness probe 连续 3 次失败
- 5xx 错误率超过 5% 持续 5 分钟
- P99 延迟超过 5s 持续 10 分钟
- DB 连接池耗尽

### 回滚路径

```bash
# 1. Helm 回滚到上一版本
helm rollback hermes-agent --wait --timeout 120s

# 2. 验证回滚
kubectl get pods -l app=hermes-agent
curl -s http://<service>:3000/health/ready

# 3. 确认指标恢复
# 检查 Prometheus dashboard
```

### 回滚验证

- `/health/ready` 返回 200
- Prometheus 指标恢复基线
- 无新增 5xx 错误

---

## Helm Chart 配置

### 资源配置

```yaml
resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

### 副本配置

| 环境 | 副本数 | 说明 |
|------|--------|------|
| staging | 1 | 验证功能 |
| production | 2+ | 高可用 |

### 待补充配置（非本次范围）

- Ingress controller 配置
- HPA (Horizontal Pod Autoscaler)
- PDB (Pod Disruption Budget)
- NetworkPolicy
- SecurityContext (runAsNonRoot, readOnlyRootFilesystem)

---

*最后更新：2026-04-28*
