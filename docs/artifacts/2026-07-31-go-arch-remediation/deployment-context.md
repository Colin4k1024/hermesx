# Deployment Context — Go Architecture Remediation

**slug:** go-arch-remediation  
**状态:** released  
**主责角色:** devops-engineer  
**日期:** 2026-07-31

---

## 环境清单

| 环境 | 用途 | 部署目标 |
|------|------|---------|
| 开发 | 本地验证 | `go build ./...` + 单元测试 |
| staging | 集成验证 | Docker + docker-compose |
| production | 正式环境 | K8s Helm chart |

## 部署入口

| 入口 | 方式 | 前置条件 |
|------|------|---------|
| 主入口 | `git merge` → CI/CD → Docker build → Helm deploy | PR review 通过 |
| 手工入口 | `go build -o hermesx ./cmd/hermesx` | Go 1.25+ |
| 回退入口 | `git revert <commit>` → CI 重新构建 | 无 |

## 配置与密钥

| 配置项 | 来源 | 说明 |
|--------|------|------|
| `DATABASE_URL` | K8s Secret / env | PostgreSQL 连接串 |
| `REDIS_URL` | K8s Secret / env | Redis 连接串 |
| `HERMES_API_KEY_LLM` | K8s Secret / env | LLM API key |
| `MINIO_*` | K8s Secret / env | 对象存储凭证 |
| `HERMES_ENV` | K8s ConfigMap | 环境标识（production） |

**本次变更影响：** `Config.Save()` 现在脱敏 `APIKey`、`ObjStore.AccessKey`、`ObjStore.SecretKey`。生产环境密钥通过环境变量注入，不经过 `Save()`，无影响。

## 运行保障

| 维度 | 措施 |
|------|------|
| Feature flag | 无——本次为纯 bug fix，不需要灰度 |
| 灰度控制 | 可通过 K8s 滚动更新逐步部署 |
| 监控 | Prometheus metrics（scheduler、rate limiter、LLM 调用） |
| 告警 | scheduler shutdown 超时、cron job 失败率、Redis 连接失败 |
| 值守安排 | 合并后观察 24h |
| 观察窗口 | 48h |

## 恢复能力

| 维度 | 措施 |
|------|------|
| 回滚触发条件 | scheduler shutdown 阻塞、cron job 大面积失败、Redis 限流失效 |
| 回滚路径 | `git revert <commit>` → CI 重建 → Helm rollback |
| 验证方法 | `go build ./...` + `go test ./...` + 人工验证 scheduler stop |
