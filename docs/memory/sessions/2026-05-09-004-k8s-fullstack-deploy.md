# Session 004: K8s MySQL 全栈部署 + E2E 测试

**日期**: 2026-05-09  
**链路**: deploy → test → fix → verify → release  
**产出**: v2.1.1 (commits 6e8e1ea → 0cd8341)

## 任务

将 HermesX 全栈（WebUI + SaaS API + MySQL + Redis + MinIO + Prometheus + Grafana）部署到本地 Kind K8s 集群，使用 MySQL 版本数据库后端，并通过 Playwright E2E 浏览器测试验证所有功能。

## 产出

### 代码变更（5 commits）

1. `6e8e1ea` — MySQL 部署修复 + SSE streaming 支持
2. `c35df20` — Memories/Skills/Usage API 修复
3. `420b037` — 多轮对话上下文修复 + Audit Logs JSON key
4. `b750db8` — /metrics 端点公开（Prometheus 可采集）
5. `0cd8341` — Lessons learned 文档

### Bug 修复（8 个）

| # | Bug | 根因 | 修复 |
|---|-----|------|------|
| 1 | sessions.id CHAR(36) 溢出 | session ID 格式 `sess_` + 32 hex = 37 chars | 改为 VARCHAR(64) |
| 2 | EndedAt nil pointer panic | `*time.Time` 为 nil 时调用 .IsZero() | 添加 nil 检查 |
| 3 | SSE streaming 不可用 | metrics middleware 的 statusWriter 未实现 Flusher | 使用 http.NewResponseController |
| 4 | Memories 页面空白 | X-Hermes-User-Id override 导致查询 user_id 不匹配 | 使用 ac.Identity 一致性 |
| 5 | Skills 详情 405 | GET /v1/skills/{name} 路由未注册 | 新增路由 |
| 6 | Usage 页面空白 | 响应字段名不匹配前端 | 修正为 input_tokens/output_tokens/total_tokens |
| 7 | 多轮对话上下文丢失 | SSE chunk ID 带 "chatcmpl-" 前缀导致 session 不匹配 | 移除前缀 |
| 8 | Audit Logs 空白 | 后端返回 "audit_logs" 前端读 "logs" | 修正 key |

### 基础设施验证

- Prometheus: 2 targets UP, hermesx_* 自定义指标正常采集
- Grafana: 10 面板 Dashboard 配置正确, Prometheus datasource 连接正常
- MySQL 8.4.6: 13 migration 全部应用，多租户数据隔离正确
- MinIO (替代 RustFS): 81 skills 成功上传
- Redis 7: 分布式速率限制正常

### E2E 测试结果

- 功能测试: 12/12 通过（Chat, Memories, Skills, Usage, Tenants, API Keys, Audit Logs）
- 深度测试: 22/22 通过（多轮对话 5 轮, 记忆持久化, 跨会话回忆, Skills 详情, 租户隔离双向验证）
- 基础设施测试: 14/14 通过（Prometheus, Grafana, Health, Metrics, Admin 管理功能）
- 截图证据: 57 张 Playwright 截图保存于 ~/Desktop/hermesx-test-report/

## 遗留事项

- Grafana SPA 路由在 port-forward 环境下不稳定（非 bug，Kind 环境限制）
- Skills 列表有重复项（tenant-scope + user-scope 双份显示）— 后续优化
- Session 命名仍为原始 ID，无可读标题 — 后续迭代
