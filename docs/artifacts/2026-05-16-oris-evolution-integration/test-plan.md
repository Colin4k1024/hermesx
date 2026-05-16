# Test Plan — Oris Evolution Integration

**版本：** v1.0  
**日期：** 2026-05-16  
**Slug：** oris-evolution-integration  
**主责角色：** qa-engineer  
**状态：** review  

---

## 测试范围

### In Scope

| 模块 | 覆盖内容 |
|------|----------|
| `internal/evolution/config.go` | DefaultConfig 默认值校验 |
| `internal/evolution/gene.go` | DetectTaskClass 全部 5 个 task class、工具名覆盖、空输入 |
| `internal/evolution/store.go` | SQLite open/close、Save/QueryTop、confidence 过滤、跨 class 隔离、RecordOutcome 置信度重算、目录自动创建、MySQL DSN 校验 |
| `internal/evolution/improver.go` | PreTurnEnrich（无 gene、低阈值、命中返回、上限截断）、PostTurnRecord（新 gene LLM 调用、已有 gene 跳过 LLM、LLM 错误不存 gene） |
| `internal/agent/agent.go` | pre-turn 注入路径、post-turn goroutine 触发条件 |
| `cmd/hermesx/main.go` | evolution disabled 时不注入 option |
| `cmd/hermesx/saas.go` | evolution disabled 时不挂载 Improver |

### Out of Scope（本轮）

- MySQL 集成测试（需要 Testcontainers 或真实 MySQL 实例）
- Oris SDK 内部 SQL injection 的端对端利用验证
- 跨租户 gene 污染的 E2E 场景（需要完整多租户环境）
- 性能基准（gene 查询 P99 延迟）

---

## 测试矩阵

| # | 场景 | 类型 | 前置条件 | 预期结果 | 状态 |
|---|------|------|----------|----------|------|
| T01 | DefaultConfig 返回正确默认值 | 单元 | — | Enabled=false, ReplayThreshold=0.75 | ✅ PASS |
| T02 | DetectTaskClass debug 关键词 | 单元 | first_msg 含 error/bug/crash | coding.debug | ✅ PASS |
| T03 | DetectTaskClass feature 关键词 | 单元 | first_msg 含 implement/create/add | coding.feature | ✅ PASS |
| T04 | DetectTaskClass analysis 关键词 | 单元 | first_msg 含 explain/review/analyze | analysis.code | ✅ PASS |
| T05 | DetectTaskClass writing 关键词 | 单元 | first_msg 含 document/readme/comment | writing.docs | ✅ PASS |
| T06 | DetectTaskClass general（无匹配） | 单元 | first_msg 为 "hello" | general | ✅ PASS |
| T07 | 工具名 terminal + debug keyword → coding.debug | 单元 | toolsUsed=["terminal"], msg 含 error | coding.debug | ✅ PASS |
| T08 | 工具名 write_file + 无 debug keyword → coding.feature | 单元 | toolsUsed=["write_file"], msg="hello" | coding.feature | ✅ PASS |
| T09 | 空 messages → general | 单元 | messages=nil | general | ✅ PASS |
| T10 | GeneStore open/close (SQLite) | 单元 | tempdir | 无 error | ✅ PASS |
| T11 | Save + QueryTop 返回已存 gene | 单元 | confidence=0.9 | 1 result, correct GeneID | ✅ PASS |
| T12 | QueryTop 按 confidence 过滤 | 单元 | low=0.3, high=0.9, minConf=0.75 | 仅 high 返回 | ✅ PASS |
| T13 | QueryTop 不跨 task class | 单元 | gene.TaskClass=coding.feature | analysis.code 查询返回 0 | ✅ PASS |
| T14 | RecordOutcome 重算置信度 | 单元 | 1 success 1 failure | confidence=0.5 | ✅ PASS |
| T15 | GeneStore 自动创建嵌套目录 | 单元 | nested/deep/path | 无 error | ✅ PASS |
| T16 | MySQL DSN 为空时返回 error | 单元 | StorageMode=mysql, DSN="" | error != nil | ✅ PASS |
| T17 | PreTurnEnrich 无 gene → 空结果 | 单元 | store 为空 | len=0 | ✅ PASS |
| T18 | PreTurnEnrich 低于阈值 → 空结果 | 单元 | confidence=0.6 < 0.75 | len=0 | ✅ PASS |
| T19 | PreTurnEnrich 命中返回 strategy 文本 | 单元 | confidence=0.9 | strategy text returned | ✅ PASS |
| T20 | PreTurnEnrich 不超过 MaxGenesInPrompt | 单元 | 5 genes, max=3 | len≤3 | ✅ PASS |
| T21 | PostTurnRecord 新 gene：LLM 被调用并存储 | 单元 | 空 store, mock LLM | mc.called=true, 1 gene stored | ✅ PASS |
| T22 | PostTurnRecord 已有高置信度 gene：LLM 不调用 | 单元 | pre-seeded gene, confidence=0.9 | mc.called=false, use_count 增加 | ✅ PASS |
| T23 | PostTurnRecord LLM 报错：不存 gene | 单元 | mock LLM returns error | len(results)=0 | ✅ PASS |
| T24 | keyword 优先级：writing > analysis > debug > feature | 单元 | "add comments" | writing.docs, not coding.feature | ✅ PASS |
| T25 | keyword 优先级：analysis > debug | 单元 | "review this code for issues" | analysis.code, not coding.debug | ✅ PASS |

**总计：25 / 25 PASS**

---

## 风险与高风险路径

| 风险 | 等级 | 描述 | 缓解 |
|------|------|------|------|
| Prompt injection via gene insight | **CRITICAL** | gene 内容未经清洗注入 system prompt | 必须在 PreTurnEnrich 添加 sanitizeInsight() |
| 跨租户 gene 共享 | **CRITICAL** | global shared store 无 tenant 隔离 | 必须在 merge 前解决（最小修复：per-tenant SQLite 或 TaskClass 前缀） |
| LLM 响应无校验直接存储 | **HIGH** | 恶意 LLM 响应可污染 gene store | 添加 validateInsight() + 长度上限 500 bytes |
| Oris SDK OrderBy SQL injection | **HIGH** | StoreQuery.OrderBy 未参数化 | 添加 safeOrderBy() 白名单 |
| MySQLDSN 写入 YAML | **HIGH** | 凭据暴露风险 | 添加 EVOLUTION_MYSQL_DSN env var 覆盖路径 |
| GeneStore 未 Close | **HIGH** | SQLite WAL 未刷盘，MySQL 连接池泄露 | 在 shutdown 序列注册 gs.Close() |
| messages slice 竞态 | **HIGH** | goroutine 可读到 mutated backing array | 启动 goroutine 前 copy slice |
| RecordOutcome 非原子 | **MEDIUM** | 并发 SaaS 下置信度写入丢失 | 文档标注；MySQL 路径考虑事务包裹 |
| SetEvolutionImprover 无并发保护 | **MEDIUM** | 启动竞态 | atomic.Pointer 替换裸指针 |
| SQLite 目录权限 0755 | **MEDIUM** | 多用户服务器可读取 gene 内容 | 改为 0700 + 文件 0600 |
| buildInsightPrompt 字节截断 | **MEDIUM** | 多字节字符分割产生非法 UTF-8 | 改用 []rune 截断 |
| config 结构重复 | **HIGH** | 字段漂移无编译器保护 | 直接 embed evolution.Config |
| DetectTaskClass 调用两次不一致 | **MEDIUM** | pre/post turn 分类可能不同 | 计算一次后传参 |
| 单基因 outcome 记录 | **MEDIUM** | 注入了 N 个 gene 只记录 1 个 | QueryTop limit 改为 MaxGenesInPrompt |

---

## 放行建议

**当前状态：NO-GO（不允许合并到生产）**

**阻塞项（必须修复后重评审）：**
1. Security Finding 1：`sanitizeInsight()` 未实现 — CRITICAL
2. Security Finding 2：跨租户 gene 隔离未实现 — CRITICAL
3. Code Finding 1：GeneStore 资源泄露 — HIGH
4. Security Finding 4：safeOrderBy 白名单 — HIGH

**可在后续 PR 修复（不阻塞 merge 到 dev/feature 分支）：**
- Security Finding 3：validateInsight 长度校验
- Code Finding 2：messages slice 防御性 copy
- Code Finding 3：config struct 重复合并
- 其余 MEDIUM/LOW 条目
