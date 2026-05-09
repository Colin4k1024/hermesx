# Deployment Context: kind Local K8s — hermes-agent SaaS

| 字段 | 值 |
|------|-----|
| 环境 | kind (local Docker K8s) |
| 日期 | 2026-04-28 |
| 集群 | desktop (1 control-plane + 3 workers) |
| 架构 | 3-tier: hermes-agent (API) + PostgreSQL + Redis (待集成) |

---

## 集群信息

- **kind version**: v0.31.0
- **Kubernetes version**: v1.35.1
- **Container runtime**: containerd
- **节点**: 4 台（desktop-control-plane + desktop-worker/2/3）

## 环境清单

| 组件 | 类型 | 版本 | 命名空间 | 端口 |
|------|------|------|----------|------|
| hermes-agent | Deployment | local (saas-api) | default | 8080 (ClusterIP) |
| hermes-agent | Service | NodePort | default | 30080 |
| PostgreSQL | StatefulSet | postgres:16-alpine | default | 5432 (ClusterIP, headless) |

## 镜像

| 镜像 | 标签 | 来源 | 架构 |
|------|------|------|------|
| hermes-agent-saas | local | 本地构建 | linux/arm64 |
| postgres | 16-alpine | Docker Hub | linux/arm64 |

## 部署入口

### Hermes-agent（Helm）

```bash
cd deploy/helm/hermes-agent/
helm upgrade --install hermes-agent . \
  -f ../../kind/values.local.yaml \
  --wait --timeout 120s
```

### PostgreSQL（Raw YAML）

```bash
kubectl apply -f deploy/kind/postgres.yaml
```

## 配置

### DATABASE_URL（内部 Service DNS）

```
postgres://hermes:hermes-dev-password@postgres:5432/hermes?sslmode=disable
```

### 认证

- **HERMES_ACP_TOKEN**: `dev-token-change-in-production`
- **HERMES_API_KEY**: `dev-api-key-change-in-production`
- **CORS**: 允许所有来源（`*`），生产环境需限制

## 运行保障

- **健康检查**: `/health/live` + `/health/ready`（含 DB 连通性）
- **Prometheus**: `/metrics` 自动挂载（pod annotation）
- **Static Files**: `/static/admin.html`, `/static/index.html`
- **租户种子**: 自动创建默认租户 `00000000-0000-0000-0000-000000000001`

## 验证命令

```bash
# Pod 状态
kubectl get pods -l app=hermes-agent
kubectl logs -l app=hermes-agent --tail=20

# 健康检查（pod 内）
kubectl exec deploy/hermes-agent -- curl -s http://localhost:8080/health/live
kubectl exec deploy/hermes-agent -- curl -s http://localhost:8080/health/ready

# API 认证调用
kubectl exec deploy/hermes-agent -- curl -s \
  -H "Authorization: Bearer dev-token-change-in-production" \
  http://localhost:8080/v1/me | python3 -m json.tool

# 外部访问（kubectl port-forward）
kubectl port-forward svc/hermes-agent 18080:8080
# 然后访问 http://localhost:18080/admin.html
```

## 已知限制

- Redis 未集成（REDIS_URL 为空，使用 localLimiter fallback）
- 生产环境需替换所有 dev token
- 无 persistent storage（PostgreSQL 使用 emptyDir）
- 无 Ingress（NodePort 暴露，仅供本地开发）
