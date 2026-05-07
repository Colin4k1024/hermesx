# 压测报告 — MiniMaxi Anthropic API Mode

> 日期：2026-05-07  
> 环境：Docker Desktop (macOS) — docker-compose.saas.yml 全栈部署  
> 目标端点：`POST /v1/agent/chat`  
> LLM Provider：MiniMaxi Anthropic-compatible API (`https://api.minimaxi.com/anthropic`)

## 测试环境

| 组件 | 版本/配置 |
|------|-----------|
| hermes-saas | 本地构建（main branch） |
| PostgreSQL | 16 |
| Redis | 7 |
| MinIO | latest |
| API Mode | `anthropic` |
| 认证 | API Key (hk_xxx) |
| Rate Limit | 120 RPM / tenant（默认） |

## 代码修复

本次测试前修复了 API mode 传播缺陷：

- `internal/api/chat_handler.go` — 添加 `apiMode` 字段，读取 `HERMES_API_MODE` 环境变量
- `internal/api/agent_chat.go` — 添加 `agent.WithAPIMode(h.apiMode)` 确保 agent 使用正确协议

修复前：`detectAPIMode()` 无法识别 `minimaxi.com` 为 Anthropic 协议，导致协议不匹配。

## 测试结果

| 轮次 | 模型 | 并发 | 总请求 | 成功率 | 吞吐量 | P50 | P90 | P99 | Max |
|------|------|------|--------|--------|--------|-----|-----|-----|-----|
| 1（热身） | MiniMax-M2.7 | 10 | 30 | **100%** | 5.23 req/s | 1.49s | 1.87s | 2.10s | 2.20s |
| 2（高压） | MiniMax-M2.7 | 20 | 100 | **88%** | 6.49 req/s | 3.38s | 4.12s | 4.52s | 4.53s |
| 3（极速） | MiniMax-M2.7-highspeed | 5 | 50 | **100%** | 5.95 req/s | 823ms | 972ms | 1.08s | 1.08s |
| 4（极速高压） | MiniMax-M2.7-highspeed | 10 | 100 | **62%** | 7.97 req/s | 1.86s | 2.19s | 2.40s | 2.87s |

## 失败分析

所有失败均为 HTTP 429（限流），来源分两类：

### 1. 本地 Tenant Rate Limiter

默认 120 RPM / tenant。当并发×请求数在窗口内超限时触发。

### 2. MiniMaxi 上游 Token Plan 限制

```
Token Plan 主要面向个人开发者的交互式使用场景。
当前请求量较高，请稍后重试；如需支持更高并发或自动化任务，
建议升级至更高级别套餐，或使用按量付费 API。 (2062)
```

断路器正确记录了上游 429 并触发保护。

## 关键结论

1. **Hermes SaaS 平台零故障** — 无 5xx、无 panic、无内存泄漏，所有失败源自限流策略或上游配额
2. **Highspeed 模型延迟降低约 2x** — P50 从 1.49s → 823ms，适合低延迟场景
3. **中间件全链路验证通过** — Auth → Tenant → Audit → Rate Limit → Circuit Breaker → LLM 协议正确
4. **API Mode 传播修复生效** — `HERMES_API_MODE=anthropic` 正确应用到 agent 层
5. **断路器正确响应** — 上游 429 触发 circuit breaker 记录，防止雪崩

## 瓶颈与建议

| 瓶颈 | 当前限制 | 建议 |
|------|----------|------|
| MiniMaxi Token Plan | ~5-6 并发 | 升级至按量付费 API 套餐 |
| Tenant Rate Limit | 120 RPM | 生产环境按 SLA 调整 |
| LLM 延迟 | 1-3s (M2.7) / 0.6-1s (highspeed) | 使用 SSE streaming 改善用户感知 |

## 压测工具

`scripts/stresstest.go` — 支持参数：

```bash
go run scripts/stresstest.go \
  -c 10          # 并发数
  -n 100         # 总请求数
  -key "hk_xxx"  # API Key
  -url "http://localhost:18080"
  -model "MiniMax-M2.7-highspeed"
  -prompt "your prompt"
  -stream        # 启用 SSE 流式
```
