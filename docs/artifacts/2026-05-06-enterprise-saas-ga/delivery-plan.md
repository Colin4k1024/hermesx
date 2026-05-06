# Delivery Plan: Enterprise SaaS GA Hardening v1.2.0

**状态**: Draft  
**主责**: tech-lead  
**日期**: 2026-05-06  
**Slug**: enterprise-saas-ga  
**版本目标**: v1.2.0 GA

---

## 1. 版本目标

将 hermes-agent-go 从准生产级（6.2/10）推进到企业 GA（8.5+/10），修复所有 CRITICAL 安全缺口，补全商业完整性和运维可运营性。

**放行标准**：
- 5 个 CRITICAL 缺口全部修复（含 RLS WITH CHECK、审计不可篡改、默认凭证、GDPR MinIO、备份可验证）
- K8s PDB + HPA 生效（kind 集群验证）
- CI pipeline 全绿（go test + gofmt + race detection）
- 集成测试覆盖新增功能

---

## 2. Phase 划分与 Story Slices

### Phase 1 — CRITICAL 安全修复（P0）

**目标**：消除所有 CRITICAL 安全阻断缺口  
**前置依赖**：无  
**预估**：3-4 天

| Slice | US | 主责 | 验收标准 | 依赖 |
|-------|-----|------|---------|------|
| P1-S1 | US-01 | backend-engineer | 9 张表 WITH CHECK + FORCE RLS migration 通过；跨租户 INSERT 返回 policy violation | 无 |
| P1-S2 | US-02 | backend-engineer | REVOKE DELETE + SECURITY DEFINER GDPR 函数；应用角色 DELETE 返回 permission denied | P1-S1 |
| P1-S3 | US-03 | devops-engineer | docker-compose 无默认值；缺 env 启动失败；.env.example 占位符 | 无 |
| P1-S4 | US-04 | backend-engineer | GDPR deleteViaTx 后 MinIO prefix 为空；失败返回 207 + purge_audit_logs 记录 | P1-S2 |
| P1-S5 | US-05 | devops-engineer | pitr-drill.sh 完整执行；RPO delta 输出；Prometheus backup 指标 | 无 |
| P1-S6 | US-06 | devops-engineer | Helm PDB + HPA 模板；kind 集群 kubectl get pdb,hpa 有对象 | 无 |
| P1-S7 | US-14 | backend-engineer | session/message owner 校验；同租户跨用户 403 | 无 |

**P1 Handoff 终点**：所有 CRITICAL 修复 + 安全 Bug Fix 代码完成，集成测试通过，`go test ./...` 全绿。

---

### Phase 2 — 商业完整性（P1）

**目标**：补全 OAuth2/OIDC、动态计费、用户限流、断路器、账单  
**前置依赖**：Phase 1 完成（RLS WITH CHECK 影响写路径，须先稳定）  
**预估**：7-10 天

| Slice | US | 主责 | 验收标准 | 依赖 |
|-------|-----|------|---------|------|
| P2-S1 | US-07 | backend-engineer | OIDCExtractor + JWKS 动态刷新；mock IdP 测试通过 | P1 完成 |
| P2-S2 | US-08 | backend-engineer | pricing_rules 表 + Admin CRUD API；成本计算走 DB 优先 | P1 完成 |
| P2-S3 | US-09 | backend-engineer | 双层 Lua 限流（tenant + user/key）；X-RateLimit 头正确 | P1 完成 |
| P2-S4 | US-10 | backend-engineer | ProviderBreakerRegistry + ChatStream 统计修复；Prometheus gauge | P1 完成 |
| P2-S5 | US-11 | backend-engineer | billing summary + invoice JSON API；admin scope 鉴权 | P2-S2 |
| P2-S6 | US-13 | backend-engineer | OIDCExtractor ACR claim 校验；无 MFA token 访问 /admin 返回 403 | P2-S1 |

**P2 Handoff 终点**：所有 HIGH 优先级功能代码完成，接口契约锁定，单元/集成测试通过。

---

### Phase 3 — 能力完善与运维文档（P2）

**目标**：补全 MEDIUM 能力项 + 运维 Runbook + CI 覆盖率可见性  
**前置依赖**：Phase 2 接口锁定（Runbook 依赖接口）  
**预估**：4-6 天

| Slice | US | 主责 | 验收标准 | 依赖 |
|-------|-----|------|---------|------|
| P3-S1 | US-15 | backend-engineer | /admin/v1/tenants CRUD 路由 + PATCH + restore 端点 | P2 完成 |
| P3-S2 | US-16 | backend-engineer | createAPIKey 强制 ExpiresAt；List 响应 expires_soon 标志 | P2 完成 |
| P3-S3 | US-17 | backend-engineer | LoggingMiddleware 注入 trace_id/span_id；unit test 验证 | P2 完成 |
| P3-S4 | US-18 | devops-engineer | Helm Service sessionAffinity 条件化；nginx 注释更新 | P2 完成 |
| P3-S5 | US-19 | devops-engineer | CI -coverprofile + coverage artifact 上传 | P2 完成 |
| P3-S6 | US-20 | devops-engineer | NetworkPolicy 模板（enabled: false 默认） | P2 完成 |
| P3-S7 | US-12 | devops-engineer | 8 份 Runbook（incident-response 优先） | P2 接口锁定 |

**P3 Handoff 终点**：全部 20 个 US 代码/文档完成，CI 全绿，进入 review。

---

## 3. 角色分工

| 角色 | Phase 1 | Phase 2 | Phase 3 |
|------|---------|---------|---------|
| tech-lead | 方案决策、CRITICAL 优先级裁定、安全审查 | 接口契约收口、scope 决策 | 上线门禁、放行决策 |
| architect | RLS 设计审查、OIDC 架构指导 | breaker 注册表设计、pricing 接口审查 | NetworkPolicy 规则审查 |
| backend-engineer | P1-S1/S2/S4/S7 实现 | P2-S1~S6 实现 | P3-S1~S3 实现 |
| devops-engineer | P1-S3/S5/S6 实现 | 协助 kind 验证 | P3-S4~S7 实现 |
| qa-engineer | P1 集成测试 | P2 集成测试 + OIDC mock 验证 | 全量回归 + 覆盖率报告 |
| security-reviewer | P1 安全审查（RLS、审计、凭证） | OIDC 安全配置审查 | 最终安全扫描 |

---

## 4. 风险与缓解

| 风险 | 影响 | 概率 | 缓解措施 | Owner |
|------|------|------|---------|-------|
| RLS WITH CHECK 破坏现有写入路径 | HIGH | MEDIUM | 先测试库逐表验证；SET LOCAL 确保事务内生效 | backend-engineer |
| OIDC 需外部 IdP 配合测试 | HIGH | HIGH | mock IdP (httptest.Server) + coreos/go-oidc | backend-engineer |
| pitr-drill.sh Docker 环境限制 | MEDIUM | HIGH | docker exec 模式替代 volume inspect | devops-engineer |
| 双层限流 Lua 脚本复杂度 | MEDIUM | MEDIUM | 充分表驱动测试 + Redis 集群模式验证 | backend-engineer |
| Helm PDB minAvailable 阻塞 drain | LOW | HIGH | 条件化渲染 replicaCount >= 2 | devops-engineer |

---

## 5. 节点检查

| 检查点 | 时间 | 角色 | 门禁 |
|--------|------|------|------|
| Phase 1 安全审查 | P1 完成后 | security-reviewer | 无 CRITICAL/HIGH 遗留 |
| Phase 2 接口冻结 | P2-S5 完成后 | architect + tech-lead | 接口契约 JSON 通过对比审查 |
| Phase 3 全量回归 | P3 完成后 | qa-engineer | go test 全绿 + race detection PASS |
| Release Readiness | P3 + 回归后 | tech-lead | CI 全绿 + 安全扫描 PASS |

---

## 6. 降级到 v1.3.0 的功能

| 功能 | 原因 |
|------|------|
| API Key 30 天提醒通知 | 通知基础设施未建立 |
| CI 80% 覆盖率硬阻断 | 当前覆盖率 34%，需先提升基线 |
| Cookie 粘滞（nginx sticky） | Nginx 开源版不支持 |
| NetworkPolicy 生产验证 | kind CNI 不支持 NetworkPolicy enforce |
| per-tenant OIDC IdP 映射 | 单 IdP 限制 GA 文档说明 |
| invoice 不可变快照 | invoices 表状态机工作量大 |

---

## 7. Karpathy Guidelines 收口

### 核心假设
1. 所有写路径已在事务中 — 需逐包验证（store/pg 层）
2. GoBreaker v2 支持泛型 — 已确认 go.mod 中版本兼容
3. MinIO ListObjects + RemoveObject 可覆盖全部租户对象 — 需验证 soul/skills 前缀一致性
4. kind 集群 metrics-server 安装后 HPA 可正常工作 — 已有成熟脚本

### 更简单备选路径
- OIDC: 可用 `coreos/go-oidc/v3` 替代 `lestrrat-go/jwx`（更少依赖）
- 断路器: 可保持 model 级 breaker 不改粒度（但违背 US-10 要求）
- 账单: 可直接 SQL SUM 不做缓存（GA 数据量有限）

### 当前不做项
- WebUI 前端
- 多云/多地域部署
- SOC2 认证流程
- RL Training 接入
- PDF 发票

### 为什么本轮范围已足够
v1.2.0 修复所有 CRITICAL + HIGH 后，安全合规评分从 6.2 提升至 8.5+，满足企业采购审查的最低门槛。MEDIUM 项中 US-13~US-20 均为增量改进，不阻塞 GA 发布。

---

## 8. Implementation Readiness 结论

**状态**: `handoff-ready`

**前置证据**：
- ✅ PRD 完成（20 个 US，范围/成功标准明确）
- ✅ 需求挑战会完成（9 项 tech-lead 决策已记录）
- ✅ 接口约定已在 requirement-challenge.md 中锁定
- ✅ 技术依赖已满足（PG16、Redis7、MinIO SDK、gobreaker v2 均已在 go.mod）
- ✅ 待确认项已全部收口

**可执行条件**：本 delivery-plan 经 tech-lead 确认后，即可进入 `/team-execute` Phase 1。
