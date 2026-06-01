# Security Policy / 安全政策

## English

HermesX is an agent runtime control plane. Security reports may involve tenant isolation, authentication, RBAC, sandbox execution, egress controls, workflow execution, or supply-chain configuration.

### Supported Versions

| Version line | Security support |
|--------------|------------------|
| Latest released baseline (`v2.3.x`) | Supported |
| Current development branch (`v2.4.0-dev`) | Best-effort review before release |
| Older versions | Not guaranteed |

Public documentation distinguishes the latest released baseline from unreleased current-branch capabilities. See [README.md](README.md) and [docs/CHANGELOG.en.md](docs/CHANGELOG.en.md).

### Reporting a Vulnerability

Please do not disclose vulnerability details in a public issue, pull request, discussion, or chat log.

Preferred reporting channel:

1. Use GitHub Private Vulnerability Reporting / Security Advisories for this repository.
2. If that is unavailable, open a non-sensitive issue asking for a maintainer security contact. Do not include exploit details, credentials, tenant identifiers, logs with secrets, or proof-of-concept payloads in the public issue.

Include, when safe:

- Affected version or commit.
- Deployment mode (`local`, Docker, Kubernetes, SaaS API, Web UI).
- Minimal reproduction steps.
- Expected impact and affected trust boundary.
- Any temporary mitigation you have found.

### Response Expectations

Maintainers will triage reports on a best-effort basis. Accepted reports should receive a severity assessment, a fix or mitigation plan, and coordinated disclosure guidance before public details are shared.

### Security Boundary

Security-sensitive behavior is documented in:

- [Security model](SECURITY_MODEL.md)
- [RBAC matrix](RBAC_MATRIX.md)
- [Enterprise readiness](ENTERPRISE_READINESS.md)
- [Deployment guide](docs/deployment.en.md)

## 中文

HermesX 是 Agent 运行时控制平面。安全报告可能涉及租户隔离、认证、RBAC、沙箱执行、出站访问控制、工作流执行或供应链配置。

### 支持版本

| 版本线 | 安全支持 |
|--------|----------|
| 最新已发布基线（`v2.3.x`） | 支持 |
| 当前开发分支（`v2.4.0-dev`） | 发布前 best-effort 审查 |
| 更早版本 | 不保证支持 |

公开文档会区分最新已发布基线和当前分支未发布能力。参见 [README.md](README.md) 和 [docs/CHANGELOG.md](docs/CHANGELOG.md)。

### 漏洞上报

请不要在公开 issue、pull request、discussion 或聊天记录中披露漏洞细节。

首选上报方式：

1. 使用本仓库的 GitHub Private Vulnerability Reporting / Security Advisories。
2. 如果该入口不可用，请创建一个不包含敏感细节的 issue，请求维护者提供安全联系方式。不要在公开 issue 中包含漏洞利用细节、凭证、租户标识、含密日志或 PoC payload。

在安全的前提下，请提供：

- 受影响版本或 commit。
- 部署模式（`local`、Docker、Kubernetes、SaaS API、Web UI）。
- 最小复现步骤。
- 预期影响和受影响的信任边界。
- 你已经找到的临时缓解方式。

### 响应预期

维护者会以 best-effort 方式分诊报告。确认有效的报告应获得严重度评估、修复或缓解计划，并在公开披露前协调披露节奏。

### 安全边界

安全相关行为记录在：

- [安全模型](SECURITY_MODEL.md)
- [RBAC 矩阵](RBAC_MATRIX.md)
- [企业就绪度](ENTERPRISE_READINESS.md)
- [部署指南](docs/deployment.md)
