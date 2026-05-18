# Release Plan: HermesX Security Enhancement

> **Slug:** security-enhancement-ironclaw  
> **State:** released  
> **Date:** 2026-05-18  
> **Owner:** devops-engineer  
> **Status:** draft

---

## 发布信息

| 项目 | 内容 |
|------|------|
| 版本 | v2.2.0 (security enhancement) |
| 范围 | F3 Prompt Injection Defense + F2 Credential Isolation + F4 Network Allowlisting |
| 影响模块 | `internal/safety/` (新增), `internal/secrets/` (扩展), `internal/egress/` (新增) |
| 变更量 | ~3000 LOC 新增, ~50 LOC 修改 (registry.go + url_safety.go) |
| Breaking Changes | 无 (纯新增, 向后兼容) |
| DB Migration | 2 张新表 (safety_policies, egress_rules) |

---

## 变更与风险

### 变更清单

| 变更 | 影响 | 风险 |
|------|------|------|
| 新增 `internal/safety/` 包 (9 files) | Agent loop 可选接入 | 低 — 不接入则无影响 |
| 扩展 `internal/secrets/` (5 files) | LeakScanner 可选使用 | 低 — 不调用则零开销 |
| 新增 `internal/egress/` 包 (9 files) | 工具 HTTP client 可选接入 | 低 — 不接入则不拦截 |
| `internal/tools/registry.go` 新增字段 | ToolContext 结构体扩展 | 极低 — 零值不影响现有工具 |
| `internal/tools/url_safety.go` 导出函数 | 函数可见性变更 | 极低 — 新增导出, 不改签名 |
| DB: safety_policies 表 | 新表 | 极低 — 不影响现有表 |
| DB: egress_rules 表 | 新表 | 极低 — 不影响现有表 |

### 风险评估

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| Safety interceptor 增加延迟 | 低 | P99 超预算 | log_only 默认, benchmark 验证 < 50ms |
| LeakScanner 正则回溯 | 极低 | CPU spike | Go RE2 引擎无回溯, benchmark 验证 |
| 误报导致正常请求被拒 | 中 | 业务影响 | log_only 先观察 72h |
| DB migration 失败 | 低 | 部署阻塞 | 新增表无 FK, 可独立重试 |

---

## 执行步骤

### Pre-flight (发布前)

| 步骤 | 负责人 | 验证 |
|------|--------|------|
| 1. DB migration: 执行 `000001_add_safety_policies.sql` | DBA | 表存在, 无错误 |
| 2. DB migration: 执行 `000002_add_secret_patterns.sql` | DBA | 表存在, 无错误 |
| 3. 构建新版本 image | CI/CD | `go build ./...` 成功 |
| 4. 运行全量测试 | CI/CD | 1765 tests 通过 |
| 5. 推送 image 到 registry | CI/CD | image tag 可拉取 |

### Deployment (发布)

| 步骤 | 负责人 | Go/No-Go 判断 |
|------|--------|---------------|
| 6. 部署 staging | devops-engineer | healthz 200, smoke test 通过 |
| 7. Staging 观察 30min | devops-engineer | 无异常日志, 延迟基线稳定 |
| 8. **Go/No-Go:** staging 验证通过 | tech-lead | 确认继续 |
| 9. 灰度发布 production (1 pod) | devops-engineer | healthz 200 |
| 10. Production 灰度观察 1h | devops-engineer | 延迟/错误率正常 |
| 11. **Go/No-Go:** 灰度验证通过 | tech-lead | 确认全量 |
| 12. 全量发布 production | devops-engineer | 所有 pod 健康 |

### Post-deployment (发布后)

| 步骤 | 负责人 | 验证 |
|------|--------|------|
| 13. 验证 safety metrics 上报 | devops-engineer | Grafana 可见 |
| 14. 验证 egress metrics 上报 | devops-engineer | Grafana 可见 |
| 15. 确认 log_only 模式激活 | backend-engineer | 日志可见检测结果 |
| 16. 通知团队发布完成 | devops-engineer | Slack #releases |

---

## 验证与监控

### Smoke Test

| 验证项 | 方法 | 预期 |
|--------|------|------|
| API 健康 | `GET /healthz` | 200 |
| 正常对话请求 | 发起普通 agent 对话 | 正常返回 |
| Safety interceptor 不阻断 | log_only 模式下发请求 | 请求通过, 日志有检测记录 |
| 工具调用正常 | 调用 web 工具 | 正常返回 (未接入 SecureTransport, 不拦截) |

### 监控 Dashboard

- Grafana: `HermesX Security` dashboard (待创建)
- 包含: safety_check_duration, input_guard_matches, leak_detected, egress_blocked, canary_leaks

---

## 回滚方案

### 快速缓解 (不重启)

```
UPDATE safety_policies SET mode = 'disabled' WHERE tenant_id = '<affected_tenant>';
```

### 代码回滚

```bash
kubectl set image deployment/hermesx hermesx=registry/hermesx:v2.1.1
```

### 回滚验证

1. `GET /healthz` → 200
2. 核心 API 延迟恢复基线
3. safety/egress metrics 停止上报

### DB 兼容性

- 新增表 (safety_policies, egress_rules) 不影响旧版本 binary
- 回滚后表保留, 无数据丢失风险
- 无需 DB 回滚

---

## 放行结论

### Go/No-Go 检查

| 检查项 | 状态 |
|--------|------|
| 全量测试通过 | ✅ 1765 tests, 零回归 |
| 安全评审完成 | ✅ CRITICAL/HIGH 已修复 |
| 代码评审完成 | ✅ 无阻塞 |
| DB migration 就绪 | ⏳ 待 DBA review |
| 监控 dashboard 就绪 | ⏳ 待创建 |
| 回滚方案验证 | ✅ 向后兼容 |
| 文档更新 | ✅ ADR-006 + 全套 artifacts |

### 结论

**建议发布**, 以 `log_only` 模式上线, 72h 观察后再切换 `enforce`。

### 后续观察项

1. 误报率 (input_guard_matches 中 false positive 比例)
2. 性能影响 (safety_check_duration P99)
3. Canary token 效果验证 (注入真实 canary 验证检测链路)
4. 下一迭代: 工具层 SecureTransport 集成 + redirect 防护

---

## 企业内控补充

| 项目 | 内容 |
|------|------|
| 应用等级 | T2 (企业 Agent Runtime, 多租户) |
| 技术架构等级 | T2 (Go single binary, Kubernetes multi-replica) |
| 关键组件偏离 | 无 (纯 Go 实现, 无新外部依赖) |
| 资源隔离 | per-tenant policy isolation via DB + RLS |
| 资产文档入口 | `docs/artifacts/2026-05-18-security-enhancement-ironclaw/` |
