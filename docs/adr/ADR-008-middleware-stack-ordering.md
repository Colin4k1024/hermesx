# ADR-008: HTTP Middleware Stack 固定顺序

## 决策信息

| 项目 | 内容 |
|------|------|
| 编号 | ADR-008 |
| 标题 | HTTP Middleware Stack 固定顺序及其设计原因 |
| 状态 | Accepted |
| 日期 | 2026-07-03 |
| Owner | tech-lead |
| 关联需求 | enterprise-architecture-governance |

---

## 背景与约束

HermesX 的 HTTP 入口使用 9 层中间件栈（`internal/middleware/chain.go`），执行顺序为：

```
Tracing → Metrics → RequestID → Auth → Tenant → Logging → Audit → CSRF → RBAC → RateLimit → Handler
```

此顺序是固定的，所有 HTTP Server（SaaS API、ACP）共享同一配置。此前缺少对"为什么是这个顺序"的架构记录，导致维护者在扩展或调整时缺乏判断依据。

---

## 备选方案

### 方案 A：固定顺序 + 文档化（采用）

- 所有服务共享同一栈定义，层序由代码固定
- 通过 `StackConfig` 中的 nil slot 跳过不需要的层
- 顺序变更必须更新本 ADR

### 方案 B：可配置顺序

- 允许通过配置文件自定义中间件排列
- 灵活但容易因错误排列引入安全漏洞（如 Auth 在 RBAC 之后）

### 方案 C：分组嵌套

- 将中间件分为"基础设施层"和"安全层"两组
- 组内可配，组间固定
- 增加复杂度，当前规模不需要

---

## 决策结果

采用 **方案 A**。固定顺序由以下依赖关系决定：

| 层 | 位置 | 依赖说明 |
|----|------|----------|
| Tracing | 最外层 | 覆盖整个请求生命周期，包括所有内层产生的 span |
| Metrics | 2 | 需要统计包含认证失败在内的所有请求，不能在 Auth 之后 |
| RequestID | 3 | 为后续所有层提供 request_id，Auth 错误日志也需要它 |
| Auth | 4 | 提取身份信息，后续层依赖 `auth.IdentityFromContext` |
| Tenant | 5 | 依赖 Auth 提取的身份确定租户，注入 tenant_id 到 context |
| Logging | 6 | 在 Auth + Tenant 之后，日志中可携带 tenant_id 和 user_id |
| Audit | 7 | 在认证之后记录已认证请求，减少噪音 |
| CSRF | 8 | 在 Auth 之后验证 token，避免对未认证请求做无效检查 |
| RBAC | 9 | 依赖 Auth 提供角色、Tenant 提供权限范围 |
| RateLimit | 10（最内层） | 依赖 Tenant 确定限额，只对已认证请求计数 |

### 关键设计决策

1. **Metrics 在 Auth 之前**：确保 4xx/5xx 认证错误也被统计到 Prometheus，不会产生监控盲区。
2. **Audit 在 Auth 之后**：只审计已认证的请求，减少日志量和存储成本；Auth 失败的审计由 AuthMiddleware 内部处理。
3. **RateLimit 在最内层**：限流依赖 tenant_id 和 user_id 做双层限额，必须在身份和租户解析之后。
4. **Logging 在 Tenant 之后**：结构化日志需要 tenant_id 字段进行关联查询。

---

## 企业内控补充

- 应用等级：T2（多租户 SaaS，要求高可用）
- 技术架构等级：核心入口路径
- 关键组件偏离：无，使用标准 `net/http` 中间件模式
- 资产文档入口：`internal/middleware/chain.go`

---

## 后续动作

| 动作 | Owner | 完成条件 |
|------|-------|----------|
| 中间件栈变更时同步更新本 ADR | backend-engineer | PR review 中确认 |
| 若新增中间件层，须在本 ADR 中补充依赖说明 | architect | ADR 更新后合并 |
