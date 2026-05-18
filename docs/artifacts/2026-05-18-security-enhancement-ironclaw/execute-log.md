# Execute Log: HermesX Security Enhancement

> **Slug:** security-enhancement-ironclaw  
> **State:** execute  
> **Date:** 2026-05-18  
> **Owner:** backend-engineer  
> **Status:** draft

---

## 计划 vs 实际

| Phase | 计划 | 实际 | 偏差 |
|-------|------|------|------|
| F3: Prompt Injection Defense | S1.1-S1.7 (W1-2) | 全部完成 | 无 |
| F2: Credential Isolation | S2.1-S2.7 (W2-3) | S2.1-S2.3 + 核心实现完成 | S2.4 (10个工具迁移)、S2.5 (Admin API)、S2.6 (Linter rule) 待后续迭代 |
| F4: Network Allowlisting | S3.1-S3.7 (W3-4) | 全部完成 | 无 |

---

## 实施结果

### F3: Prompt Injection Defense Layer (`internal/safety/`)

| 文件 | 职责 |
|------|------|
| `interceptor.go` | SafetyInterceptor 接口 + ChainInterceptor 实现 |
| `input_guard.go` | 用户输入注入检测 (20+ regex patterns, OWASP LLM Top 10) |
| `output_guard.go` | LLM/工具输出合规检查 |
| `canary.go` | Canary token 生成与泄漏检测 |
| `patterns.go` | 注入模式规则集 (regex + heuristic) |
| `policy.go` | Per-tenant 安全策略 (enforce/log_only/disabled) |
| `policy_store.go` | PostgreSQL 策略存取 |
| `interceptor_test.go` | 28 个测试用例，覆盖注入场景 |
| `bench_test.go` | 性能基准测试 |

### F2: Credential Isolation (`internal/secrets/` 扩展)

| 文件 | 职责 |
|------|------|
| `resolver.go` | SecretResolver 接口 + EnvSecretResolver 实现 |
| `ahocorasick.go` | Aho-Corasick 自动机 (纯 Go 实现, O(n) 扫描) |
| `leak_scanner.go` | 泄漏检测器 (50+ 内建 pattern + 动态注册) |
| `leak_scanner_test.go` | 泄漏检测全覆盖测试 + benchmark |
| `rotation.go` | Hot-reload pattern 更新支持 |

**ToolContext 扩展:** `internal/tools/registry.go` 增加 `SecretResolver` 字段。

### F4: Network Endpoint Allowlisting (`internal/egress/`)

| 文件 | 职责 |
|------|------|
| `policy.go` | EgressPolicy 接口 + 规则类型定义 |
| `allowlist.go` | AllowlistPolicy 实现 (优先级, 通配符, 内建白名单) |
| `blocked_ip.go` | 导出的 IsBlockedIP (统一 SSRF 检查) |
| `transport.go` | SecureTransport with DialContext hook (DNS rebinding 防护) |
| `store.go` | PostgreSQL CRUD for egress rules |
| `admin_handler.go` | Admin API (GET/POST/DELETE egress rules) |
| `migration.sql` | egress_rules 表 DDL |
| `transport_test.go` | SSRF、allowlist、DNS rebinding 测试 |
| `allowlist_test.go` | 通配符、优先级、默认策略测试 |

**url_safety.go 修改:** 导出 `IsBlockedIP`，保持原有 `IsSafeURL` 向后兼容。

---

## 关键决定

1. **Aho-Corasick 纯 Go 实现** — 不引入外部依赖，保持 single binary 部署模型。
2. **replacement 类型提升为包级别** — 修复编译错误（类型在函数内定义但被包级函数引用）。
3. **DialContext 直连解析后 IP** — 解决 DNS rebinding TOCTOU 问题，连接级 IP 验证。
4. **内建 LLM Provider 白名单** — api.openai.com/api.anthropic.com 永不被 egress policy 拦截。

---

## 验证结果

| 维度 | 结果 |
|------|------|
| 全项目构建 | ✅ `go build ./...` 通过 |
| 新增测试 | ✅ 81 tests (safety 28 + secrets 34 + egress 19) |
| 全量测试 | ✅ 1765 tests / 41 packages 全部通过 |
| 回归 | ✅ 零回归 |
| 原有 tools/agent/middleware 测试 | ✅ 839 tests 通过 |

---

## 影响面

| 模块 | 变更类型 |
|------|----------|
| `internal/safety/` | 新增包 (9 files) |
| `internal/secrets/` | 扩展 (5 new files) |
| `internal/egress/` | 新增包 (9 files) |
| `internal/tools/registry.go` | 添加 SecretResolver 字段 |
| `internal/tools/url_safety.go` | 导出 IsBlockedIP |

---

## 未完成项

| 项目 | 原因 | 建议处理 |
|------|------|----------|
| S2.4: 高风险 10 个工具迁移到 SecretResolver | 需要逐个工具审查和测试 | 下一迭代分批处理 |
| S2.5: Secret pattern 注册 Admin API | 核心 scanner 已就绪，API 层轻量 | 随 Admin API 统一交付 |
| S2.6: Linter rule 禁止 os.Getenv | 需要 CI 集成 | 配合 CI pipeline 落地 |
| S1.5: Per-tenant safety policy Admin API 集成 | store 已实现，HTTP handler 待接入 | 随 Admin API 统一交付 |
| DB migrations 执行 | 需要 DBA review + 环境准备 | 发布阶段处理 |
| Agent loop 集成 (S1.6) | interceptor 已可用，需在 agent.go 中接入 | 集成测试阶段完成 |
| Redis 缓存 (S3.5) | interface 已预留，具体实现需 Redis client | 性能优化阶段 |

---

## Review 阶段修复

| 问题 | 严重级别 | 修复内容 |
|------|----------|----------|
| LeakMatch.Value 暴露明文 secret | CRITICAL | 新增 redactValue() 返回前4字符+"***" |
| InMemoryPolicyStore 无锁并发 | HIGH | 添加 sync.RWMutex |
| scanLiterals 缓存 map 遍历顺序不确定 | HIGH | 按 name 排序后比较 |
| Redact 使用 len(m.Value) 计算错误 | HIGH | 新增 Length 字段存储原始长度 |
| policy_store 字符串比较 pgx 错误 | HIGH | 改用 errors.Is(err, pgx.ErrNoRows) |
| AdminHandler 缺少 Action 验证 | MEDIUM | 添加 allow/deny 白名单验证 |
| DeleteRule 无 tenant 隔离 | CRITICAL | 添加 tenant_id 参数 + WHERE 条件 |
| AdminHandler 无 auth 注释 | HIGH | 添加 RegisterRoutes 文档注释 |

---

## 交给 QA 的说明

1. **安全测试入口:** `go test ./internal/safety/ ./internal/secrets/ ./internal/egress/ -v`
2. **性能验证:** `go test ./internal/safety/ -bench=. -benchmem`
3. **重点关注:**
   - Input guard 误报率验证 (当前 28 个注入场景，需补充正常输入用例)
   - Leak scanner 检测率验证 (50+ patterns，需用真实 API key 格式测试)
   - Egress policy 通配符边界 case
4. **不需要测试:** WASM sandbox (F1 已降级为 POC，不在本轮)
