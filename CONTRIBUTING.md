# Contributing to HermesX / 贡献指南

## English

Thank you for your interest in HermesX. This guide covers the public contribution workflow for the current repository.

### Getting Started

Prerequisites:

- Go 1.25 or later
- Git
- Docker, when working on SaaS, integration, or deployment flows
- Node.js 20, when working on `webui/`

Build the CLI/API binary:

```bash
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx
go build -o hermesx ./cmd/hermesx/
```

Run common checks:

```bash
go test ./... -short -count=1
go vet ./...
go test -race ./internal/agent/... ./internal/tools/... ./internal/gateway/... -count=1 -short
```

### Development Workflow

1. Fork the repository.
2. Create a focused branch from `main`, for example `feat/my-feature` or `docs/update-guide`.
3. Make a scoped change with clear commits.
4. Run the checks that match your change.
5. Submit a pull request using the [PR template](.github/PULL_REQUEST_TEMPLATE.md).

For platform-specific code, run cross-build checks:

```bash
GOOS=linux GOARCH=amd64 go build ./cmd/hermesx/
GOOS=darwin GOARCH=arm64 go build ./cmd/hermesx/
GOOS=windows GOARCH=amd64 go build ./cmd/hermesx/
```

### Commit Messages

Use Conventional Commits:

```text
feat(agent): add context compression threshold config
fix(gateway): wire signal.Notify for graceful shutdown
test(tools): add approval pattern matching tests
docs: update README with deployment notes
refactor(llm): extract provider detection
```

Common scopes include `agent`, `tools`, `gateway`, `cli`, `llm`, `config`, `skills`, `api`, `workflow`, `webui`, `docs`, and `ci`.

### Code and Documentation Style

- Follow standard Go conventions (`gofmt`, `go vet`).
- Use `slog` for structured logging.
- Keep security-sensitive details out of public issues and follow [SECURITY.md](SECURITY.md).
- Update README, docs, API reference, or examples when public behavior changes.
- Prefer focused tests over broad incidental coverage.

### Project Structure

```text
cmd/hermesx/             CLI and SaaS API entry point
internal/                Go implementation packages
skills/                  Bundled skill files
optional-skills/         Optional skill catalog metadata
docs/                    Maintained documentation site content
webui/                   Vue-based Web UI
deploy/                  Deployment and observability assets
scripts/                 Operational and validation scripts
```

### Should It Be a Skill or a Tool?

- Tools are Go capabilities registered in the runtime and callable by agents.
- Skills are Markdown/YAML instruction bundles that provide domain-specific workflows.

Specialized integrations or workflows should usually start as skills unless they need runtime privileges, typed execution, or shared platform behavior.

### License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).

## 中文

感谢你对 HermesX 的贡献兴趣。本指南说明当前仓库的公开贡献流程。

### 快速开始

前置要求：

- Go 1.25 或更高版本
- Git
- 涉及 SaaS、集成测试或部署流程时需要 Docker
- 涉及 `webui/` 时需要 Node.js 20

构建 CLI/API 二进制：

```bash
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx
go build -o hermesx ./cmd/hermesx/
```

常用检查：

```bash
go test ./... -short -count=1
go vet ./...
go test -race ./internal/agent/... ./internal/tools/... ./internal/gateway/... -count=1 -short
```

### 开发流程

1. Fork 仓库。
2. 从 `main` 创建聚焦分支，例如 `feat/my-feature` 或 `docs/update-guide`。
3. 提交范围清晰的变更。
4. 运行与变更匹配的检查。
5. 使用 [PR 模板](.github/PULL_REQUEST_TEMPLATE.md) 提交 pull request。

涉及平台相关代码时，运行交叉构建检查：

```bash
GOOS=linux GOARCH=amd64 go build ./cmd/hermesx/
GOOS=darwin GOARCH=arm64 go build ./cmd/hermesx/
GOOS=windows GOARCH=amd64 go build ./cmd/hermesx/
```

### 提交信息

使用 Conventional Commits：

```text
feat(agent): add context compression threshold config
fix(gateway): wire signal.Notify for graceful shutdown
test(tools): add approval pattern matching tests
docs: update README with deployment notes
refactor(llm): extract provider detection
```

常见 scope 包括 `agent`、`tools`、`gateway`、`cli`、`llm`、`config`、`skills`、`api`、`workflow`、`webui`、`docs`、`ci`。

### 代码与文档风格

- 遵循标准 Go 约定（`gofmt`、`go vet`）。
- 使用 `slog` 输出结构化日志。
- 不要在公开 issue 中披露安全敏感细节，并遵循 [SECURITY.md](SECURITY.md)。
- 公共行为变化时，同步更新 README、docs、API reference 或 examples。
- 优先补充聚焦测试，避免只追求偶然覆盖率。

### 项目结构

```text
cmd/hermesx/             CLI 和 SaaS API 入口
internal/                Go 实现包
skills/                  内置技能文件
optional-skills/         可选技能目录元数据
docs/                    持续维护的文档站内容
webui/                   Vue Web UI
deploy/                  部署与可观测性资产
scripts/                 运维与验证脚本
```

### Skill 还是 Tool？

- Tool 是注册到运行时、可由 Agent 调用的 Go 能力。
- Skill 是 Markdown/YAML 指令包，用于提供领域工作流。

专用集成或工作流通常应先作为 skill，除非它需要运行时权限、类型化执行或共享平台行为。

### 许可证

提交贡献即表示你同意贡献内容按 [MIT License](LICENSE) 授权。
