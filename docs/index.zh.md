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
| 测试文件 | 123 个 |
| 测试总数 | 1,585 个 |
| RLS 保护表 | 10 个 |
| API 端点 | 22+ 个 |
| 版本 | v2.1.1 |

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

## 安装

```bash
# 从源码编译（需要 Go 1.23+）
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx
go build -o hermesx ./cmd/hermesx/
```

完整部署流程请参阅 [SaaS 快速开始](saas-quickstart.md)。
