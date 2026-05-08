# Release Plan — hermesx-webui

**任务**: hermesx-webui  
**版本**: v2.1.0-webui（后端安全修复 + 前端 v0.1）  
**日期**: 2026-05-08  
**角色**: devops-engineer  
**状态**: released  
**关联**: deployment-context.md · launch-acceptance.md · test-plan.md

---

## 1. 发布信息

| 字段 | 内容 |
|------|------|
| 发布版本 | v2.1.0-webui (tag on main) |
| 发布范围 | 后端安全修复 (bootstrap.go, server.go) + 全新 webui/ 前端 SPA |
| 发布方式 | docker-compose build → 部署；前端 `npm run build` → nginx |
| 发布责任人 | devops-engineer |
| 观察窗口 | 上线后 24h（重点）→ 72h（收口） |
| 回滚阈值 | 任意 CRITICAL 异常 / Admin Console 无法登录 / SSE 流异常率 > 10% |

---

## 2. 变更与风险

### 变更清单

| 类别 | 变更 | 风险 |
|------|------|------|
| 后端安全 | `subtle.ConstantTimeCompare` 替换 ACP token 字符串比较 | 低 — 行为等价，防止 timing attack |
| 后端安全 | Bootstrap `sync.Mutex` TOCTOU guard | 低 — 仅序列化 bootstrap 创建 |
| 后端 | `Vary: Origin` CORS header | 低 — 新增响应头，不影响现有客户端 |
| 后端 | `GET /admin/v1/tenants/{id}/api-keys` 新端点 | 低 — 新接口，无破坏性变更 |
| 后端 | `POST/GET /admin/v1/bootstrap` 新端点 | 低 — 新接口，原有接口不变 |
| 后端 | 旧静态 HTML 文件删除（chat.html 等） | 低 — 已确认无 CI 依赖 |
| 前端 | 全新 webui/ SPA（Vue 3 + Vite multi-page） | 中 — 全新路由结构，需 smoke 验证 |
| 前端 | sessionStorage 不再存储原始 API key | 低 — UX tradeoff：刷新需重新登录 |
| 前端 | `isAdmin` 基于 roles 数组 | 低 — 更严格的权限检查 |
| CI | `webui.yml` 新增 workflow（最小权限） | 低 — 新 workflow，不影响现有 CI |

### 关键风险

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| SSE 流式代理配置不当 | 低 | 高 | nginx proxy_buffering off + read_timeout 300s 已配置 |
| Bootstrap 跨实例 TOCTOU | 极低 | 中 | 单实例 Mutex 已覆盖；多实例场景列入 backlog |
| 用户刷新页面需重新登录 | 确定 | 低（已知 tradeoff） | 上线前在文档中说明 |

---

## 3. 执行步骤

### Pre-flight（上线前）

```bash
# 1. 确认 Go build 干净
go build ./... 
# Expected: no output (exit 0)

# 2. 确认 webui CI 通过
cd webui && npm ci && npm run type-check && npm run build
# Expected: dist/ 产出 index.html + admin.html

# 3. 确认 bootstrap status 端点
curl -s http://localhost:18080/admin/v1/bootstrap/status
# Expected: {"bootstrap_required": true} (首次) 或 false (已初始化)

# 4. 确认 CORS Vary: Origin
curl -si -H "Origin: https://your-domain.com" http://localhost:18080/health/live | grep -i vary
# Expected: Vary: Origin
```

### 执行步骤

```
Step 1: git tag v2.1.0-webui && git push origin v2.1.0-webui
Step 2: docker-compose build --no-cache
Step 3: docker-compose up -d
Step 4: 健康检查 — GET /health/live → 200, GET /health/ready → 200
Step 5: Bootstrap smoke — GET /admin/v1/bootstrap/status → 200
Step 6: Admin Console smoke — 访问 /admin.html → 200
Step 7: User Portal smoke — 访问 / → 200
Step 8: 登录验证 — Admin Console 使用 bootstrap key 登录
Step 9: SSE smoke — 发送一条 chat 消息验证 token 流式接收
Step 10: 记录完成时间，进入观察窗口
```

### 暂停点 / Go-No-Go

| 检查点 | 通过条件 | 失败动作 |
|--------|---------|---------|
| Step 4 | 两个健康端点均 200 | 立即回滚 |
| Step 8 | Admin 登录成功 | 检查 Bootstrap 状态，必要时回滚 |
| Step 9 | SSE token 正常流式 | 检查 nginx proxy_buffering 配置 |

---

## 4. 验证与监控

### Smoke 验证清单

| 页面/端点 | 验证项 | 预期 |
|-----------|--------|------|
| `GET /` | HTTP 200 + index.html | ✅ |
| `GET /admin.html` | HTTP 200 + admin.html | ✅ |
| `GET /health/live` | HTTP 200 | ✅ |
| `GET /admin/v1/bootstrap/status` | HTTP 200 + JSON | ✅ |
| Admin Console 登录 | bootstrap key 有效 | ✅ |
| User Portal 登录 | user key + uid 有效 | ✅ |
| SSE 流式聊天 | token 逐字到达，finish 正常 | ✅ |
| sessionStorage 检查 | hx_user_key / hx_admin_key 不存在 | ✅ |
| CORS Vary header | curl 响应含 Vary: Origin | ✅ |

### 监控项

- `GET /health/ready` 持续 200（数据库 + Redis 连接正常）
- Prometheus `/metrics` — `hermesx_chat_requests_total` 正常增长
- nginx access log — SSE 端点 `/v1/chat/completions` 无异常 4xx/5xx 突增
- Bootstrap 端点 — 监控 401 spike（暴力破解探测）

---

## 5. 回滚方案

```bash
# 触发条件：Admin Console 无法访问 / SSE 流式失败率 > 10% / 健康检查连续失败

# 回滚步骤
docker-compose down
git checkout <previous-tag>       # e.g. v2.1.0
docker-compose up --build -d

# 验证回滚
curl -s http://localhost:18080/health/live   # → 200
curl -s http://localhost:18080/             # → index.html (旧版本)
```

**预期回滚时间**: < 5 分钟（同机重建）

---

## 6. 放行结论

| 维度 | 状态 |
|------|------|
| 全部 CRITICAL 安全修复 | ✅ 已落地并验证 |
| 全部 HIGH 修复 | ✅ 已落地 |
| Go build 干净 | ✅ `go build ./...` 通过 |
| launch-acceptance 结论 | ✅ 允许上线 |
| 前端 CI workflow | ✅ 已加入最小权限 |
| MEDIUM 遗留项（Bootstrap 限速、useSse 401/403） | ⚠️ 列入 backlog，下版本处理 |

**最终放行结论：✅ 允许上线。**  
devops-engineer — 2026-05-08

---

## 企业内控补充

- **应用等级**: T4（开发者工具 / POC 级别）
- **技术架构等级**: 单机 Docker Compose（可升级为 K8s NodePort）
- **关键组件偏离**: 无（使用标准 PostgreSQL + Redis + nginx + Go）
- **资产入口**: `docs/artifacts/2026-05-08-hermesx-webui/` 目录
