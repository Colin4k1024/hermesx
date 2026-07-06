# Release Plan: WorkBuddy 能力对齐 v2.5.0

| 字段 | 值 |
|------|-----|
| 状态 | draft |
| 主责 | devops-engineer |
| 日期 | 2026-07-06 |
| 版本 | v2.5.0 |

## 发布信息

| 字段 | 值 |
|------|-----|
| 版本 | v2.5.0 |
| 关联任务 | 2026-07-06-workbuddy-capability-align |
| 发布范围 | 三栏 WebUI + 多任务并行 + 办公产物生成 + 文件工作区 + 深度研究 |
| 发布负责人 | devops-engineer |
| 审批人 | tech-lead |
| 发布窗口 | 持续发布（无固定窗口） |

## 变更与风险

### 变更清单

| 类型 | 数量 | 说明 |
|------|------|------|
| 新增 Go 文件 | ~15 | tools, api, middleware, store |
| 修改 Go 文件 | ~10 | server, agent_chat, ratelimit, redis, store |
| 新增前端文件 | ~15 | workspace 组件, hooks, store |
| 修改前端文件 | ~8 | router, ResultsPanel, DialogArea, TaskSidebar |
| 数据库 migration | 6 | 124-129 (file_entries + RLS) |
| 新增 Go 依赖 | 1 | excelize/v2 |
| 新增 ADR | 3 | SSE 多流 / 文件 Hybrid / 文档生成 |

### 风险矩阵

| 风险 | 等级 | 缓解 |
|------|------|------|
| PG migration 阻塞启动 | Medium | 新表，不修改现有表结构 |
| Redis lock key 格式变化 | Medium | 已兼容新旧格式（滚动部署安全） |
| excelize 新依赖引入漏洞 | Low | Apache-2.0, MIT-licensed, 17k stars |
| Workspace UI 替代 Chat 的用户接受度 | Low | 旧路由保持可用 |

## 执行步骤

### Step 1: 预发布检查

- [ ] `go build ./...` 通过
- [ ] `go vet ./...` 通过
- [ ] `go test ./...` 通过
- [ ] `cd webui && npm run build` 通过
- [ ] 安全审查无遗留 CRITICAL 问题
- [ ] 前端质量门禁无遗留 BLOCK 问题

### Step 2: 构建镜像

```bash
docker build -f Dockerfile.saas -t hermesx:v2.5.0 .
```

### Step 3: 数据库预检

- [ ] 确认 PostgreSQL 版本 ≥ 16（RLS 支持）
- [ ] 确认 Redis 版本 ≥ 7
- [ ] 确认 MinIO 可用
- [ ] Migration 124-129 dry-run 验证

### Step 4: 滚动部署

**Docker Compose:**
```bash
docker compose -f docker-compose.prod.yml up -d --force-recreate hermesx-saas
```

**K8s Helm:**
```bash
helm upgrade hermesx deploy/helm/hermesx/ --set image.tag=v2.5.0
```

### Step 5: 部署后验证

- [ ] `curl localhost:8080/health/ready` 返回 200
- [ ] `/workspace` 页面可访问
- [ ] 创建新 session + 发送消息（SSE 流正常）
- [ ] 生成 xlsx 测试（`generate_spreadsheet` tool 可用）
- [ ] 文件上传/列表/下载端到端
- [ ] Prometheus 指标正常（无突增错误）

### Step 6: 观察窗口

| 时长 | 观察项 | 判定标准 |
|------|--------|---------|
| 0-15min | Health check, 错误日志 | 无 5xx 增长 |
| 15-60min | SSE 连接数, 沙箱执行 | 无 OOMKill, 无连接泄漏 |
| 1-4h | 用户反馈, 产物生成成功率 | 成功率 > 90% |
| 24h | 整体稳定性 | 无回滚触发 |

## 验证与监控

### 关键监控面板

| 面板 | 指标 | 正常范围 |
|------|------|---------|
| API 请求率 | `http_requests_total` | 无突增突降 |
| SSE 活跃连接 | `sse_active_connections` | < 80% 上限 |
| 工具执行耗时 | `tool_call_duration_seconds` | P95 < 30s |
| 文件存储用量 | `file_entries` 表记录数 | 增长平稳 |
| DB 连接数 | `pg_stat_activity` | 无连接泄漏 |

### 日志关键字

| 关键字 | 含义 | 处置 |
|--------|------|------|
| `SSE connection rejected` | 连接超限 | 正常限流 |
| `sandbox OOM` | 沙箱内存溢出 | 调整 SANDBOX_MEMORY_LIMIT_MB |
| `migration` + `error` | Migration 失败 | 停止部署，排查 |
| `RLS` + `violation` | 行级安全违规 | 检查 tenant context |

## 放行结论

| 条件 | 状态 |
|------|------|
| 测试计划已执行 | ✅ test-plan.md |
| 安全审查通过 | ✅ 7 项修复已完成 |
| 前端质量门禁通过 | ✅ 4 BLOCK 已修复 |
| Launch Acceptance 已签发 | ✅ launch-acceptance.md |
| 部署上下文已文档化 | ✅ deployment-context.md |
| 回滚路径已验证 | ✅ 新表 migration 可独立回滚 |

**放行结论：允许发布 v2.5.0**

## 回滚方案

```bash
# Docker Compose
docker compose -f docker-compose.prod.yml down
# 使用旧镜像 tag 重启
docker compose -f docker-compose.prod.yml up -d

# K8s Helm
helm rollback hermesx <previous-revision>

# Database (仅在严重问题时)
psql -c "DROP TABLE IF EXISTS file_entries; DELETE FROM schema_migrations WHERE version IN (124,125,126,127,128,129);"
```

## 后续观察项

| 项 | Owner | 时间 |
|----|-------|------|
| P95 响应时间稳定性 | devops | +24h |
| 用户 Workspace 使用率 | product | +1w |
| MinIO 存储增长 | devops | +1w |
| P2 修复清单执行 | backend | v2.5.1 |
