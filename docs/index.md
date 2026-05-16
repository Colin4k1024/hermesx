# HermesX

**企业级 Agent 运行时 & 多租户 SaaS 控制平面**

面向企业规模的 AI Agent 部署、隔离和治理的生产级平台。使用 Go 构建，单二进制部署、原生并发、零依赖分发。

---

## 快速链接

| | |
|---|---|
| [SaaS 快速开始](saas-quickstart.md) | 几分钟内启动一个租户 |
| [API 参考](api-reference.md) | 完整端点文档 |
| [架构概览](architecture.md) | 系统设计与组件地图 |
| [配置说明](configuration.md) | 所有环境变量与配置项 |
| [部署指南](deployment.md) | Docker、Kubernetes 及裸机部署 |

---

## 项目数据

| 指标 | 数值 |
|------|------|
| Go 源文件 | 413 个 |
| 代码行数 | 78,000+ 行 |
| 注册工具 | 50 个（36 核心 + 14 扩展） |
| 平台适配器 | 15 个 |
| 终端后端 | 7 个 |
| 内置技能 | 126 个 |
| 测试文件 | 127 个 |
| 测试总数 | 1,597 个 |
| RLS 保护表 | 10 个 |
| API 端点 | 33+ 个 |
| 版本 | v2.2.0 |

---

## 核心能力

### 企业 SaaS 平台

- **多租户隔离** — PostgreSQL 行级安全（RLS），每事务 `SET LOCAL app.current_tenant`
- **认证链** — 静态 Token → API Key（SHA-256 哈希）→ JWT/OIDC
- **5 种角色** — `super_admin`、`admin`、`owner`、`user`、`auditor`
- **双层限流** — 原子 Redis Lua 脚本 + 本地 LRU 降级
- **审计追踪** — 所有状态变更操作的不可变日志
- **GDPR 合规** — 全链路租户数据导出 + 事务性删除
- **沙箱隔离** — 按租户的代码执行环境，Docker 网络/资源限制

### Agent 运行时

- **50 个工具** — 终端、文件、网络搜索/爬取、浏览器、视觉、图像生成、TTS、代码执行、子 Agent、MCP 等
- **15 个平台适配器** — Telegram、Discord、Slack、WhatsApp、Signal、邮件、Matrix、钉钉、飞书、企业微信等
- **双 API 支持** — OpenAI 兼容接口 + Anthropic Messages API（含提示缓存）
- **LLM 弹性** — FallbackRouter + RetryTransport（指数退避）+ 熔断器（按模型独立）
- **技能系统** — YAML/Markdown 文件的过程记忆 + Hub 搜索安装

### 基础设施

- **单二进制** — 零运行时依赖，可交叉编译至任意 OS/架构
- **多副本就绪** — 已验证 3 副本 + Nginx `ip_hash` 负载均衡
- **Kubernetes 就绪** — 含 PDB、HPA、保守缩容策略的 Helm Chart
- **可观测性** — Prometheus 指标、OpenTelemetry 链路追踪、结构化 JSON 日志

---

## v2.3 新增能力（7 大架构增强）

### 1. Extended Thinking API（扩展思维）

为 Claude 模型接入深度推理能力。通过 `ReasoningConfig` 将思考预算注入 Anthropic 请求体，支持 5 级精度控制：

| 级别 | Token 预算 | 适用场景 |
|------|-----------|---------|
| `minimal` | 1,024 | 简单分类、格式转换 |
| `low` | 2,048 | 常规问答、轻量推理 |
| `medium` | 4,096 | 多步推理、代码生成（默认） |
| `high` | 10,000 | 复杂架构设计、长链推导 |
| `xhigh` | 32,000 | 极端复杂场景、研究级推理 |

启用后 `max_tokens` 自动调整为 `budget_tokens + output_tokens`，确保思考与输出不互相挤占空间。

**配置方式：**

```yaml
# .hermes/config.yaml
reasoning: high
```

---

### 2. Model Aliases（模型别名）

用简短的人类可读名称替代冗长的模型标识符，降低配置负担：

| 别名 | 解析到 |
|------|--------|
| `opus` | `anthropic/claude-opus-4-20250514` |
| `sonnet` | `anthropic/claude-sonnet-4-20250514` |
| `haiku` | `anthropic/claude-haiku-4-20250414` |
| `gpt4o` | `openai/gpt-4o` |
| `o3` | `openai/o3` |
| `flash` | `google/gemini-2.5-flash` |
| `gemini` | `google/gemini-2.5-pro` |
| `r1` | `deepseek/deepseek-r1` |
| `llama` | `meta-llama/llama-4-maverick` |

别名解析不区分大小写，自动 trim 空格。未识别的名称直接透传，兼容自定义端点。

**使用示例：**

```yaml
model: opus          # 等效于 anthropic/claude-opus-4-20250514
```

---

### 3. Project-Scoped Config（项目级配置）

自动发现项目根目录（git root 或含 `.hermes/` 的目录）下的 `.hermes/config.yaml`，实现项目粒度的行为定制。

**安全设计：**

- 仅允许安全字段覆盖：`model`、`max_iterations`、`max_tokens`、`reasoning`、`toolsets`、`plugins`、`cache` 等
- 自动消毒敏感字段：`api_key`、`database`、`redis`、`objstore`、`provider`、`base_url` 一律清空
- 项目配置可安全提交到版本控制，不会泄漏凭据

**优先级（从低到高）：**

```
全局默认 → ~/.hermes/config.yaml → {project}/.hermes/config.yaml → 环境变量 → CLI 参数
```

---

### 4. Declarative Permission Policies（声明式权限策略）

基于 YAML 的工具级访问控制，支持 `allow` / `deny` / `ask` 三种动作，集成到 `CheckDangerousCommand` 流程：

```yaml
# .hermes/permissions.yaml
default: ask
rules:
  - tool: terminal
    action: deny
    commands: ["rm -rf *", "DROP TABLE*"]
    reason: "禁止破坏性命令"
  - tool: file_write
    action: allow
    paths: ["src/**", "tests/**"]
    reason: "允许写入源代码和测试目录"
  - tool: browser
    action: ask
    reason: "浏览器操作需人工确认"
```

**分层加载：**

1. 用户级：`~/.hermes/permissions.yaml`（全局基线）
2. 项目级：`{project}/.hermes/permissions.yaml`（覆盖用户级）

支持 glob 模式匹配路径和命令，`*` 通配符匹配所有工具。

---

### 5. Structured Compaction（结构化上下文压缩）

在长对话的上下文压缩过程中，通过 **Tool Spine** 机制保留工具调用的结构化摘要：

**工作原理：**

1. 从被压缩的消息中提取所有工具调用的结果
2. 为每次调用生成三元组：`(工具名, 成功/失败, 一行关键结果)`
3. 将 Tool Spine 附加到压缩摘要中

**输出格式：**

```
### Tool Call History
1. terminal [ok]: go test ./... passed (127 tests)
2. file_write [ok]: success
3. grep [ok]: 15 results
4. terminal [FAIL]: exit code 1; package not found
```

确保即使对话被大幅压缩，Agent 仍能追溯"做了什么、结果如何"，避免重复已失败的操作或遗忘已完成的步骤。

---

### 6. OAuth Device Flow（设备授权流）

实现 RFC 8628 标准的 OAuth 2.0 设备授权流，支持无浏览器环境下的 Anthropic 账号登录：

**流程：**

```
1. CLI 请求设备码 → Anthropic 返回 user_code + verification_uri
2. 用户在浏览器中打开 URL 并输入验证码
3. CLI 轮询 token 端点（间隔 5s）
4. 获取 access_token + refresh_token → 持久化到 ~/.hermes/anthropic.json
5. Token 过期前 30s 自动刷新
```

**特性：**

- Token 安全持久化（文件权限 `0600`）
- 自动刷新，无需重复登录
- `ResolveAnthropicAPIKey` 辅助函数：优先环境变量 → 回退到 OAuth token
- 支持自定义 OAuth 端点（用于私有部署）

---

### 7. MCP Auto-Reconnect（MCP 自动重连）

为 SSE 传输层的 MCP 服务器连接提供生产级可靠性保障：

**健康监测：**

- 每 30 秒发送 JSON-RPC `ping`，主动探测连接健康
- 监听 SSE 流关闭事件，第一时间感知断连

**重连策略（指数退避 + 抖动）：**

| 参数 | 值 |
|------|-----|
| 初始延迟 | 1 秒 |
| 退避因子 | 2.0x |
| 最大延迟 | 30 秒 |
| 最大重试 | 10 次 |
| 抖动范围 | ±25% |

**重连后自动恢复：**

- 自动重新执行 `tools/list` 刷新工具定义
- 注销旧工具 → 注册新工具，确保 Registry 与服务端一致
- Prometheus 指标 `mcp_server_reconnects_total` 记录每个服务器的重连次数

**容错设计：**

- 单次工具调用失败时也会尝试即时重连 + 重试
- `tools/list_changed` 通知触发工具列表热刷新
- 连接彻底丢失后 placeholder 工具返回友好的错误指引

---

## 安装

```bash
# 从源码编译（需要 Go 1.23+）
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx
go build -o hermesx ./cmd/hermesx/
```

完整部署流程请参阅 [SaaS 快速开始](saas-quickstart.md)。
