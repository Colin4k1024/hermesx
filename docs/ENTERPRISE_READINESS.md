# 企业就绪度矩阵

> HermesX - 面向 Agent 的运行时控制平面
> 当前文档/API 基线：`v2.4.0-dev`
> 最新已发布基线：`v2.3.0`
> 最后更新：2026-05-24

本文件按证据口径描述企业就绪度。"Released" 表示属于 changelog 中的最新已发布基线；"Unreleased" 表示能力存在于当前分支或 changelog 的 Unreleased 区域，但在 `v2.4.0` 正式发布前不属于最新稳定发布。

## 汇总

| 能力 | 当前状态 | 发布状态 | 证据 | 后续动作 |
|------|----------|----------|------|----------|
| 多租户隔离 | Done | Released baseline | PostgreSQL RLS migration、Tenant middleware、租户级 Store、`tests/integration/` 集成测试 | 保持真实 PostgreSQL RLS 测试进入 CI |
| Auth、API Key、RBAC | Done | Released baseline + OIDC plan | `internal/auth/`、`internal/middleware/rbac.go`、API key scopes、`RBAC_MATRIX.md`、[OIDC 集成测试计划](runbooks/enterprise-OIDC-integration-test-plan.md) | 执行 Keycloak/Auth0/local IdP E2E 并沉淀证据 |
| Agent Runtime | Done | Released baseline + Unreleased Eino path | `internal/agent/`、Chat API、tool loop、skills、memory、MCP、changelog Unreleased Eino 0.9 条目 | Eino 0.9 行为在发布前按 `v2.4.0-dev` 处理 |
| 执行回执与审计 | Done | Released baseline | Execution receipt store/API、Audit middleware/store、API 文档中的 trace 关联说明 | 如面向强监管部署，补充更长保留期与外部导出策略 |
| 工作流与人工任务 | Done | Released baseline + Unreleased Eino 默认执行器 | `internal/workflow/`、workflow stores、OpenAPI workflow paths、`workflow-guide.md` | 将 workflow Eino executor 的发布说明与稳定 workflow API 分开 |
| 沙箱隔离 | Done | Released baseline + Unreleased K8s Job mode | Local/Docker sandbox policy、租户级沙箱控制；K8s Job mode 位于 Unreleased | 生产前验证集群 RBAC、镜像策略、网络策略和资源限制 |
| Egress 控制 | Done | 当前分支 `v2.4.0-dev` | `SecureTransport`、租户 allowlist、生产 `deny-all` 默认值、`HERMES_EGRESS_DEFAULT` override | 在生产类环境补 allowlist smoke test |
| Metering 与 Usage | Partial | Released tenant usage + Unreleased admin aggregation | `usage_records`、租户 usage API、Unreleased admin aggregation | 明确 billing/invoicing 不属于当前能力边界 |
| 可观测性 | Done | Released baseline + Unreleased 预置观测包 | Prometheus metrics、OTel tracing、结构化日志、[Grafana dashboard](../deploy/grafana/dashboards/hermesx-overview.json)、[alerts](../deploy/prometheus/alerts.yml)、[Prometheus config](../deploy/prometheus/prometheus.yml) | 在 staging 导入 dashboard 并 dry-run alerts |
| 备份与灾备 | Partial | Released PG backup baseline + Unreleased Redis/MinIO scripts | PG backup/restore 文档、[scripts/dr-test.sh](../scripts/dr-test.sh)、[scripts/pitr-drill.sh](../scripts/pitr-drill.sh)、[PITR runbook](runbooks/pg-pitr-recovery.md) | 用生产级数据恢复演练记录 RTO/RPO |
| OpenAPI 契约 | Done | 当前文档/API 基线 `v2.4.0-dev` | `internal/api/openapi.go`、`GET /v1/openapi`、OpenAPI tests | 保持 `info.version` 与 README 发布状态说明一致 |
| CI 与安全门禁 | Done | Released baseline | Go tests、integration tests、race/coverage workflow、安全 workflow 文档 | 如生产策略要求，补 DAST/container runtime checks |

## 发布状态说明

| 领域 | 已发布基线（`v2.3.0`） | 当前分支（`v2.4.0-dev`） |
|------|-------------------------|---------------------------|
| API 版本 | 公共文档把 `v2.3.0` 标为最新已发布基线 | OpenAPI 报告 `2.4.0-dev`，因为它描述当前分支契约 |
| Agent Runtime | 稳定 chat/tool/memory/skill/MCP runtime | Eino 0.9 主链、checkpoint resume、agentic blocks 调试输出 |
| 运维 | Metrics、tracing、结构化日志、PG backup/restore | Grafana dashboard、Prometheus alerts、OTel collector compose、Redis/MinIO backup scripts |
| 沙箱 | Local 与 Docker sandbox | K8s Job sandbox mode |
| Admin usage | 租户 usage 可用 | 按租户聚合的 Admin usage API 未发布 |

## 已知风险

| 风险 | 严重度 | 缓解 |
|------|--------|------|
| `v2.4.0-dev` 文档可能被误解为稳定发布承诺 | Medium | README、OpenAPI、changelog 和本矩阵均区分当前分支与最新已发布基线 |
| K8s Job sandbox 需要集群策略验证 | Medium | 在 cluster RBAC、network policy、image policy、resource limit 验证前保持 Unreleased |
| OIDC E2E 尚未对外部 IdP 留痕 | Medium | 企业放行前执行 [enterprise-OIDC-integration-test-plan.md](runbooks/enterprise-OIDC-integration-test-plan.md) |
| Grafana/Prometheus 配置需要 live validation | Low | JSON/YAML 已本地检查；需要导入 staging 并记录 dashboard/alert 证据 |
| Backup/DR 在不同数据存储之间不均衡 | Medium | PG baseline 已存在；Redis/MinIO scripts 在自动恢复演练前保持 Unreleased |
| Billing 仍是用量记录，不是开票系统 | Low | 将 usage API 描述为计量/控制平面能力，而非 billing platform |
