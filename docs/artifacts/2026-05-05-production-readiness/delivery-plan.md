# Delivery Plan: HermesX — Production Readiness

**状态**: Draft  
**主责**: tech-lead  
**日期**: 2026-05-05  
**版本目标**: v1.1.0 (Production Ready)

---

## 1. 版本目标

将 hermesx 从 v1.0.0（安全架构验证完成）推进到 v1.1.0（可接入首批企业客户的生产就绪状态）。

### 放行标准

- [ ] Redis 分布式限流在压测下误差 < 5%
- [ ] LLM 双路由（OpenAI→Anthropic）降级自动触发且正确标记
- [ ] CI Pipeline 包含集成测试门禁（PR 合并前强制）
- [ ] OTel trace 贯穿 HTTP→Store→LLM 全链路
- [ ] Admin API 覆盖 SandboxPolicy + API Key 管理
- [ ] Token 级用量持久化 + 聚合查询 API
- [ ] PG PITR 恢复演练成功
- [ ] 3 副本 + LB 下所有隔离测试通过
- [ ] 审计日志覆盖所有写操作

---

## 2. Brownfield 状态（关键发现）

**项目远比初始评估成熟。** 代码审查发现以下已实现：

| 能力 | 已有实现 | 差距 |
|------|----------|------|
| LLM 多 Provider | OpenAI+Anthropic+Gemini+Bedrock transport factory | 缺 fallback routing 编排 |
| 熔断 | sony/gobreaker per-model circuit breaker | 已完成 |
| 限流 | RateLimiter interface + per-tenant + local fallback | 缺 Redis sliding window impl |
| OTel | Tracer init + HTTP middleware + PG tracer | 已接线，需验证覆盖面 |
| Auth | API Key hash+expiry+revoke+roles + JWT | 缺 scope 细化 + rotation API |
| Redis | Client + distributed session locks | 已有基础 |
| Prometheus | Handler registered | 缺 custom business metrics |
| Token Usage | Response 中解析了 input/output tokens | 缺持久化到 DB |
| CI | build/test/lint/race/docker | 缺 integration tests |

**工作量重新评估**: 原计划 8 周 → 修订为 **4-5 周**（大量工作是"接线+增强"而非从零构建）。

---

## 3. 工作拆解

### Phase 1: 基础加固（Week 1）

| # | Story Slice | 主责 | 依赖 | 验收标准 |
|---|-------------|------|------|----------|
| 1.1 | Redis Sliding Window RateLimiter | backend-engineer | 无 | 实现 `RateLimiter` interface，unit test + benchmark |
| 1.2 | CI 集成测试门禁 | devops-engineer | 无 | GHA services(PG/Redis/MinIO) + `go test -tags=integration` 通过 |
| 1.3 | OTel 覆盖验证 | backend-engineer | 无 | 验证 trace 从 HTTP→middleware→store→LLM 全链路贯穿 |
| 1.4 | API Key Scope 增强 | backend-engineer | 无 | 新增 `scopes` 字段 + RBAC middleware 校验 |
| 1.5 | Prometheus Business Metrics | backend-engineer | 1.1 | request_total, llm_latency, rate_limit_rejected counters |

**Phase 1 交付物**: CI green with integration tests, Redis rate limiter live, tracing verified

---

### Phase 2: LLM 可靠性 + 扩展验证（Week 2-3）

| # | Story Slice | 主责 | 依赖 | 验收标准 |
|---|-------------|------|------|----------|
| 2.1 | LLM Fallback Router | backend-engineer | 无 | 主模型失败/超时→自动切换备选模型→response 标记 `degraded` |
| 2.2 | LLM Retry with Backoff | backend-engineer | 2.1 | 指数退避 3 次重试 + 可配置间隔 |
| 2.3 | LLM Integration Tests | qa-engineer | 2.1, 2.2 | 用真实 API（低成本模型）跑 streaming + retry + circuit break |
| 2.4 | Docker Compose 3 副本 | devops-engineer | 1.1 | docker-compose.ha.yml + Nginx LB + health check |
| 2.5 | 多副本隔离测试 | qa-engineer | 2.4 | 现有 38 集成测试在 3 副本 LB 下全部通过 |
| 2.6 | Redis 限流压测 | qa-engineer | 1.1, 2.4 | 100 并发 × 5 分钟, 限流准确率 > 95% |

**Phase 2 交付物**: LLM degradation works, multi-replica safe, rate limiting proven

---

### Phase 3: Admin API + 计量 + 备份（Week 3-4）

| # | Story Slice | 主责 | 依赖 | 验收标准 |
|---|-------------|------|------|----------|
| 3.1 | Admin API: SandboxPolicy CRUD | backend-engineer | 无 | POST/GET/PUT/DELETE /admin/v1/tenants/:id/sandbox-policy |
| 3.2 | Admin API: API Key Management | backend-engineer | 1.4 | Create/Rotate/Revoke/List endpoints + audit log |
| 3.3 | Token Usage Recording | backend-engineer | 无 | 新表 `usage_records` + 每次 LLM 调用后写入 |
| 3.4 | Usage Aggregation API | backend-engineer | 3.3 | GET /v1/usage?from=&to=&granularity=day |
| 3.5 | PG PITR Setup | devops-engineer | 无 | WAL archiving + pg_basebackup + restore runbook |
| 3.6 | PG PITR 恢复演练 | devops-engineer | 3.5 | 模拟数据丢失→恢复到 5 分钟前→数据验证通过 |

**Phase 3 交付物**: Admin self-service, metering live, backup verified

---

### Phase 4: 合规 + 安全收尾（Week 4-5）

| # | Story Slice | 主责 | 依赖 | 验收标准 |
|---|-------------|------|------|----------|
| 4.1 | 审计日志覆盖率验证 | qa-engineer | 无 | 所有 POST/PUT/DELETE 产生审计记录 |
| 4.2 | 审计日志查询 API | backend-engineer | 4.1 | GET /admin/v1/audit-logs?tenant_id=&from=&to= |
| 4.3 | GDPR 端到端验证 | qa-engineer | 无 | 数据导出 + 数据删除请求→验证完全清除 |
| 4.4 | 安全扫描集成 | devops-engineer | 无 | CI 加入 gosec + trivy 扫描 |
| 4.5 | 渗透测试准备文档 | qa-engineer | 所有前置 | 攻击面清单 + 已知防御 + 测试建议 |
| 4.6 | Production Deployment Guide | devops-engineer | 3.5 | 部署清单 + 环境变量 + 监控配置 |

**Phase 4 交付物**: Audit complete, compliance documented, deployment guide ready

---

## 4. 角色分工

| 角色 | 主要负责 Story |
|------|----------------|
| backend-engineer | 1.1, 1.3, 1.4, 1.5, 2.1, 2.2, 3.1-3.4, 4.2 |
| devops-engineer | 1.2, 2.4, 3.5, 3.6, 4.4, 4.6 |
| qa-engineer | 2.3, 2.5, 2.6, 4.1, 4.3, 4.5 |
| tech-lead | 方案决策, 交叉 review, 收口验收 |

---

## 5. 风险与缓解

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| Redis sliding window 在高并发下精度不足 | Medium | 限流误差 > 5% | 用 Lua 原子脚本 + benchmark 验证 |
| LLM 真实测试 API 费用失控 | Low | 预算超支 | 用 GPT-4o-mini / Claude Haiku，单次测试预算 < $5 |
| 3 副本下发现 hidden race condition | Medium | P2 延期 | 已有 race detection 基础 + 渐进式加副本 |
| PG PITR 恢复时间 > 1h | Low | 不满足 RTO | 使用增量备份 + 并行恢复 |

---

## 6. 依赖关系图

```
P1.1 (Redis Limiter) ─────┬──→ P2.4 (3-replica) ──→ P2.5 (isolation re-test)
                           ├──→ P2.6 (pressure test)
P1.2 (CI integration) ────┤
P1.3 (OTel verify) ───────┘
P1.4 (Key scope) ─────────→ P3.2 (Key mgmt API)

P2.1 (Fallback router) ──→ P2.2 (Retry) ──→ P2.3 (LLM integration test)

P3.3 (Usage recording) ──→ P3.4 (Usage API)
P3.5 (PITR setup) ────────→ P3.6 (Recovery drill)

P4.1 (Audit verify) ──────→ P4.2 (Audit API)
                            P4.3 (GDPR verify)
                            P4.4 (Security scan)
                            P4.5 (Pentest prep) ──→ P4.6 (Deploy guide)
```

---

## 7. 检查节点

| 节点 | 时间 | 检查内容 | 决策方 |
|------|------|----------|--------|
| P1 Review | Week 1 末 | CI green + Redis limiter + OTel trace | tech-lead |
| P2 Review | Week 3 末 | LLM fallback demo + 3-replica test result | tech-lead |
| P3 Review | Week 4 末 | Admin API demo + backup recovery evidence | tech-lead |
| Final Gate | Week 5 末 | 全部放行标准 checklist | tech-lead |

---

## 8. 新增数据库变更

### Migration v61: usage_records table
```sql
CREATE TABLE usage_records (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id),
    session_id TEXT NOT NULL,
    user_id TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL,
    provider TEXT NOT NULL,
    input_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    cache_read_tokens INT NOT NULL DEFAULT 0,
    cache_write_tokens INT NOT NULL DEFAULT 0,
    cost_usd NUMERIC(10,6) DEFAULT 0,
    degraded BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_usage_records_tenant_date ON usage_records(tenant_id, created_at);
```

### Migration v62: api_keys scope enhancement
```sql
ALTER TABLE api_keys ADD COLUMN scopes TEXT[] DEFAULT '{}';
-- scopes: ["read", "write", "admin", "sandbox"]
```

---

## 9. 新增 API 契约（概要）

| Method | Path | 说明 |
|--------|------|------|
| POST | /admin/v1/tenants/:id/sandbox-policy | Set sandbox policy |
| GET | /admin/v1/tenants/:id/sandbox-policy | Get sandbox policy |
| DELETE | /admin/v1/tenants/:id/sandbox-policy | Reset to default |
| POST | /admin/v1/tenants/:id/api-keys | Create API key |
| POST | /admin/v1/tenants/:id/api-keys/:kid/rotate | Rotate key |
| DELETE | /admin/v1/tenants/:id/api-keys/:kid | Revoke key |
| GET | /v1/usage | Tenant usage summary |
| GET | /v1/usage/details | Per-session usage |
| GET | /admin/v1/audit-logs | Query audit logs |

---

## 10. 技能装配清单

| 技能 | 启用原因 | 主责角色 |
|------|----------|----------|
| `golang-patterns` | Go 实现最佳实践 | backend-engineer |
| `golang-testing` | 集成测试编写 | qa-engineer |
| `api-design` | Admin API 设计 | backend-engineer |
| `deployment-patterns` | CI/CD + HA | devops-engineer |
| `security-review` | 安全扫描 + 审计 | qa-engineer |
| `postgres-patterns` | PITR + usage table | devops-engineer |
| `docker-patterns` | 多副本 compose | devops-engineer |

---

## 11. 不做项（本轮明确排除）

| 项目 | 排除原因 |
|------|----------|
| OIDC/SSO | 无客户需求驱动，API Key 已足够 |
| 多区域部署 | 超出首批客户需求 |
| 自动 PG failover | RTO<1h 不需要，PITR 够用 |
| Kubernetes 部署 | Docker Compose 阶段足够验证 |
| 正式安全认证 | 仅准备材料，不申请认证 |
| 前端 UI | 本次无前端变更 |

---

## 12. Implementation Readiness 结论

| 维度 | 状态 | 说明 |
|------|------|------|
| Challenge 完成 | ✅ | 7 项决策已锁定 |
| Design 就位 | ✅ | arch-design.md（并行产出中） |
| Brownfield 梳理 | ✅ | 已有基础远超预期，工作量下调 40% |
| 阻塞项 | 无 | 所有依赖已有基础 |
| ADR 需求 | 否 | 无架构级新选型（都是增强已有） |

**结论**: **Ready for `/team-execute`**。所有前置条件满足，可直接进入实现。

---

**已创建**: `docs/artifacts/2026-05-05-production-readiness/delivery-plan.md`
