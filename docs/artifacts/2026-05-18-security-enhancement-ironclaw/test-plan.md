# Test Plan: HermesX Security Enhancement

> **Slug:** security-enhancement-ironclaw  
> **State:** review  
> **Date:** 2026-05-18  
> **Owner:** qa-engineer  
> **Status:** draft

---

## 测试范围

### 功能范围

| Feature | 覆盖内容 |
|---------|----------|
| F3: Prompt Injection Defense | InputGuard (27 patterns), OutputGuard (系统提示泄漏), CanaryDetector (token 注入/检测), InterceptorChain, Per-tenant Policy |
| F2: Credential Isolation | SecretResolver, Aho-Corasick 自动机, LeakScanner (50+ patterns + literal), PatternWatcher (hot-reload), Redact |
| F4: Network Allowlisting | AllowlistPolicy (优先级/通配符/内建白名单), SecureTransport (DNS rebinding), IsBlockedIP (SSRF), AdminHandler (CRUD + validation) |

### 非功能范围

- P99 overhead < 50ms (safety interceptor benchmark)
- Aho-Corasick 线性扫描 (O(n) 不随 pattern 数增长)
- 并发安全 (InMemoryPolicyStore, CanaryDetector, AllowlistPolicy)
- 零回归 (1765 existing tests)

### 不覆盖项

- WASM Sandbox (F1, 降级为 POC，不在本轮)
- 工具层 HTTP client 迁移到 SecureTransport (C3, 下一迭代)
- ResolvedValues 接口限制 (H6, 设计决策待下一迭代)
- Redis 缓存层 (S3.5, 性能优化阶段)

---

## 测试矩阵

### F3: Prompt Injection Defense

| 场景 | 类型 | 前置条件 | 预期结果 |
|------|------|----------|----------|
| 已知注入模式检测 (ignore_previous, system_prompt_request, etc.) | Unit | InputGuard 实例化 | 返回 matches, severity >= medium |
| 无害输入不误报 | Unit | 正常文本 | 无 matches |
| 输出包含系统提示片段 | Unit | OutputGuard + system_prompt | 检测到泄漏 |
| Canary token 生成唯一性 | Unit | 多次调用 GenerateToken | 不重复 |
| Canary token 泄漏检测 | Unit | 输出包含 canary | Detect 返回 match |
| InterceptorChain 多拦截器链 | Unit | 2+ interceptors | 聚合所有 matches |
| Policy mode=enforce 时阻断 | Unit | ModeEnforce | CheckInput 返回 error |
| Policy mode=log_only 不阻断 | Unit | ModeLogOnly | 仅记录，不返回 error |
| Policy mode=disabled 跳过 | Unit | ModeDisabled | 跳过检查 |
| PostgresPolicyStore 读写 | Integration | PG 连接 | CRUD 正确 |
| 并发 GetPolicy/UpsertPolicy | Race | go test -race | 无 race |

### F2: Credential Isolation

| 场景 | 类型 | 前置条件 | 预期结果 |
|------|------|----------|----------|
| AWS key 检测 (AKIA...) | Unit | LeakScanner | match, severity=critical |
| GitHub token 检测 (ghp_...) | Unit | LeakScanner | match, severity=critical |
| JWT 检测 (eyJ...) | Unit | LeakScanner | match, severity=high |
| 50+ builtin patterns 覆盖 | Unit | 各 pattern 样本 | 全部命中 |
| Literal secret 检测 (Aho-Corasick) | Unit | SetLiteralSecrets | 检测到明文泄漏 |
| 短 secret (< 4 chars) 忽略 | Unit | SetLiteralSecrets("abc") | 不检测 |
| Redact 正确替换 | Unit | 含 secret 文本 | 替换为 [REDACTED:name] |
| redactValue 不暴露原文 | Unit | LeakMatch.Value | 仅前4字符 + "***" |
| 重叠 match 合并 | Unit | 两个 pattern 同位置命中 | mergeOverlapping 正确 |
| PatternWatcher 热加载 | Unit | Start + source 更新 | scanner patterns 增长 |
| ForceReload 重置 | Unit | ForceReload | 仅保留 builtin + source |
| scanLiterals 缓存一致性 | Unit | 同内容不同 map 遍历序 | 缓存复用 |
| 并发 Scan + SetLiteralSecrets | Race | go test -race | 无 race |

### F4: Network Allowlisting

| 场景 | 类型 | 前置条件 | 预期结果 |
|------|------|----------|----------|
| 私有 IP 阻断 (127.0.0.1, 10.x, 192.168.x) | Unit | IsBlockedIP | true |
| CGNAT 阻断 (100.64.x.x) | Unit | IsBlockedIP | true |
| 公网 IP 放行 | Unit | IsBlockedIP | false |
| 内建白名单 (api.openai.com) 永远放行 | Unit | AllowlistPolicy | allowed=true |
| 通配符匹配 (*.example.com) | Unit | AllowlistPolicy | 子域匹配 |
| 优先级排序 (deny > allow) | Unit | AllowlistPolicy | 高优先级规则生效 |
| DefaultDenyAll 未知 host 拒绝 | Unit | AllowlistPolicy | allowed=false |
| SecureTransport DNS rebinding 防护 | Unit | mock resolver 返回私有IP | ErrBlockedIP |
| SecureTransport 允许公网连接 | Unit | mock resolver 返回公网IP | 连接成功 |
| AdminHandler createRule 验证 | Unit | 无效 action | 400 |
| AdminHandler deleteRule tenant_id 必须 | Unit | 缺少 tenant_id | 400 |
| AdminHandler CRUD 流程 | Integration | PG 连接 | 完整 CRUD |

---

## 风险

### 高风险路径

| 风险 | 影响 | 缓解 |
|------|------|------|
| C3: 工具 HTTP client 未接入 SecureTransport | F4 对工具层无效 | 下一迭代逐个工具迁移，当前 AdminHandler 有 auth 注释 |
| C4: HTTP redirect 可绕过 IP 检查 | SSRF via redirect | 下一迭代加 CheckRedirect |
| M3: Unicode homoglyph 绕过注入检测 | false negative | 下一迭代加 NFKC normalization |
| M2: 4 个过宽 pattern 产生误报 | 告警疲劳 | 标记为 SeverityLow, 后续收紧 |
| H5: Canary token map 无限增长 | 内存泄漏 | 下一迭代加 TTL 清理 |

### 回归关注点

- `internal/tools/registry.go` 新增 SecretResolver 字段，需确认所有工具 handler 不受影响
- `internal/tools/url_safety.go` 导出 IsBlockedIP，原有 IsSafeURL 保持不变

---

## 放行建议

### 建议：**有条件放行**

**已满足：**
- 全项目构建通过
- 1765 tests / 41 packages 全部通过，零回归
- 81 新增测试覆盖 3 个安全包
- CRITICAL bug (LeakMatch plaintext) 已修复
- HIGH (InMemoryPolicyStore race, scanLiterals cache, Redact length) 已修复
- HIGH (AdminHandler action validation, DeleteRule tenant isolation) 已修复
- HIGH (policy_store string comparison) 已修复

**阻塞项：** 无硬阻塞

**已接受风险：**
- C3 (工具层未迁移 SecureTransport) — 下一迭代，当前 Docker sandbox + IsSafeURL 兜底
- H5 (Canary map 增长) — 短期内不会成为问题 (tokens 小对象)
- H6 (ResolvedValues 接口暴露) — 接口设计决策，待下一迭代收紧
- M1 (默认 log_only) — 有意为之，新租户 onboarding 需要观察期

**补充验证：**
- 建议对 50+ leak scanner patterns 用真实格式样本做验证 (避免过宽/过窄)
- 建议对 input_guard 补充正常输入误报率测试 (当前仅有注入场景)
