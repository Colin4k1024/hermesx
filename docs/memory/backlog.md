# Backlog Snapshot

**来源任务**: 2026-05-07-v012-absorption
**更新时间**: 2026-05-08
**更新角色**: writer-2

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

## Phase 3 已完成项

| # | 事项 | 状态 | 完成版本 |
|---|------|------|----------|
| 7 | OIDC wiring 到 server.go auth chain | ✅ 完成 | v1.3.0 |
| 8 | 断路器 Prometheus metrics + failure recording | ✅ 完成 | v1.3.0 |
| 9 | CI/CD Pipeline (GitHub Actions + ghcr.io) | ✅ 完成 | v1.3.0 |

## v1.4.0 已完成项

| # | 事项 | 状态 | 完成版本 |
|---|------|------|----------|
| 10 | ExecutionReceipt API + store + ReceiptRecorder | ✅ 完成 | v1.4.0 |
| 11 | Auditor RBAC 角色 + execution-receipts 路由守卫 | ✅ 完成 | v1.4.0 |
| 12 | Prometheus Business Metrics (tool/chat/store) | ✅ 完成 | v1.4.0 |
| 13 | OpenAPI 规范扩展（22 端点，完整 schema） | ✅ 完成 | v1.4.0 |
| 14 | Production Compose (PostgreSQL 16 + Redis 7 + MinIO) | ✅ 完成 | v1.4.0 |
| 15 | OTel + Jaeger 链路追踪 | ✅ 完成 | v1.4.0 |
| 16 | 备份/恢复脚本 (pg_dump) | ✅ 完成 | v1.4.0 |
| 17 | Model Catalog hot-reload + CJK trigram search | ✅ 完成 | v1.4.0 |
| 18 | Gateway platform registry refactor | ✅ 完成 | v1.4.0 |
| 19 | MultimodalRouter (image/audio/video dispatch) | ✅ 完成 | v1.4.0 |
| 20 | Autonomous Memory Curator | ✅ 完成 | v1.4.0 |
| 21 | Self-improvement Loop | ✅ 完成 | v1.4.0 |
| 22 | Gateway Media Parity + Lifecycle Hooks | ✅ 完成 | v1.4.0 |

## v2.0.0 待完成项

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 23 | HermesX 品牌文档同步（所有文档 branding 更新） | P1 | v2.0.0 release 前 | writer |
| 24 | v2.0.0 Release Notes 编写 | P1 | v2.0.0 release 前 | writer |
| 25 | ExecutionReceipt API 文档完善（集成示例 + 幂等性说明） | P2 | 文档同步 sprint | writer |
| 26 | API Reference 文档与代码端点完整对齐（见 api-reference.md） | P2 | 代码变更后 | writer |

## 技术债

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 27 | RLS SELECT policies 评估 | P3 | 读隔离需求确认 | architect |
| 28 | pgxmock 引入 (store 层 mock 测试) | P3 | 测试覆盖率提升需求 | backend-engineer |
| 29 | CORS 动态管理 (DB/config 加载) | P3 | 多域名需求出现 | backend-engineer |
| 30 | 多副本 LocalDualLimiter 精确性优化 | P3 | 生产 Redis 频繁故障 | backend-engineer |
| 31 | HasScope empty scopes 放行修复 | P3 | OIDC wiring 完成后 | backend-engineer |
| 32 | GHA actions digest-pin | P3 | 安全扫描周期 | devops-engineer |

## 产品需求候选

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 33 | Admin UI for pricing rules | P3 | 产品需求确认 | frontend-engineer |
| 34 | 用量 dashboard / 计费报告 | P3 | 产品需求确认 | frontend-engineer |
| 35 | GDPR 自助数据导出 UI | P4 | 合规需求 | frontend-engineer |

---

## GitHub Issues 草稿

### P1 - 必须完成

- [ ] **[P1] LifecycleHooks 接入 Gateway Runner**: 将 LifecycleHooks 集成到 Gateway Runner 执行链路，实现生命周期钩子的真正串联，而非仅独立正确。 (Owner: backend-engineer, Label: enhancement)
- [ ] **[P1] HermesX 品牌文档同步**: 完成所有文档（README, ARCHITECTURE, DEPLOYMENT, API Reference, Quickstart 等）的 hermes → hermesx rebrand，确保 v2.0.0 发布时品牌一致。 (Owner: writer, Label: documentation)
- [ ] **[P1] v2.0.0 Release Notes 编写**: 编写 v2.0.0 正式 Release Notes，包含 hermesx 品牌升级、ExecutionReceipt API、Auditor 角色、Prometheus 业务指标等变更。 (Owner: writer, Label: documentation)

### P2 - 应该做

- [ ] **[P2] SelfImprover 接入 Agent 循环**: 将 SelfImprover 集成到 Agent 对话循环中，使其在对话过程中能自我改进，而非仅独立正确。 (Owner: backend-engineer, Label: enhancement)
- [ ] **[P2] prompt sanitization 一致性**: 对齐 compress.go 和 curator.go 中的 LLM prompt 处理逻辑，确保所有用户数据进入 prompt 前都经过一致的 sanitize 流程。 (Owner: backend-engineer, Label: security)
- [ ] **[P2] payload.URL 路径遍历检查扩展**: 扩展 payload.URL 的路径遍历检查，覆盖更多的 URL 类型和协议。 (Owner: backend-engineer, Label: security)
- [ ] **[P2] ExecutionReceipt API 文档完善**: 补充 ExecutionReceipt 的集成示例、幂等性行为说明、idempotency_id 使用方式，以及与审计日志的关系说明。 (Owner: writer, Label: documentation)
- [ ] **[P2] API Reference 与代码端点对齐**: 将 Sessions API、Memories API、ExecutionReceipts API、Skills 单独 GET 端点、Agent Chat 别名、GDPR cleanup-minio 等缺失端点完整记录到 api-reference.md。 (Owner: writer, Label: documentation)

### P3 - 可以做

- [ ] **[P3] Curator O(n²) dedup 优化**: 当 MaxMemories > 100 时，当前 dedup 逻辑存在 O(n²) 性能问题，需优化为 O(n) 或 O(n log n)。 (Owner: backend-engineer, Label: performance)
- [ ] **[P3] RLS SELECT policies 评估**: 对已部署的 RLS SELECT 策略进行系统性评估，确认读隔离边界是否正确覆盖所有查询路径。 (Owner: architect, Label: security)
- [ ] **[P3] pgxmock 引入**: 在 store 层测试中引入 pgxmock，消除对真实 PostgreSQL 的依赖，提升单元测试速度和覆盖率。 (Owner: backend-engineer, Label: testing)
- [ ] **[P3] CORS 动态管理**: 将 CORS 配置从环境变量扩展为可从数据库或配置中心动态加载，支持多域名按租户配置。 (Owner: backend-engineer, Label: enhancement)
- [ ] **[P3] LocalDualLimiter 多副本精确性优化**: 在 Redis 频繁故障时，多副本部署下的 LocalDualLimiter 精确性不足，需设计更优的分布式协调方案。 (Owner: backend-engineer, Label: performance)
- [ ] **[P3] HasScope empty scopes 放行修复**: 修复 HasScope 在空 scopes 列表时的错误放行行为，确保空 scopes 严格拒绝无 scope 的请求。 (Owner: backend-engineer, Label: bug)
- [ ] **[P3] GHA actions digest-pin**: 将 GitHub Actions workflow 中所有第三方 action 替换为带 commit SHA 的 digest-pin 形式，避免供应链风险。 (Owner: devops-engineer, Label: security)

### P4 - 长远规划

- [ ] **[P4] Unicode bidi chars sanitization**: 在 LLM 安全要求升级后，对所有进入 prompt 的文本执行 Unicode Bidi 字符清理，防止 Unicode 文本混淆攻击。 (Owner: backend-engineer, Label: security)
- [ ] **[P4] Admin UI for pricing rules**: 产品需求确认后，开发定价规则管理的前端界面。 (Owner: frontend-engineer, Label: feature)
- [ ] **[P4] 用量 dashboard / 计费报告**: 产品需求确认后，开发用量统计和计费报告前端界面。 (Owner: frontend-engineer, Label: feature)
- [ ] **[P4] GDPR 自助数据导出 UI**: 合规需求明确后，开发 GDPR 自助数据导出前端界面，支持租户管理员自主导出数据。 (Owner: frontend-engineer, Label: feature)
