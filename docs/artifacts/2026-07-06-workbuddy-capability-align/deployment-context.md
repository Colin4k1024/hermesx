# Deployment Context: WorkBuddy 能力对齐

| 字段 | 值 |
|------|-----|
| 状态 | draft |
| 主责 | devops-engineer |
| 日期 | 2026-07-06 |
| 目标版本 | v2.5.0 |

## 环境清单

| 环境 | 用途 | 访问入口 | 部署目标 |
|------|------|---------|---------|
| 生产 SaaS | 多租户在线服务 | `hermesx saas-api` :8080 | Docker Compose / K8s Helm |
| 预发 | 变更验证 | :18080 | docker-compose.saas.yml |
| 集成测试 | CI 自动化 | 测试端口隔离 | docker-compose.test.yml |

## 部署入口

| 入口 | 命令 | 前置条件 |
|------|------|---------|
| 主入口 | `docker compose -f docker-compose.prod.yml up -d` | PG 16+, Redis 7+, MinIO |
| K8s 主入口 | `helm upgrade --install hermesx deploy/helm/hermesx/` | K8s 1.28+, Helm 3 |
| 手工回退 | `docker compose -f docker-compose.prod.yml down && docker compose -f docker-compose.prod.yml up -d --no-build hermesx-saas` | 旧镜像 tag |

## 本次变更配置与密钥

### 新增环境变量

| 变量 | 默认值 | 说明 | 必需 |
|------|--------|------|------|
| `SANDBOX_MEMORY_LIMIT_MB` | 512 | Python 沙箱内存限制 | 否 |
| `SANDBOX_CPU_LIMIT` | 1 | Python 沙箱 CPU 核数 | 否 |
| `SANDBOX_TIMEOUT_SEC` | 120 | Python 沙箱超时 | 否 |
| `SANDBOX_OUTPUT_DIR` | /tmp/output/ | 沙箱输出目录 | 否 |
| `MAX_SSE_STREAMS_PER_USER` | 5 | per-user SSE 并发上限 | 否 |

### 数据库 Migration

| Migration | 内容 | 风险 |
|-----------|------|------|
| 124-129 | `file_entries` 表 + RLS 策略 + 索引 + 唯一约束 | **Medium** — 新表，不修改现有表 |

- Migration 自动执行（Go 代码内置）
- 回滚方式：手动 DROP TABLE file_entries + 删除 migration 记录

### MinIO 配置

| 配置项 | 值 | 说明 |
|--------|-----|------|
| 新增 key 前缀 | `{tenant}/{user}/workspace/` | 用户工作区文件 |
| 新增 key 前缀 | `{tenant}/{user}/sessions/{sid}/` | 会话临时文件 |
| 新增接口 | `PutObjectWithContentType` | 带 MIME 类型的上传 |

### 新增依赖

| 依赖 | 版本 | 许可证 | 用途 |
|------|------|--------|------|
| `github.com/xuri/excelize/v2` | latest | Apache-2.0 | xlsx 生成 |

### 运行时依赖（Python sandbox 内）

| 包 | 用途 | 安装方式 |
|---|------|---------|
| python-docx | docx 生成 | sandbox 内 pip install |
| python-pptx | pptx 生成 | sandbox 内 pip install |

## 运行保障

### Feature Flag

| Flag | 说明 | 回退行为 |
|------|------|---------|
| 无新增 feature flag | 所有功能默认启用 | 旧 `/chat` 路由保持可用作为降级 |

### 灰度控制

- 本次无灰度需求，全量发布
- `/workspace` 路由与 `/chat` 并存，用户可自行选择

### 监控项

| 指标 | 来源 | 告警阈值 |
|------|------|---------|
| `sse_active_connections` | Prometheus | > 80% per-user limit |
| `sse_rejected_total` | Prometheus | > 0（429 拒绝） |
| 文件上传 `upload_bytes_total` | API 日志 | 无告警，观察 |
| 办公产物生成 `tool_call_duration_seconds` | Agent traces | P95 > 30s |
| PG migration 执行时间 | 启动日志 | > 60s 需排查 |

### 告警

| 告警 | 条件 | 处置 |
|------|------|------|
| SSE 连接池告警 | `sse_active_connections > 4 * max_users` | 检查是否有连接泄漏 |
| 沙箱 OOM | 容器 OOMKill | 检查 SANDBOX_MEMORY_LIMIT_MB |
| 文件存储配额告警 | 用户存储 > 80% quota | 通知用户清理 |

## 恢复能力

### 回滚触发条件

| 条件 | 动作 |
|------|------|
| Go build 失败 | 停止发布，修复后重试 |
| Health check 持续失败 > 2min | 回滚到上一个镜像 |
| SSE 连接错误率 > 10% | 回滚 |
| 文件上传错误率 > 5% | 回滚 |

### 回滚路径

1. **Docker Compose**: `docker compose down && docker compose up -d --no-build` 用旧镜像 tag
2. **K8s Helm**: `helm rollback hermesx <revision>`
3. **Database**: Migration 124-129 为新增表，回滚只需 `DROP TABLE file_entries`（数据丢失，但为新功能无存量数据）
4. **Redis lock key**: 旧 key 格式已在代码中兼容处理，回滚无需额外操作

### 验证方法

| 验证项 | 方法 |
|--------|------|
| 健康检查 | `curl localhost:8080/health/ready` |
| WebUI 可用 | 访问 `/workspace` 路由 |
| 文档生成 | 调用 `generate_spreadsheet` tool |
| 文件上传 | 调用 `POST /v1/files/upload` |
| SSE 连接 | 建立 3 个并行 SSE 流 |
