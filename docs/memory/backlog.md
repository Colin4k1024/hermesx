# Backlog Snapshot

**来源任务**: 2026-05-07-v012-absorption  
**更新时间**: 2026-05-07  
**更新角色**: tech-lead

---

## v0.12 Absorption 残余项

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 1 | LifecycleHooks 接入 Gateway Runner | P1 | 下一 sprint 启动 | backend-engineer |
| 2 | SelfImprover 接入 Agent 循环 | P2 | 下一 sprint 启动 | backend-engineer |
| 3 | compress.go / curator.go prompt sanitization 一致性 | P2 | 安全加固 sprint | backend-engineer |
| 4 | payload.URL 路径遍历检查扩展 | P2 | 安全加固 sprint | backend-engineer |
| 5 | Curator O(n²) dedup 优化 | P3 | MaxMemories > 100 需求 | backend-engineer |
| 6 | Unicode bidi chars sanitization | P4 | LLM 安全要求升级 | backend-engineer |

## Phase 3 候选项

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 7 | OIDC wiring 到 server.go auth chain | P1 | 运维提供 IdP 配置 | devops-engineer |
| 8 | 断路器 registry (ChatStream breaker 重构) | P2 | Phase 3 规划启动 | backend-engineer |
| 9 | CI/CD Pipeline (GitHub Actions) | P2 | Phase 3 规划启动 | devops-engineer |

## 技术债

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 10 | RLS SELECT policies 评估 | P3 | 读隔离需求确认 | architect |
| 11 | pgxmock 引入 (store 层 mock 测试) | P3 | 测试覆盖率提升需求 | backend-engineer |
| 12 | CORS 动态管理 (DB/config 加载) | P3 | 多域名需求出现 | backend-engineer |
| 13 | 多副本 LocalDualLimiter 精确性优化 | P3 | 生产 Redis 频繁故障 | backend-engineer |
| 14 | HasScope empty scopes 放行修复 | P3 | OIDC wiring 完成后 | backend-engineer |
| 15 | GHA actions digest-pin | P3 | 安全扫描周期 | devops-engineer |

## 产品需求候选

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 16 | Admin UI for pricing rules | P3 | 产品需求确认 | frontend-engineer |
| 17 | 用量 dashboard / 计费报告 | P3 | 产品需求确认 | frontend-engineer |
| 18 | GDPR 自助数据导出 UI | P4 | 合规需求 | frontend-engineer |
