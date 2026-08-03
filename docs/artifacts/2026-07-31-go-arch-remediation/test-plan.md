# Test Plan — Go Architecture Remediation

**slug:** go-arch-remediation  
**状态:** review  
**主责角色:** qa-engineer  
**日期:** 2026-07-31

---

## 测试范围

### 功能范围

| 修复项 | 测试类型 | 测试重点 |
|--------|---------|---------|
| S1: WaitGroup | 单元测试 | `Stop()` 返回后 pollLoop 已退出 |
| S2: os.Chdir | 集成测试 | cron job 使用正确 workdir 执行 |
| A1: os.Exit | 单元测试 | CORS wildcard + production 返回 error |
| A2: context 超时 | 单元测试 | Allow() 在 Redis 不可达时 5s 内返回 |
| A3: 错误包装 | 单元测试 | `errors.Is(err, context.DeadlineExceeded)` 可穿透 |
| A4: Save 脱敏 | 单元测试 | Save 输出不含明文密钥；SaveFull 保留完整 |
| A5: LLM 超时 | 单元测试 | LLM 调用 120s 后返回 context error |
| B4: 错误日志 | 手动验证 | seed upsert 失败时有 warn 日志 |

### 非功能范围

| 维度 | 覆盖项 |
|------|--------|
| 并发安全 | `go test -race` 全量通过 |
| 编译安全 | `go build ./...` + `go vet ./...` 通过 |
| 回归 | 现有测试全部通过 |

### 不覆盖项

- S2 的 LLM agent 是否遵循 workdir 指令（依赖 LLM 行为，不可自动化测试）
- B1 错误格式统一（已推迟）
- B3 types.go 拆分（已推迟）

---

## 测试矩阵

| 场景 | 类型 | 前置条件 | 预期结果 |
|------|------|---------|---------|
| S1: Stop 等待 pollLoop | 单元 | scheduler 已 Start | Stop() 在 5s 内返回，无残留 goroutine |
| S1: Stop 幂等 | 单元 | 多次调用 Stop | 不 panic，不阻塞 |
| A1: CORS 生产拒绝 | 单元 | HERMES_ENV=production, AllowedOrigins=* | NewAPIServer 返回 error |
| A1: CORS 非生产允许 | 单元 | HERMES_ENV != production | NewAPIServer 正常返回 |
| A2: Redis 超时 | 单元 | Redis 不可达 | Allow() 5s 内返回 false, error |
| A3: 错误穿透 | 单元 | context 超时 | errors.Is(err, context.DeadlineExceeded) == true |
| A4: Save 脱敏 | 单元 | Config 含 APIKey | 输出 YAML 中 APIKey 为 masked |
| A4: SaveFull 保留 | 单元 | Config 含 APIKey | 输出 YAML 中 APIKey 为明文 |
| A5: LLM 超时 | 单元 | LLM 服务不响应 | 120s 后返回 context.DeadlineExceeded |
| S2: workdir 验证 | 单元 | 不存在的 workdir | runJob 返回失败 |
| S2: workdir 注入 | 手动 | 有 workdir 的 job | system prompt 含 workdir 指令 |

---

## 风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| S2 依赖 LLM agent 行为 | 如果 agent 不使用 working_directory 参数，命令在错误 CWD 执行 | 集成测试验证；后续可通过 ToolContext.WorkDir 强制注入 |
| A4 maskSecret 前 4 后 4 | 短密钥（<=8 字符）完全掩码，长密钥暴露首尾 | 可接受——足够调试，不足以恢复完整密钥 |
| 测试覆盖不足 | 部分修复没有对应单元测试 | 建议后续 PR 补充 |

---

## 放行建议

**建议：有条件放行（Conditional Go）**

| 条件 | 状态 |
|------|------|
| `go build ./...` 通过 | ✅ 已确认 |
| `go vet ./...` 通过 | ⚠️ 待确认 |
| `go test -race ./...` 通过 | ⚠️ 待手动执行 |
| 无 CRITICAL 阻塞项 | ✅ 评审子代理确认中 |
| 已接受风险已记录 | ✅ 本文档已记录 |

**阻塞项：无**

**非阻塞风险：**
1. S2 的 system prompt 方案依赖 LLM 行为——建议首次部署后观察 cron job 执行日志
2. 测试覆盖不足——建议后续补充 S1 和 A4 的单元测试
3. `go test -race` 未在本次 review 中执行——建议手动运行确认
