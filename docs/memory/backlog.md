# Backlog Snapshot

**来源任务**: 2026-05-06-enterprise-saas-ga  
**更新时间**: 2026-05-07  
**更新角色**: tech-lead

---

## Phase 3 候选项 (下一阶段)

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 1 | OIDC wiring 到 server.go auth chain | P1 | 运维提供 IdP 配置 | devops-engineer |
| 2 | 断路器 registry (ChatStream breaker 重构) | P2 | Phase 3 规划启动 | backend-engineer |
| 3 | CI/CD Pipeline (GitHub Actions) | P2 | Phase 3 规划启动 | devops-engineer |

## 技术债

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 4 | RLS SELECT policies 评估 | P3 | 读隔离需求确认 | architect |
| 5 | pgxmock 引入 (store 层 mock 测试) | P3 | 测试覆盖率提升需求 | backend-engineer |
| 6 | CORS 动态管理 (DB/config 加载) | P3 | 多域名需求出现 | backend-engineer |
| 7 | 多副本 LocalDualLimiter 精确性优化 | P3 | 生产 Redis 频繁故障 | backend-engineer |
| 8 | HasScope empty scopes 放行修复 | P3 | OIDC wiring 完成后 | backend-engineer |

## 产品需求候选

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 9 | Admin UI for pricing rules | P3 | 产品需求确认 | frontend-engineer |
| 10 | 用量 dashboard / 计费报告 | P3 | 产品需求确认 | frontend-engineer |
| 11 | GDPR 自助数据导出 UI | P4 | 合规需求 | frontend-engineer |
