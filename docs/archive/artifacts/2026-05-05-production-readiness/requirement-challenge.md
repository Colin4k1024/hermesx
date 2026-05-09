# Requirement Challenge Session — Production Readiness

**日期**: 2026-05-05  
**主持**: tech-lead  
**参与**: tech-lead (决策), architect (待确认)

---

## 挑战结论

### Decision 1: 认证体系

| 挑战 | 结论 |
|------|------|
| Keycloak 对 <50 租户是否过重？ | **暂不做 OIDC** |
| 替代方案？ | 仅增强 API Key（密钥轮换 + 作用域控制 + 过期机制） |
| 理由 | YAGNI — 没有客户明确要求 SSO，先把 API Key 做到位 |
| 后续触发条件 | 首个企业客户提出 SSO 需求时启动 OIDC |

**Impact**: P1 范围显著缩减，不含认证系统重构。

---

### Decision 2: LLM 集成范围

| 挑战 | 结论 |
|------|------|
| 只做 OpenAI 还是多路由？ | **OpenAI + Anthropic 双路由** |
| 架构方案 | 统一 Provider 接口 → 首期实现 OpenAI + Anthropic 两个 adapter |
| 降级策略 | 主模型超时/失败 → 自动降级到备选模型 → 返回降级标记 |
| 测试策略 | 用低成本模型（GPT-4o-mini / Claude Haiku）做集成测试 |

**Impact**: P2 工作量确认为 1.5x（双 adapter + 路由逻辑 + 降级测试）。

---

### Decision 3: 水平扩展

| 挑战 | 结论 |
|------|------|
| 验证方式 | **Docker Compose 多副本 + Nginx LB** |
| 理由 | 成本最低，本地可跑，CI 可复现 |
| 验证目标 | 3 副本下所有隔离测试通过 + 无 session 状态泄露 |
| 与真实生产差距 | 接受差距，记录为已知限制 |

---

### Decision 4: 计费计量

| 挑战 | 结论 |
|------|------|
| 粒度 | **Token 级精确计量** |
| 实现 | 每次 LLM 调用后写入 usage_records 表（input/output/model/cost） |
| 聚合 | 支持按 tenant/day/month 维度聚合查询 |
| 理由 | Token 级是最灵活的基础，向上聚合容易，向下拆分不可能 |

---

### Decision 5: CI 集成测试

| 挑战 | 结论 |
|------|------|
| 方式 | **GitHub Actions Services** |
| 理由 | 配置简单，与现有 CI 无缝集成，不需要额外 docker-compose step |
| 服务 | PG 16 + Redis 7 + MinIO |

---

### Decision 6: 灾备目标

| 挑战 | 结论 |
|------|------|
| RTO/RPO | **RTO<1h / RPO<5min** |
| 实现 | PG WAL archiving + PITR（pg_basebackup + wal-g） |
| 不需要 | 流复制、多主、自动 failover |
| 验证 | 写恢复 runbook + 实际恢复演练 |

---

### Decision 7: 时间约束

| 挑战 | 结论 |
|------|------|
| Deadline | **无硬性 deadline** |
| 节奏 | 按质量推进，每 Phase 完成后评估下一步 |
| 最小可交付 | P1 完成后即具备"可监控的可靠系统"基线 |

---

## 范围修订

### 原 P1（认证 + CI + 可观测性）→ 修订后 P1

| 原计划 | 修订 | 原因 |
|--------|------|------|
| OAuth2/OIDC 中间件 | **移除** | 暂不需要 |
| API Key 增强 | **新增** | 密钥轮换 + Scope + 过期 |
| CI 集成测试 | 保留 | GHA Services 方式 |
| OpenTelemetry 接线 | 保留 | — |

### 修订后 Phase 结构

```
P1 (Week 1-2): API Key 增强 + CI 门禁 + OTel 全链路
P2 (Week 3-4): LLM 双路由(OpenAI+Anthropic) + 分布式限流 + Docker Compose 3 副本验证
P3 (Week 5-6): Admin API + Token 计量 + PG PITR 备份恢复
P4 (Week 7-8): 审计完善 + 合规文档 + 安全扫描
```

---

## 被否决的选项（记录为 ADR 输入）

| 选项 | 否决原因 |
|------|----------|
| 自建 Keycloak | 运维成本过重，无客户需求驱动 |
| 仅 OpenAI 单路由 | 无法满足双模型降级需求 |
| Kind/云 K8s 验证 | 复杂度过高，Docker Compose 够用 |
| Session 级计费 | 精度不够，后续无法拆分 |
| docker-compose in CI | 与 GHA Services 相比无明显优势且耗时更长 |

---

## 新增待确认项

| # | 问题 | 影响 Phase |
|---|------|-----------|
| 1 | API Key scope 设计：read-only / read-write / admin 三级？ | P1 |
| 2 | Anthropic adapter 是否需要 streaming？ | P2 |
| 3 | 限流的 Redis key 设计：per-tenant vs per-key vs per-user | P2 |
| 4 | usage_records 表是否需要分区（按月）？ | P3 |
| 5 | PITR 备份存储位置：本地 vs S3 | P3 |

---

**挑战会状态**: COMPLETED  
**下一步**: `/team-plan` 基于修订后的 Phase 结构出详细交付计划
