# Deployment Context — hermesx-webui

**任务**: hermesx-webui  
**版本**: v2.1.0 + webui v0.1  
**日期**: 2026-05-08  
**角色**: devops-engineer  
**状态**: released

---

## 1. 环境清单

| 环境 | 用途 | 访问入口 |
|------|------|---------|
| local-dev | 本地开发，前端 HMR | `npm run dev` → http://localhost:5173（User）, http://localhost:5174（Admin）via vite proxy |
| docker-compose | 全栈集成验收 | `docker-compose up` → http://localhost:18080 |
| K8s local | 本地 K8s 验证 | NodePort 30080（API）, 30081（WebUI）|
| production | 生产 SaaS | 由运营团队指定域名（SAAS_ALLOWED_ORIGINS 配置） |

---

## 2. 部署入口

### 主入口（Docker Compose）

```bash
# 1. 复制并填充环境变量
cp .env.example .env
# 编辑 .env — 填写 LLM_API_KEY, HERMES_ACP_TOKEN, POSTGRES_PASSWORD, MINIO/RUSTFS keys

# 2. 构建并启动全栈
docker-compose up --build -d

# 3. Bootstrap（首次）
curl -X POST http://localhost:18080/admin/v1/bootstrap \
  -H "Authorization: Bearer <HERMES_ACP_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"name":"initial-admin-key"}'
# 保存返回的 key — 只显示一次
```

### 前端独立构建

```bash
cd webui
npm ci
npm run build          # 产出 dist/index.html + dist/admin.html + assets/
npm run type-check     # TypeScript 校验
```

### 回退入口

```bash
# 回滚到上一镜像版本
docker-compose down
git checkout <prev-tag>
docker-compose up --build -d
```

---

## 3. 配置与密钥

| 变量 | 用途 | 来源 | 必填 |
|------|------|------|------|
| `LLM_API_URL` | LLM provider base URL | .env | ✅ |
| `LLM_API_KEY` | LLM provider API key | .env / Secret Manager | ✅ |
| `LLM_MODEL` | 模型名 | .env | ✅ |
| `HERMES_ACP_TOKEN` | Bootstrap 一次性管理令牌（≥32 chars） | `openssl rand -hex 32` | ✅ |
| `HERMES_API_KEY` | 应用级 API key（legacy，可选） | .env | — |
| `POSTGRES_USER/PASSWORD/DB` | PostgreSQL 凭证 | .env | ✅ |
| `MINIO_ACCESS_KEY/SECRET_KEY` | 对象存储凭证（RustFS/MinIO） | .env | ✅ |
| `REDIS_PASSWORD` | Redis 密码（可为空） | .env | — |
| `SAAS_ALLOWED_ORIGINS` | CORS 允许的前端 origin（逗号分隔） | .env | ✅ (prod) |
| `OIDC_ISSUER_URL` | OIDC IdP 地址（可选，设置则启用 SSO） | .env | — |

**密钥生成**：`openssl rand -hex 32`  
**禁止**：在 `.env` 外明文存储 `HERMES_ACP_TOKEN`；Bootstrap 完成后从环境中清除或旋转。

---

## 4. WebUI Nginx 配置关键项

```nginx
# SSE 流必须关闭 proxy_buffering
location /v1/chat/ {
  proxy_buffering off;
  proxy_read_timeout 300s;
  proxy_http_version 1.1;
  proxy_set_header Connection '';
}

# Admin API 路由必须在 /admin SPA fallback 之前
location /admin/v1/ { proxy_pass ...; }
location /admin     { try_files $uri $uri/ /admin.html; }

# User Portal SPA fallback
location /          { try_files $uri $uri/ /index.html; }
```

---

## 5. 运行保障

| 项目 | 配置 |
|------|------|
| 健康检查 | `GET /health/live` → 200；`GET /health/ready` → 200 |
| Prometheus 指标 | `GET /metrics`（需 admin scope 或内网访问） |
| 日志 | JSON structured logs via `log/slog` |
| 告警建议 | 监控 `/v1/chat/completions` P99 > 10s；Bootstrap 端点异常 401 突增 |
| 观察窗口 | 上线后 24h 重点观察；72h 后收口 |

---

## 6. 恢复能力

| 触发条件 | 回滚路径 | 验证方法 |
|---------|---------|---------|
| WebUI build 失败 | 保留上版静态文件，回滚 docker image | `GET /` → index.html 200 |
| Bootstrap 端点双重创建 | sync.Mutex + 403 guard 已保护；DB 查验确认单 key | `GET /admin/v1/bootstrap/status` → false |
| SSE 连接不通 | 检查 nginx proxy_buffering off；检查 CORS Vary: Origin | curl 测试 SSE 端点 |
| CORS 错误 | 检查 SAAS_ALLOWED_ORIGINS 是否包含前端 origin | 浏览器 DevTools Network |
