# Requirement Challenge Session Log

**任务:** Go 架构审查修复
**slug:** `go-arch-remediation`
**日期:** 2026-07-31
**主持人:** product-manager
**关联 PRD:** [prd.md](prd.md)
**关联 Delivery Plan:** [delivery-plan.md](delivery-plan.md)
**阶段:** `requirement-challenge`

---

## 一、上游质疑记录

### Q1: 这个需求真的需要做吗？不做最坏后果是什么？

**目标:** 整个 Go 架构审查修复任务本身

**分析:**
- **S1（WaitGroup）**: 不做的最坏后果——`pollLoop` goroutine 在 `Stop()` 后继续运行，可能导致竞态条件和资源泄漏。但经代码确认，**PRD 描述的"Stop() 永久阻塞"是错误的**。实际情况是 `wg.Add()` 从未被调用（`internal/scheduler/scheduler.go` 全文只有 `wg` 声明在 75 行和 `wg.Wait()` 在 148 行，无 `wg.Add` / `wg.Done`），所以 `wg.Wait()` 立即返回，`Stop()` 根本不阻塞。真正的问题是 **shutdown 不完整**——pollLoop 可以在 Stop() 返回后继续执行，而非永久阻塞。
- **S2（os.Chdir）**: 不做的最坏后果——并发 cron job 互相干扰工作目录，导致不可预测的文件操作。在当前单 Pod 或串行场景下可能未触发，但 SaaS 多租户模式下是真实的并发风险。
- **A1-A4**: 不做的最坏后果——代码质量债务累积，但当前无已知的生产事故。

**结论:** 任务有必要做，但**严重程度分级需要校正**——S1 的实际表现与 PRD 描述不一致，应由 tech-lead 重新评估影响等级。

**升级:** tech-lead（校正 S1 严重程度定义）

---

### Q2: 有没有比当前方案更简单的解决路径？

**目标:** 最小可行范围的 6 项修复

**分析:**
- 当前方案是"按审查报告逐项修复"，共 6 项代码变更。
- 更简单的路径：**先只做 S2（os.Chdir）**，因为这是唯一一个有明确并发安全问题且代码证据确凿的修复。其余 5 项（S1、A1-A4）要么问题描述与代码不符（S1），要么触发条件极为罕见（A1 仅在 `HERMES_ENV=production` + `AllowedOrigins=*` 时触发），要么当前已有缓解措施（A4 的 `config.yaml` 权限已是 `0600`）。
- 替代路径的成本收益分析：S2 单项修复 ~1 天，可消除唯一的并发安全实际风险；其余 5 项 ~4 天，收益主要是代码质量改进。

**结论:** 应考虑将范围拆为两个批次——**批次 1 只做 S2**（唯一的运行时并发风险），**批次 2 做 A1+A3+A4**（代码质量），S1 和 A2 需要先确认实际影响再定。

**升级:** tech-lead（批次拆分决策）

---

### Q3: 用户要的是这个具体方案，还是要解决某个底层问题？

**目标:** 需求背后的真实动机

**分析:**
- 业务方触发原因是"项目进入稳定运营期，系统性排查技术债务"。
- 但 Delivery Plan 已经对原始审查范围做了调整（B2 升 A5、B1 推迟），说明审查报告的优先级分级并不完全准确。
- 更深层的问题是：**当前测试覆盖率只有 46%，且现有测试的可信度未经验证**。PRD 假设"现有测试是可信的"，但这个假设本身就需要先验证。如果测试不可靠，6 项修复的回归验证就是空中楼阁。

**结论:** 用户要的是"确保生产环境可靠性"，而不是"修复审查报告中的每一项"。应先建立可信的测试基线（至少覆盖 scheduler、cron、config 三个核心包），再执行修复。

**升级:** tech-lead（测试基线优先级决策）

---

## 二、最小可行范围假设质疑

### 假设 1: "S1 修复是最高优先级，因为 Stop() 永久阻塞"

**质疑:** PRD 和审查报告对 S1 的描述与代码实际行为不一致。

**代码证据:**
- `internal/scheduler/scheduler.go:75`——`wg sync.WaitGroup` 声明
- `internal/scheduler/scheduler.go:148`——`s.wg.Wait()` 调用
- 全文件无 `wg.Add()` 或 `wg.Done()` 调用
- 实际行为：`wg.Wait()` 的 WaitGroup 计数器始终为 0，**立即返回**，不存在永久阻塞
- 真正的 bug：`pollLoop` goroutine（136 行）在 `Stop()` 返回后可能仍在运行，是 **shutdown 不完整**，而非 **shutdown 永久阻塞**

**影响:**
- 如果按"永久阻塞"的描述去修复，方案可能是添加 `wg.Add(1)` + `defer wg.Done()` 到 pollLoop——这确实能修复问题，但**问题定义错误会导致测试用例写错方向**
- "不永久阻塞"的测试永远通过（因为本来就不阻塞），应该测试的是"Stop() 返回后 pollLoop 确实已退出"

**建议:** 由 tech-lead 重新定义 S1 的 bug 描述和验收标准。当前 PRD 的验收标准「`Stop()` 在 `pollLoop` 退出后正常返回，不永久阻塞」技术上已经满足——需要改为「`Stop()` 返回后，`pollLoop` goroutine 确实已退出，无残留 goroutine」。

---

### 假设 2: "A4（Config.Save 密钥泄露）是安全风险，必须纳入最小范围"

**质疑:** A4 的实际风险等级被高估，且修复可能引入新问题。

**代码证据:**
- `internal/config/config.go:291-308`——`Save()` 使用 `yaml.Marshal(cfg)` 写入 `config.yaml`
- 文件权限为 `0600`（仅 owner 可读写）
- 密钥来源优先级：环境变量 > `.env` 文件 > `config.yaml`（见 `applyEnvOverrides` 208-260 行）
- 在 SaaS 部署模式下，密钥通常通过环境变量注入，`config.yaml` 中的值会被 env 覆盖

**反例分析:**
- **不做 A4 最坏影响:** 如果攻击者已获得容器文件系统访问权限（能读 `config.yaml`），那么环境变量同样可读（`/proc/1/environ`）。`config.yaml` 脱敏不改变实际攻击面。
- **做 A4 最坏影响:** `Save()` 脱敏后，如果用户依赖 `Save` + `Load` 往返保存完整配置（含密钥），会丢失密钥。Delivery Plan 提到保留 `SaveFull()` 逃生舱，但这增加了 API 复杂度。

**建议:** A4 应降级为 B 级。真正的密钥安全应通过 secrets manager 或 Kubernetes Secrets 解决，而非在应用层对配置文件做脱敏——这是治标不治本。

---

### 假设 3（附加）: "A2（context.Background 在中间件）需要修复"

**质疑:** `RedisRateLimiter.Allow()` 使用 `context.Background()` 可能是**有意为之**。

**代码证据:**
- `internal/middleware/redis_ratelimiter.go:51`——`ctx := context.Background()`
- `Allow()` 是一个原子性 Redis Lua 脚本调用，通常在微秒级别完成
- 限流检查不应被请求取消——如果请求被客户端取消，限流计数仍然应该生效，否则恶意用户可以通过快速取消请求绕过限流

**反例分析:**
- **不修复的最坏影响:** 请求取消时，限流 Redis 调用仍会完成（微秒级），不会造成资源浪费。实际上这可能是正确行为。
- **修复的最坏影响:** 如果改为使用请求 context，恶意用户可以在限流检查完成前取消请求，绕过 rate limiting。

**建议:** 需要 tech-lead 确认——`Allow()` 的 `context.Background()` 是 bug 还是 feature。不应默认替换。

---

## 三、替代路径

### 替代路径 A: 缩小范围——只做 S2，其余排入下一迭代

**理由:**
- 经代码审查，6 项中只有 S2（os.Chdir 并发不安全）有明确的运行时并发安全风险
- S1 的问题描述与代码实际行为不符，需要重新定义
- A1 触发条件极为苛刻（仅 `HERMES_ENV=production` + `AllowedOrigins=*`）
- A2 可能是正确行为
- A3 是代码质量改进，不影响运行时
- A4 风险被高估

**成本:** ~1 天（仅 S2 修复 + 测试）
**收益:** 消除唯一的实际并发安全风险

### 替代路径 B: 扩大范围——加入测试基线建设

**理由:**
- PRD 假设"现有测试可信"但未验证
- 当前覆盖率 46%，scheduler 包的测试覆盖情况未知
- 如果在不可靠的测试基线上做修复，回归风险高于不修复

**成本:** 在原计划 5-6 天基础上增加 2-3 天
**收益:** 修复有可信的回归保护

### 推荐: 替代路径 A

**理由:** Karpathy 收敛原则——只修已确认的实际问题。S1/A2 需要先澄清，A1/A3/A4 可以排入下一迭代的"代码质量改进"批次。

---

## 四、阻断条件

### 🔴 阻断项 1: S1 的 bug 定义与代码实际行为不一致

**描述:** PRD 写的验收标准是「`Stop()` 在 `pollLoop` 退出后正常返回，不永久阻塞」，但代码中 `Stop()` 从不永久阻塞（`wg.Wait()` 立即返回）。实际 bug 是 shutdown 不完整（pollLoop 可能 outlive Stop）。

**影响:** 如果按当前 PRD 实施，修复方向和测试用例可能偏离实际问题。backend-engineer 可能写出"验证 Stop() 不阻塞"的测试——这会永远通过。

**必须由:** tech-lead 重新定义 S1 的 bug 描述和验收标准
**阻断阶段:** 在进入 `/team-execute` 前必须解决

### 🔴 阻断项 2: A2 的 `context.Background()` 可能是正确行为

**描述:** Redis rate limiter 使用 `context.Background()` 可能是有意设计——限流检查不应因请求取消而被跳过。

**影响:** 如果盲目改为请求 context，可能引入安全漏洞（恶意用户通过取消请求绕过限流）。

**必须由:** tech-lead 确认 A2 的行为意图，决定是否纳入修复范围
**阻断阶段:** 在进入 `/team-execute` 前必须解决

### 🟡 阻断项 3: Delivery Plan 与 PRD 范围不一致

**描述:** PRD 定义最小可行范围为 S1+S2+A1+A2+A3+A4（6 项），但 Delivery Plan 已调整为 S1+S2+A1+A2+A3+A4+A5+B4（8 项），其中 B2 被升级为 A5。两份文档的范围定义不一致，会导致执行时的歧义。

**影响:** backend-engineer 不确定是否需要做 A5（LLM 超时）和 B4（mcpcatalog 日志）。

**必须由:** tech-lead 统一 PRD 和 Delivery Plan 的范围定义
**阻断阶段:** 在进入 `/team-execute` 前必须解决

---

## 五、待确认项汇总

| # | 待确认项 | 需要谁确认 | 影响 | 当前状态 |
|---|---------|-----------|------|---------|
| C1 | S1 的实际 bug 描述：是"永久阻塞"还是"shutdown 不完整"？ | tech-lead | 修复方向和测试用例 | **阻断** |
| C2 | A2 的 `context.Background()` 是 bug 还是 feature？ | tech-lead | 是否纳入修复范围 | **阻断** |
| C3 | PRD 与 Delivery Plan 范围统一（6 项 vs 8 项） | tech-lead | 执行范围 | **阻断** |
| C4 | A4 是否应降级为 B 级（实际风险低于描述） | tech-lead | 最小范围定义 | 待确认 |
| C5 | 测试基线是否需要先建立（46% 覆盖率是否可信） | tech-lead | 回归验证可靠性 | 待确认 |

---

## 六、结论与下一步

### 结论

1. **任务有必要做**，但当前 PRD 中至少 2 项（S1、A2）的问题描述需要校正
2. **最小可行范围假设不成立**——S1 的描述与代码不符，A2 可能是正确行为，A4 风险被高估
3. **建议缩小范围为 S2 单项**，其余在澄清后排入下一迭代
4. **存在 3 个阻断条件**，需要 tech-lead 在进入 execute 前解决

### 下一步

| 动作 | 主责 | 时间 |
|------|------|------|
| tech-lead 重新定义 S1 bug 描述和验收标准 | tech-lead | 进入 execute 前 |
| tech-lead 确认 A2 行为意图 | tech-lead | 进入 execute 前 |
| 统一 PRD 和 Delivery Plan 范围 | tech-lead | 进入 execute 前 |
| 确认是否采用"替代路径 A（只做 S2）" | tech-lead | 进入 execute 前 |

**当前阶段:** `requirement-challenge`
**目标阶段:** `handoff-ready`（阻断条件解决后）
**就绪状态:** `blocked`（3 个阻断项未解决）

---

**accepted_by:** （待 tech-lead 确认）
