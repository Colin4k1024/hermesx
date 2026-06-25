# Contributing to HermesX / 贡献指南

## English

Thank you for your interest in HermesX. This guide covers the public contribution workflow for the current repository.

### Getting Started

Prerequisites:

- Go 1.25 or later
- Git
- Docker, when working on SaaS, integration, or deployment flows
- Node.js 20, when working on `webui/`

#### Quick Start (Single Command)

```bash
git clone https://github.com/Colin4k1024/hermesx.git && cd hermesx && docker compose -f docker-compose.test.yml up -d && go test ./... -short -count=1
```

This will:
1. Clone the repository
2. Start test dependencies (PostgreSQL, Redis, MinIO)
3. Run all unit tests

#### Manual Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/Colin4k1024/hermesx.git
   cd hermesx
   ```

2. Build the CLI/API binary:
   ```bash
   go build -o hermesx ./cmd/hermesx/
   ```

3. Start test dependencies:
   ```bash
   docker compose -f docker-compose.test.yml up -d
   ```

4. Run common checks:
   ```bash
   go test ./... -short -count=1
   go vet ./...
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
sdk/                     Generated SDKs (Go, TypeScript)
pkg/governance/          L2 interface contracts
```

### Adding a New Tool

To add a new tool to HermesX:

1. Create a new file in `internal/tools/` for your tool:
   ```go
   package tools
   
   import (
       "context"
       "github.com/Colin4k1024/hermesx/internal/store"
   )
   
   type MyTool struct {
       store store.Store
   }
   
   func (t *MyTool) Name() string {
       return "my_tool"
   }
   
   func (t *MyTool) Description() string {
       return "Description of what my tool does"
   }
   
   func (t *MyTool) Schema() map[string]any {
       return map[string]any{
           "name":        t.Name(),
           "description": t.Description(),
           "parameters": map[string]any{
               "type": "object",
               "properties": map[string]any{
                   "param1": map[string]any{
                       "type":        "string",
                       "description": "Description of param1",
                   },
               },
               "required": []string{"param1"},
           },
       }
   }
   
   func (t *MyTool) Execute(ctx context.Context, args map[string]any, tctx *ToolContext) string {
       // Implement tool logic here
       param1, _ := args["param1"].(string)
       return "Result: " + param1
   }
   ```

2. Register the tool in `internal/tools/registry.go`:
   ```go
   func init() {
       Register(&MyTool{})
   }
   ```

3. Add tests in `internal/tools/my_tool_test.go`:
   ```go
   package tools
   
   import (
       "context"
       "testing"
   )
   
   func TestMyTool_Execute(t *testing.T) {
       tool := &MyTool{}
       result := tool.Execute(context.Background(), map[string]any{"param1": "test"}, nil)
       if result != "Result: test" {
           t.Errorf("expected 'Result: test', got %q", result)
       }
   }
   ```

4. Update documentation in `docs/` if needed.

### Adding a New Platform Adapter

To add a new platform adapter (e.g., Notion, Slack, Discord):

1. Create a new file in `internal/platforms/` for your adapter:
   ```go
   package platforms
   
   import (
       "context"
       "github.com/Colin4k1024/hermesx/internal/llm"
   )
   
   type NotionAdapter struct {
       apiKey string
   }
   
   func NewNotionAdapter(apiKey string) *NotionAdapter {
       return &NotionAdapter{apiKey: apiKey}
   }
   
   func (a *NotionAdapter) Name() string {
       return "notion"
   }
   
   func (a *NotionAdapter) SendMessage(ctx context.Context, channelID, message string) error {
       // Implement Notion API integration
       return nil
   }
   
   func (a *NotionAdapter) ReceiveMessages(ctx context.Context, channelID string) (<-chan llm.Message, error) {
       // Implement message receiving
       msgCh := make(chan llm.Message)
       close(msgCh)
       return msgCh, nil
   }
   ```

2. Register the adapter in `internal/platforms/registry.go`:
   ```go
   func init() {
       Register("notion", NewNotionAdapter)
   }
   ```

3. Add tests in `internal/platforms/notion_test.go`.

4. Update documentation in `docs/platforms/`.

### Testing

#### Unit Tests

Run unit tests:
```bash
go test ./... -short -count=1
```

#### Integration Tests

Run integration tests (requires test dependencies):
```bash
go test -tags=integration ./tests/integration/... -v -count=1
```

#### Race Detection

Run tests with race detection:
```bash
go test -race ./internal/agent/... ./internal/tools/... ./internal/gateway/... -count=1 -short
```

### Pull Request Process

1. Ensure your PR has a clear title and description.
2. Reference any related issues (e.g., "Fixes #123").
3. Include tests for new functionality.
4. Update documentation if public behavior changes.
5. Run all checks before submitting.
6. PRs require at least one approval from a maintainer.
7. PRs should be merged within 2 weeks if no issues are raised.

### Good First Issues

Looking for ways to contribute? Check out these good first issues:

- ["Add a new platform adapter for Notion"](https://github.com/Colin4k1024/hermesx/issues/XX) - Mimic existing adapters
- ["Write integration test for rate limiter under concurrent load"](https://github.com/Colin4k1024/hermesx/issues/XX) - Help improve test coverage
- ["Add govulncheck baseline suppression file"](https://github.com/Colin4k1024/hermesx/issues/XX) - Security tooling
- ["Document the ExecutionReceipt API in openapi.yaml"](https://github.com/Colin4k1024/hermesx/issues/XX) - Documentation improvement

### License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).

HermesX does not currently require a CLA. Contributions use the inbound-equals-outbound model: your contribution is accepted under the same MIT license as the project.

Do not commit local AI-agent configuration or scratch files such as `.claude/`, `.deepseek/`, `.cursor/`, `.copilot/`, or `user_notes.md`. These paths are ignored to prevent accidental tool configuration or prompt artifacts from entering release branches.

---

## 中文

感谢你对 HermesX 的贡献兴趣。本指南说明当前仓库的公开贡献流程。

### 快速开始

前置要求：

- Go 1.25 或更高版本
- Git
- 涉及 SaaS、集成测试或部署流程时需要 Docker
- 涉及 `webui/` 时需要 Node.js 20

#### 快速开始（单命令）

```bash
git clone https://github.com/Colin4k1024/hermesx.git && cd hermesx && docker compose -f docker-compose.test.yml up -d && go test ./... -short -count=1
```

这将：
1. 克隆仓库
2. 启动测试依赖（PostgreSQL、Redis、MinIO）
3. 运行所有单元测试

#### 手动设置

1. 克隆仓库：
   ```bash
   git clone https://github.com/Colin4k1024/hermesx.git
   cd hermesx
   ```

2. 构建 CLI/API 二进制：
   ```bash
   go build -o hermesx ./cmd/hermesx/
   ```

3. 启动测试依赖：
   ```bash
   docker compose -f docker-compose.test.yml up -d
   ```

4. 运行常用检查：
   ```bash
   go test ./... -short -count=1
   go vet ./...
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

常见 scope 包括 `agent`、`tools`、`gateway`、`cli`、`llm`、`config`、`skills`、`api`、`workflow`、`webui`、`docs` 和 `ci`。

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
sdk/                     生成的 SDK（Go、TypeScript）
pkg/governance/          L2 接口合约
```

### 添加新工具

要向 HermesX 添加新工具：

1. 在 `internal/tools/` 中创建新文件：
   ```go
   package tools
   
   import (
       "context"
       "github.com/Colin4k1024/hermesx/internal/store"
   )
   
   type MyTool struct {
       store store.Store
   }
   
   func (t *MyTool) Name() string {
       return "my_tool"
   }
   
   func (t *MyTool) Description() string {
       return "工具功能描述"
   }
   
   func (t *MyTool) Schema() map[string]any {
       return map[string]any{
           "name":        t.Name(),
           "description": t.Description(),
           "parameters": map[string]any{
               "type": "object",
               "properties": map[string]any{
                   "param1": map[string]any{
                       "type":        "string",
                       "description": "param1 描述",
                   },
               },
               "required": []string{"param1"},
           },
       }
   }
   
   func (t *MyTool) Execute(ctx context.Context, args map[string]any, tctx *ToolContext) string {
       // 实现工具逻辑
       param1, _ := args["param1"].(string)
       return "结果: " + param1
   }
   ```

2. 在 `internal/tools/registry.go` 中注册工具：
   ```go
   func init() {
       Register(&MyTool{})
   }
   ```

3. 在 `internal/tools/my_tool_test.go` 中添加测试：
   ```go
   package tools
   
   import (
       "context"
       "testing"
   )
   
   func TestMyTool_Execute(t *testing.T) {
       tool := &MyTool{}
       result := tool.Execute(context.Background(), map[string]any{"param1": "test"}, nil)
       if result != "结果: test" {
           t.Errorf("expected '结果: test', got %q", result)
       }
   }
   ```

4. 如果需要，更新 `docs/` 中的文档。

### 添加新平台适配器

要添加新平台适配器（例如 Notion、Slack、Discord）：

1. 在 `internal/platforms/` 中创建新文件：
   ```go
   package platforms
   
   import (
       "context"
       "github.com/Colin4k1024/hermesx/internal/llm"
   )
   
   type NotionAdapter struct {
       apiKey string
   }
   
   func NewNotionAdapter(apiKey string) *NotionAdapter {
       return &NotionAdapter{apiKey: apiKey}
   }
   
   func (a *NotionAdapter) Name() string {
       return "notion"
   }
   
   func (a *NotionAdapter) SendMessage(ctx context.Context, channelID, message string) error {
       // 实现 Notion API 集成
       return nil
   }
   
   func (a *NotionAdapter) ReceiveMessages(ctx context.Context, channelID string) (<-chan llm.Message, error) {
       // 实现消息接收
       msgCh := make(chan llm.Message)
       close(msgCh)
       return msgCh, nil
   }
   ```

2. 在 `internal/platforms/registry.go` 中注册适配器：
   ```go
   func init() {
       Register("notion", NewNotionAdapter)
   }
   ```

3. 在 `internal/platforms/notion_test.go` 中添加测试。

4. 更新 `docs/platforms/` 中的文档。

### 测试

#### 单元测试

运行单元测试：
```bash
go test ./... -short -count=1
```

#### 集成测试

运行集成测试（需要测试依赖）：
```bash
go test -tags=integration ./tests/integration/... -v -count=1
```

#### 竞态检测

运行带竞态检测的测试：
```bash
go test -race ./internal/agent/... ./internal/tools/... ./internal/gateway/... -count=1 -short
```

### Pull Request 流程

1. 确保你的 PR 有清晰的标题和描述。
2. 引用相关 issue（例如 "Fixes #123"）。
3. 包含新功能的测试。
4. 如果公共行为发生变化，更新文档。
5. 提交前运行所有检查。
6. PR 需要至少一个维护者的批准。
7. 如果 2 周内没有问题，PR 应该被合并。

### Good First Issues

寻找贡献方式？查看这些 good first issues：

- ["添加 Notion 平台适配器"](https://github.com/Colin4k1024/hermesx/issues/XX) - 模仿现有适配器
- ["编写限流器并发负载集成测试"](https://github.com/Colin4k1024/hermesx/issues/XX) - 帮助提高测试覆盖率
- ["添加 govulncheck 基线抑制文件"](https://github.com/Colin4k1024/hermesx/issues/XX) - 安全工具
- ["在 openapi.yaml 中文档化 ExecutionReceipt API"](https://github.com/Colin4k1024/hermesx/issues/XX) - 文档改进

### 许可证

提交贡献即表示你同意贡献内容按 [MIT License](LICENSE) 授权。

HermesX 当前不要求 CLA。贡献采用 inbound-equals-outbound 模型：你的贡献按项目相同的 MIT 许可证接收。

不要提交本地 AI Agent 配置或草稿文件，例如 `.claude/`、`.deepseek/`、`.cursor/`、`.copilot/` 或 `user_notes.md`。这些路径已被忽略，用于避免工具配置或 prompt 草稿误入发布分支。
