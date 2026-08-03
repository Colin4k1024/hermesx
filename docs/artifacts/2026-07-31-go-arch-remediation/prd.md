# PRD — Go Architecture Remediation (2026-07-31)

**slug:** go-arch-remediation  
**状态:** intake  
**主责角色:** tech-lead  
**日期:** 2026-07-31  
**来源:** 全方位 Go 架构审查（4 个并行审查子模块 + 直接静态分析）

---

## 背景

对 HermesX 项目进行全方位 Go 语言架构审查，覆盖模块结构、错误处理、接口设计、并发安全、测试覆盖、分层架构、配置安全、可观测性、API 设计共 9 个维度。审查发现 2 个严重 bug、6 个高优先级问题、6 个中优先级问题和若干低优先级改进项。

**触发原因:** 项目进入稳定运营期，需要系统性排查技术债务和运行时缺陷，确保生产环境可靠性。

**当前约束:**
- 项目为多租户 SaaS 控制平面，停机影响面大
- 部分修复（如 WaitGroup、os.Chdir）涉及运行时行为变更，需回归验证
- 测试覆盖率偏低（46%），修复时需同步补充测试

---

## 目标与成功标准

**业务目标:** 消除生产环境运行时缺陷，提升代码质量基线，降低维护成本。

**成功标准:**
1. 2 个严重 bug 全部修复并通过回归测试
2. 6 个高优先级问题全部修复
3. 现有测试全部通过（`go test ./... -race`）
4. 无新增 CRITICAL/HIGH 问题
5. 修复涉及的包补充单元测试

**非目标（显式排除）:**
- 不做大规模架构重构（如拆分 `internal/tools/` 子包）
- 不做测试覆盖率全面提升到 80%（范围过大，应单独立项）
- 不引入新的 lint 工具链或 CI 管线（属于工程基础设施范畴）
- 不修改 API 响应格式统一化（涉及前端兼容性，需独立评估）

---

## 用户故事与验收标准

### 严重 Bug（S 级）

| # | 用户故事 | 验收标准 | 涉及文件 |
|---|---------|---------|---------|
| S1 | 作为运维人员，我希望 scheduler 能正常优雅停机，等待所有 goroutine 退出 | `SaasScheduler.Stop()` 等待 `pollLoop` 退出后再返回（当前 `wg.Wait()` 因未调用 `wg.Add(1)` 而立即返回，pollLoop goroutine 泄漏） | `internal/scheduler/scheduler.go` |
| S2 | 作为运维人员，我希望并发 cron job 不会互相干扰工作目录 | 并发执行不同 `Workdir` 的 job 时，每个 job 使用独立工作目录，不互相影响 CWD | `internal/cron/scheduler.go` |

### 高优先级（A 级）

| # | 用户故事 | 验收标准 | 涉及文件 |
|---|---------|---------|---------|
| A1 | 作为开发者，我希望库代码不直接终止进程 | `internal/api/server.go` 中 CORS 校验失败时返回错误，由调用方决定是否退出 | `internal/api/server.go` |
| A2 | 作为开发者，我希望 Redis 限流器有超时保护且不阻塞请求处理 | `redis_ratelimiter.go` 的 `Allow()` 方法使用带超时的独立 context（`context.WithTimeout(context.Background(), 5s)`），而非裸 `context.Background()` | `internal/middleware/redis_ratelimiter.go` |
| A3 | 作为开发者，我希望错误链完整可追溯 | 所有 `fmt.Errorf` 使用 `%w` 包装错误，`errors.Is()` / `errors.As()` 可穿透 | 7 处（docgen_sandbox.go, approval.go, jwt.go, factory.go, credential_rotation.go） |
| A4 | 作为安全工程师，我希望配置保存不会泄露密钥到磁盘 | `Config.Save()` 在序列化前脱敏 `APIKey`、`AccessKey`、`SecretKey` 等敏感字段 | `internal/config/config.go` |

### 中优先级（B 级）

| # | 用户故事 | 验收标准 | 涉及文件 |
|---|---------|---------|---------|
| B1 | 作为 API 消费者，我希望所有端点返回一致的错误格式 | 主 API 和 admin API 统一使用 JSON 错误响应 | `internal/api/*.go` |
| B2 | 作为运维人员，我希望 batch/cron 的 LLM 调用有超时保护 | batch runner 和 cron scheduler 的 LLM 调用使用带超时的 context | `internal/batch/runner.go`, `internal/cron/scheduler.go` |
| B3 | 作为开发者，我希望 `store/types.go` 按领域组织 | 类型定义按域拆分为 `types_identity.go`、`types_workflow.go` 等 | `internal/store/types.go` |
| B4 | 作为开发者，我希望 upsert 错误不被静默忽略 | `mcpcatalog/catalog.go:84` 至少记录 warn 日志 | `internal/mcpcatalog/catalog.go` |
| B5 | 作为安全工程师，我希望所有 JSON body 解析有大小限制 | 所有 `json.NewDecoder(r.Body)` 路径加上 `MaxBytesReader` | `internal/api/tenants.go` 等 |
| B6 | 作为开发者，我希望启动时校验必要配置 | SaaS 模式下强制要求 `Database.URL` 存在 | `internal/config/config.go` |

---

## 范围

### In Scope

| 包/目录 | 修复项 |
|---------|--------|
| `internal/scheduler/` | S1: WaitGroup 修复 |
| `internal/cron/` | S2: os.Chdir 替换为 exec.Command |
| `internal/api/` | A1: os.Exit 改为返回错误; B1: 错误格式统一; B5: MaxBytesReader |
| `internal/middleware/` | A2: context 传播修复 |
| `internal/tools/`, `internal/auth/`, `internal/store/`, `internal/agent/` | A3: 错误包装修复 |
| `internal/config/` | A4: Save 脱敏; B6: 启动校验 |
| `internal/mcpcatalog/` | B4: 错误日志 |
| `internal/batch/` | B2: context 超时 |

### Out of Scope

- `internal/tools/` 子包拆分（49 文件重构，独立立项）
- `store/types.go` 按域拆分文件（B3 标记为低优先级，可后续处理）
- 测试覆盖率全面提升
- CI/CD 管线集成
- `nolint:errcheck` 指令清理
- `utils/` 包重命名

---

## 风险与依赖

| 风险 | 影响 | 缓解措施 | Owner |
|------|------|---------|-------|
| S1 WaitGroup 修复改变 shutdown 行为 | 现有依赖快速 shutdown 的脚本可能超时 | 补充 `scheduler_test.go` 验证 Stop 语义 | backend-engineer |
| S2 exec.Command 增加进程开销 | 高频 cron job 的 fork 成本 | 仅对有 Workdir 的 job 使用 exec.Command，无 Workdir 的保持原逻辑 | backend-engineer |
| A1 移除 os.Exit 改变启动失败行为 | 调用方需要正确处理错误 | 确保 `saas.go` 的 `RunE` 正确传播错误并退出 | backend-engineer |
| A4 Save 脱敏可能破坏配置迁移场景 | 用户依赖 Save 完整保存配置 | 保留 `SaveFull()` 方法用于明确需要的场景 | backend-engineer |
| B1 错误格式变更 | 现有客户端可能依赖纯文本错误解析 | 检查前端和 SDK 的错误处理逻辑 | frontend-engineer / backend-engineer |
| 测试覆盖率不足 | 修复引入回归 | 每个修复项同步补充对应测试 | backend-engineer |

---

## 待确认项

| # | 待确认项 | 需要谁确认 | 影响 |
|---|---------|-----------|------|
| Q1 | `saas.go` 中 `os.Exit(1)` 改为 `return error` 后，Cobra 的错误处理链是否能正确退出进程？ | backend-engineer | A1 修复方案 |
| Q2 | cron job 使用 `exec.Command` 后，Docker 容器内是否需要额外的 syscall 权限？ | devops-engineer | S2 修复方案 |
| Q3 | `Config.Save()` 脱敏后，是否有场景需要保存完整配置（含密钥）？ | tech-lead | A4 修复方案 |
| Q4 | API 错误格式从纯文本改为 JSON 是否会影响现有前端/SDK 客户端？ | frontend-engineer | B1 影响面评估 |
| Q5 | batch runner 的 LLM 超时默认值应设为多少？（建议 120s） | tech-lead | B2 参数选择 |

---

## 参与角色清单

| 角色 | 职责 | 输入缺口 |
|------|------|---------|
| **tech-lead** | intake 主责、优先级仲裁、方案评审 | 无 |
| **backend-engineer** | 实施全部修复、补充测试 | Q1, Q2, Q5 需确认后才能开始 |
| **frontend-engineer** | 评估 B1 错误格式变更对前端的影响 | 需检查 webui 错误处理逻辑 |
| **devops-engineer** | 评估 S2 对容器环境的影响 | Q2 需确认 |
| **qa-engineer** | 回归验证、race detector 测试 | 需等修复完成后执行 |

---

## 需求挑战会候选分组

### 分组 1：并发安全修复（S1 + S2）

**理由:** 两个严重 bug 都涉及 goroutine 生命周期和并发正确性，修复方案需要互相参考。

**参与角色:** backend-engineer（实施）、tech-lead（方案评审）  
**挑战焦点:**
- S1: WaitGroup 的正确使用模式——是否需要引入更优雅的 shutdown 机制（如 errgroup）？
- S2: `exec.Command` vs `syscall.Chroot` vs 每个 job 一个 goroutine + 独立 workdir 参数——哪种方案最小化副作用？

### 分组 2：安全与配置修复（A4 + B5 + B6）

**理由:** 都涉及配置和安全边界，修复需要统一考虑。

**参与角色:** backend-engineer（实施）、tech-lead（安全决策）  
**挑战焦点:**
- A4: `Save()` 脱敏是否需要 `SaveFull()` 逃生舱？密钥来源是否应完全从文件迁移到 env/secrets manager？
- B6: 启动校验的严格程度——fail-fast vs warn-and-continue？

### 分组 3：API 一致性修复（A1 + B1 + B4）

**理由:** 都涉及 HTTP 层行为变更，需要评估客户端兼容性。

**参与角色:** backend-engineer（实施）、frontend-engineer（影响评估）  
**挑战焦点:**
- B1: 错误格式变更是否需要版本化或渐进迁移？
- A1: `os.Exit` 移除后，Cobra 错误处理链的退出码是否保持一致？

### 分组 4：错误处理标准化（A3）

**理由:** 纯粹的代码质量修复，范围明确，风险低。

**参与角色:** backend-engineer（实施）  
**挑战焦点:** 7 处 `%v` → `%w` 的逐项确认，确保不会意外暴露内部错误类型给客户端。

---

## 企业治理待确认项

| 项目 | 当前状态 | 说明 |
|------|---------|------|
| 应用等级 | SaaS 多租户控制平面 | 影响所有租户，属于关键基础设施 |
| 数据/合规风险 | 低——本次修复不涉及数据模型变更 | A4 涉及密钥处理，但仅为脱敏而非新增存储 |
| 集团组件约束 | 未发现——项目使用开源技术栈 | 不涉及集团内部框架 |

---

## 领域技能包启用建议

| 技能 | 触发原因 | 主责角色 |
|------|---------|---------|
| `golang-patterns` | 所有 Go 代码修复遵循惯用模式 | backend-engineer |
| `golang-testing` | 修复项需同步补充测试 | backend-engineer |
| `springboot-verification` 不适用 | 项目非 Java | — |
| `karpathy-guidelines` | 保持修复范围最小化，不做顺手重构 | 全员 |

---

## 关键假设

1. **假设现有测试是可信的**——修复后通过现有测试即可确认无回归。如果现有测试本身有缺陷，需额外排查。
2. **假设生产环境使用 PostgreSQL**——S2 的 `exec.Command` 方案在 Linux 容器中可用。
3. **假设前端不依赖纯文本错误格式**——需要 frontend-engineer 确认。
4. **假设本次修复不触发 API 版本变更**——所有修复都是内部行为修正，不改变公开 API 契约。

---

## 最小可行范围（Karpathy 收敛）

如果时间紧迫，**最小闭环**为：

1. **S1 + S2**（严重 bug，影响生产可靠性）
2. **A1 + A2 + A3**（高优先级代码质量问题，修复成本低）
3. **A4**（安全风险，密钥泄露到磁盘）

B 级问题可排入下一迭代。

---

**下一步:** 进入 `/team-plan`，完成需求挑战会与设计收口。
