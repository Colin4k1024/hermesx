# Project Context: hermes-agent-go

**项目名**: hermes-agent-go  
**当前任务**: 2026-05-06-enterprise-saas-ga  
**阶段**: handoff-ready (plan → execute)  
**版本目标**: v1.2.0 GA

## Tech Stack

- Go 1.25 + PostgreSQL 16 + Redis 7 + MinIO
- coreos/go-oidc/v3 (新增, OIDC SSO)
- gobreaker v2 (已有, provider 级断路器)
- minio-go/v7 (已有, GDPR 对象清理)
- Helm v3 (PDB/HPA/NetworkPolicy)

## 当前状态

- v1.1.0 已提交 main (commit 7aae693+)
- PRD + 需求挑战会已完成, 所有待确认项已收口
- delivery-plan.md + arch-design.md 已产出
- 20 个 User Story, 3 个 Phase, 预估 14-20 天

## 风险

- RLS WITH CHECK 可能影响现有写路径 (需逐表验证事务包装)
- OIDC 集成需 mock IdP 测试 (无外部依赖)
- pitr-drill.sh Docker 环境限制 (docker exec 替代)
- 双层 Lua 限流复杂度 (表驱动测试覆盖)

## 下一步

1. tech-lead 确认 delivery-plan 后进入 /team-execute Phase 1
2. Phase 1: CRITICAL 安全修复 (RLS, 审计不可篡改, 默认凭证, GDPR MinIO, PITR, K8s HA)
3. Phase 2: 商业完整性 (OIDC, 动态计费, 用户限流, 断路器, 账单)
4. Phase 3: 能力完善 + Runbook
