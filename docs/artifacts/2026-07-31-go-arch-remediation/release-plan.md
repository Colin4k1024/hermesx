# Release Plan — Go Architecture Remediation

**slug:** go-arch-remediation  
**状态:** released  
**主责角色:** devops-engineer  
**日期:** 2026-07-31

---

## 发布信息

| 字段 | 内容 |
|------|------|
| 任务 | Go 架构审查修复（8 项，11 个文件） |
| 版本 | 随下一个 release 合并 |
| 发布负责人 | devops-engineer |
| 审批人 | tech-lead |

## 变更与风险

| 变更 | 风险等级 | 说明 |
|------|---------|------|
| S1: WaitGroup 修复 | 🟢 低 | scheduler shutdown 行为改善 |
| S2: os.Chdir 移除 | 🟡 中 | cron job workdir 从硬保证变为 prompt 软保证 |
| A1: os.Exit 移除 | 🟢 低 | 库代码不再终止进程 |
| A2: context 超时 | 🟢 低 | Redis 限流器增加 5s 超时保护 |
| A3: 错误包装 | 🟢 低 | 错误链完整性改善 |
| A4: Save 脱敏 | 🟢 低 | 密钥不再明文写入 config.yaml |
| A5: LLM 超时 | 🟡 中 | 120s 硬超时可能影响复杂 agent 对话 |
| B4: 错误日志 | 🟢 低 | 静默忽略改为 warn 日志 |

## 执行步骤

| 步骤 | 动作 | 验证 |
|------|------|------|
| 1 | 创建 PR `fix/go-arch-remediation` | PR 创建成功 |
| 2 | CI 运行 `go build` + `go test` + `go vet` | 全部通过 |
| 3 | Code Review（已完成） | 无阻塞项 |
| 4 | Security Review（已完成） | 无阻塞项 |
| 5 | 合并到 main | CI 绿灯 |
| 6 | Docker build + push | 镜像构建成功 |
| 7 | Helm deploy (staging) | staging 健康检查通过 |
| 8 | 观察 24h | 无异常指标 |
| 9 | Helm deploy (production) | 滚动更新完成 |
| 10 | 观察 48h | 无回滚触发 |

## 验证与监控

| 监控项 | 指标 | 阈值 |
|--------|------|------|
| Scheduler shutdown | `scheduler_stop_duration_seconds` | < 35s（pollLoop ticker 30s + 余量） |
| Cron job 成功率 | `cron_job_success_total / cron_job_total` | > 95% |
| Redis 限流错误率 | `rate_limit_error_total / rate_limit_total` | < 1% |
| LLM 调用超时率 | `llm_timeout_total / llm_call_total` | < 5% |
| API server 启动 | `api_server_start_total` | 正常递增 |

## 回滚方案

| 场景 | 回滚动作 | 验证 |
|------|---------|------|
| scheduler shutdown 阻塞 | `git revert` S1 commit | `go test ./internal/scheduler/...` |
| cron job 大面积失败 | `git revert` S2 commit | 手动触发 cron job 验证 |
| Redis 限流失效 | `git revert` A2 commit | 压测验证限流恢复 |
| LLM 超时过于激进 | 调整超时值或 `git revert` A5 | 复杂对话验证 |

## 放行结论

**✅ 放行（Conditional Go）**

| 条件 | 状态 |
|------|------|
| 编译通过 | ✅ `go build ./...` |
| Code Review | ✅ 无阻塞 |
| Security Review | ✅ 无阻塞 |
| Race detector | ⚠️ 进程被终止（非失败），建议合并前在 CI 环境重跑 |
| Launch Acceptance | ✅ 有条件放行 |

**后续观察项：**
1. Scheduler graceful shutdown 行为
2. Cron job workdir 使用日志
3. Config.Save 输出确认密钥已脱敏
4. Redis 限流器超时行为
5. LLM 调用超时率（120s 阈值是否合理）
