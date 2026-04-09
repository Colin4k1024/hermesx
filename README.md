# Hermes Agent Go

[English](#english) | [中文](#中文)

---

<a id="english"></a>

## English

A complete Go rewrite of [hermes-agent](https://github.com/NousResearch/hermes-agent) — the self-improving AI agent framework by [Nous Research](https://nousresearch.com).

### Why Go?

The original hermes-agent is written in Python (183K lines). This Go rewrite delivers the same functionality with significant advantages:

| | Python Original | Go Rewrite |
|---|---|---|
| **Distribution** | Complex (Python + uv + venv + npm) | Single binary, zero dependencies |
| **Startup time** | ~2s (Python import chain) | ~50ms |
| **Binary size** | ~500MB (with venv) | 23MB |
| **Concurrency** | asyncio + threading (GIL) | Goroutines (native parallelism) |
| **Cross-compile** | Requires per-platform setup | `GOOS=linux go build` |
| **Code volume** | 183K lines | 29K lines (6x more compact) |

### Project Stats

| Metric | Value |
|--------|-------|
| Go source files | 126 |
| Lines of code | 29,441 |
| Registered tools | 50 (36 core + 14 extended) |
| Platform adapters | 15 |
| Terminal backends | 7 |
| Bundled skills | 77 |
| Test files | 28 |

### Features

- **50 tools**: terminal, file operations, web search/crawl, browser automation, vision, image generation, TTS, code execution, subagent delegation, session search, memory, todo, cron, MCP, Home Assistant, and more
- **15 platform adapters**: Telegram, Discord, Slack, WhatsApp, Signal, Email, Matrix, Mattermost, DingTalk, Feishu, WeCom, SMS, Home Assistant, Webhook, API Server
- **7 terminal backends**: local, Docker, SSH, Modal, Daytona, Singularity, persistent shell
- **Dual API support**: OpenAI-compatible and Anthropic Messages API (with prompt caching)
- **Skill system**: procedural memory with YAML/Markdown skill files, hub search/install, security scanning
- **Session persistence**: SQLite with FTS5 full-text search
- **Context compression**: automatic summarization when approaching token limits
- **Smart model routing**: route simple queries to cheaper models
- **Fallback chain**: automatic model switching on API failures
- **Subagent delegation**: parallel task execution via goroutines (max 8 concurrent)
- **Cron scheduling**: scheduled tasks with multi-platform delivery
- **MCP integration**: Model Context Protocol client (stdio + SSE transport)
- **Profile system**: multiple isolated instances with separate configs
- **Plugin system**: user and project plugin discovery
- **ACP server**: editor integration for VS Code, Zed, JetBrains
- **Batch mode**: parallel trajectory generation for RL training

### Installation

#### From Source (recommended)

Requirements: Go 1.22+

```bash
git clone https://github.com/MLT-OSS/hermes-agent-go.git
cd hermes-agent-go
go build -o hermes ./cmd/hermes/

# Optional: install globally
sudo cp hermes /usr/local/bin/
# Or:
mkdir -p ~/.local/bin && cp hermes ~/.local/bin/
```

#### Using Make

```bash
git clone https://github.com/MLT-OSS/hermes-agent-go.git
cd hermes-agent-go
make build      # Build binary
make install    # Install to ~/.local/bin/
```

#### Docker

```bash
docker build -t hermes-agent .
docker run -it --rm \
  -v ~/.hermes:/home/hermes/.hermes \
  hermes-agent
```

#### Cross-compilation

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o hermes-linux ./cmd/hermes/

# macOS ARM
GOOS=darwin GOARCH=arm64 go build -o hermes-darwin ./cmd/hermes/

# Or build all platforms at once
make build-all
```

### Quick Start

```bash
# 1. Run the setup wizard (configure API keys and model)
./hermes setup

# 2. Start interactive CLI
./hermes

# 3. Or send a single query
./hermes chat "What tools do you have?"

# 4. Check system health
./hermes doctor
```

#### Using with custom API endpoint

```bash
# OpenAI-compatible API
./hermes --base-url "https://api.openai.com/v1" \
         --api-key "sk-..." \
         --model "gpt-4o"

# Anthropic API
./hermes --base-url "https://api.anthropic.com" \
         --api-key "sk-ant-..." \
         --api-mode anthropic \
         --model "claude-sonnet-4-20250514"

# Custom provider
./hermes --base-url "https://your-proxy.com" \
         --api-key "your-key" \
         --model "your-model"
```

### Configuration

Uses the same config files as the Python version (fully compatible):

| Path | Purpose |
|------|---------|
| `~/.hermes/config.yaml` | Main settings (model, terminal, display, etc.) |
| `~/.hermes/.env` | API keys and secrets |
| `~/.hermes/skills/` | Installed skill files |
| `~/.hermes/memories/` | Persistent memory (MEMORY.md, USER.md) |
| `~/.hermes/state.db` | SQLite session database |
| `~/.hermes/cron/` | Scheduled job data |
| `~/.hermes/skins/` | Custom CLI themes (YAML) |

### CLI Commands

```bash
hermes                  # Interactive CLI
hermes chat <query>     # Single query mode
hermes model [name]     # Show or switch model
hermes tools [list|enable|disable]   # Manage tools
hermes skills [list|search|install]  # Manage skills
hermes config [key] [value]          # View/edit config
hermes gateway start    # Start messaging gateway
hermes setup            # Interactive setup wizard
hermes doctor           # Run diagnostics
hermes cron [list|run|pause|remove]  # Manage scheduled tasks
hermes batch [prompts...] --file prompts.txt  # Batch mode
hermes claw migrate     # Migrate from OpenClaw
hermes version          # Show version info
```

### Differences from the Python Original

#### What's the same
- All 36 core tools (CoreTools) are identical
- Same `~/.hermes/` config directory structure
- Same SQLite schema for session persistence
- Same skill file format (YAML frontmatter + Markdown)
- Same toolset definitions and platform presets
- Same slash command names and aliases (40+ commands)
- Bundled skills are directly copied (77 skills)

#### What's improved
- **Single binary** — no Python/Node.js/venv setup required
- **Faster startup** — ~50ms vs ~2s
- **Native concurrency** — goroutines instead of asyncio + threading
- **Tool parallelization** — up to 8 concurrent tool executions (vs Python's sequential default)
- **Smaller footprint** — 23MB binary vs ~500MB Python environment
- **Cross-compilation** — trivial to build for any OS/arch

#### What's different
- CLI uses Lip Gloss for styling (vs Python's Rich + prompt_toolkit)
- No interactive line editing yet (bufio.Scanner vs prompt_toolkit's full readline)
- Browser tools use direct HTTP to Browserbase API (vs Python's agent-browser Node.js bridge)
- Voice mode is framework-only (detection + subprocess calls to whisper/edge-tts)
- Some deep error recovery paths are simplified (Python has 70K+ lines in terminal_tool.py alone)

#### What's not included
- Tinker-Atropos RL training environments (RL tools are stubs with setup instructions)
- WhatsApp Node.js bridge scripts (Go adapter uses HTTP bridge API)
- Documentation website (website/ directory)

### Architecture

```
hermes-agent-go/
├── cmd/hermes/              Entry point (Cobra CLI)
├── internal/
│   ├── agent/               Core agent loop, streaming, prompts, pricing
│   ├── acp/                 ACP editor integration server
│   ├── batch/               Batch trajectory generation
│   ├── cli/                 Interactive TUI, commands, skins, setup wizard
│   ├── config/              Config loading, profiles, env, migration
│   ├── cron/                Scheduler and job persistence
│   ├── gateway/             Multi-platform messaging gateway
│   │   └── platforms/       15 platform adapters
│   ├── llm/                 LLM client (OpenAI + Anthropic)
│   ├── plugins/             Plugin discovery and loading
│   ├── skills/              Skill loading, parsing, hub, security
│   ├── state/               SQLite session database + export
│   ├── tools/               50 tool implementations
│   │   └── environments/    7 terminal backends
│   ├── toolsets/            Tool grouping and resolution
│   └── utils/               Shared utilities
├── skills/                  77 bundled skills
├── optional-skills/         Official optional skills
├── Makefile
├── Dockerfile
└── go.mod
```

### Testing

```bash
go test ./...              # Run all tests
go test ./... -v           # Verbose output
go test ./... -cover       # With coverage
go test -race ./...        # Race condition detection
make test                  # Via Makefile
```

### License

MIT — same as the [original Python version](https://github.com/NousResearch/hermes-agent).

---

<a id="中文"></a>

## 中文

[hermes-agent](https://github.com/NousResearch/hermes-agent) 的完整 Go 语言重写版 —— 由 [Nous Research](https://nousresearch.com) 开发的自我进化 AI Agent 框架。

### 为什么用 Go 重写？

原版 hermes-agent 使用 Python 编写（183K 行代码）。Go 重写版在保持完整功能的同时，带来了显著优势：

| | Python 原版 | Go 重写版 |
|---|---|---|
| **分发方式** | 复杂（Python + uv + venv + npm） | 单个二进制文件，零依赖 |
| **启动时间** | ~2 秒 | ~50 毫秒 |
| **体积** | ~500MB（含 venv） | 23MB |
| **并发模型** | asyncio + threading（GIL 限制） | Goroutine（原生并行） |
| **交叉编译** | 需要各平台独立配置 | `GOOS=linux go build` |
| **代码量** | 183K 行 | 29K 行（紧凑 6 倍） |

### 项目数据

| 指标 | 数值 |
|------|------|
| Go 源文件 | 126 个 |
| 代码行数 | 29,441 行 |
| 注册工具 | 50 个（36 核心 + 14 扩展） |
| 平台适配器 | 15 个 |
| 终端后端 | 7 个 |
| 内置技能 | 77 个 |
| 测试文件 | 28 个 |

### 主要功能

- **50 个工具**：终端执行、文件操作、网页搜索/爬取、浏览器自动化、视觉分析、图像生成、语音合成、代码执行、子 Agent 委派、会话搜索、记忆、待办、定时任务、MCP、Home Assistant 等
- **15 个消息平台**：Telegram、Discord、Slack、WhatsApp、Signal、邮件、Matrix、Mattermost、钉钉、飞书、企业微信、短信、Home Assistant、Webhook、API Server
- **7 个终端后端**：本地、Docker、SSH、Modal、Daytona、Singularity、持久化 Shell
- **双 API 支持**：OpenAI 兼容格式 + Anthropic Messages API（含 Prompt 缓存）
- **技能系统**：YAML/Markdown 技能文件，Hub 搜索安装，安全扫描
- **会话持久化**：SQLite + FTS5 全文搜索
- **上下文压缩**：接近 token 上限时自动摘要
- **智能路由**：简单查询自动路由到便宜模型
- **回退链**：API 故障时自动切换模型
- **子 Agent 并行**：goroutine 池最多 8 并发
- **定时调度**：支持多平台投递的 Cron 任务
- **MCP 集成**：支持 stdio + SSE 传输层
- **多实例**：Profile 系统隔离配置
- **插件系统**：用户和项目级插件发现
- **编辑器集成**：ACP 服务器支持 VS Code/Zed/JetBrains
- **批量模式**：并行轨迹生成用于 RL 训练

### 安装

#### 从源码构建（推荐）

前置条件：Go 1.22+

```bash
git clone https://github.com/MLT-OSS/hermes-agent-go.git
cd hermes-agent-go
go build -o hermes ./cmd/hermes/

# 可选：全局安装
sudo cp hermes /usr/local/bin/
# 或者：
mkdir -p ~/.local/bin && cp hermes ~/.local/bin/
```

#### 使用 Make

```bash
git clone https://github.com/MLT-OSS/hermes-agent-go.git
cd hermes-agent-go
make build      # 构建二进制
make install    # 安装到 ~/.local/bin/
```

#### Docker

```bash
docker build -t hermes-agent .
docker run -it --rm \
  -v ~/.hermes:/home/hermes/.hermes \
  hermes-agent
```

### 快速开始

```bash
# 1. 运行设置向导（配置 API Key 和模型）
./hermes setup

# 2. 启动交互式 CLI
./hermes

# 3. 或发送单次查询
./hermes chat "你有什么工具？"

# 4. 检查系统状态
./hermes doctor
```

#### 使用自定义 API 端点

```bash
# OpenAI 兼容 API
./hermes --base-url "https://api.openai.com/v1" \
         --api-key "sk-..." \
         --model "gpt-4o"

# Anthropic API
./hermes --base-url "https://api.anthropic.com" \
         --api-key "sk-ant-..." \
         --api-mode anthropic \
         --model "claude-sonnet-4-20250514"

# 自定义代理
./hermes --base-url "https://your-proxy.com" \
         --api-key "your-key" \
         --model "your-model"
```

### 配置

与 Python 版使用完全相同的配置文件（完全兼容）：

| 路径 | 用途 |
|------|------|
| `~/.hermes/config.yaml` | 主配置（模型、终端、显示等） |
| `~/.hermes/.env` | API Key 和密钥 |
| `~/.hermes/skills/` | 已安装的技能文件 |
| `~/.hermes/memories/` | 持久化记忆（MEMORY.md、USER.md） |
| `~/.hermes/state.db` | SQLite 会话数据库 |
| `~/.hermes/cron/` | 定时任务数据 |
| `~/.hermes/skins/` | 自定义 CLI 主题（YAML） |

### 与 Python 原版的差异

#### 完全一致
- 36 个核心工具（CoreTools）名称和参数一致
- `~/.hermes/` 配置目录结构一致
- SQLite 会话数据库 Schema 一致
- 技能文件格式（YAML frontmatter + Markdown）一致
- 工具集定义和平台预设一致
- 斜杠命令名称和别名一致（40+ 命令）
- 内置技能直接复制（77 个）

#### 改进之处
- **单文件部署** —— 无需安装 Python/Node.js/venv
- **启动更快** —— ~50ms vs ~2s
- **原生并发** —— goroutine 替代 asyncio + threading
- **工具并行** —— 最多 8 个工具同时执行（Python 默认串行）
- **体积更小** —— 23MB 二进制 vs ~500MB Python 环境
- **交叉编译** —— 轻松构建任意 OS/架构

#### 实现差异
- CLI 使用 Lip Gloss 美化（而非 Python 的 Rich + prompt_toolkit）
- 暂无交互式行编辑（使用 bufio.Scanner，非 prompt_toolkit 的完整 readline）
- 浏览器工具直接通过 HTTP 调用 Browserbase API（非 Python 的 agent-browser Node.js 桥接）
- 语音模式为框架级实现（检测 + 调用 whisper/edge-tts 子进程）
- 部分深层错误恢复路径做了简化

#### 未包含
- Tinker-Atropos RL 训练环境（RL 工具为 stub，提供安装说明）
- WhatsApp Node.js 桥接脚本（Go 适配器使用 HTTP 桥接 API）
- 文档网站（website/ 目录）

### 许可证

MIT —— 与 [Python 原版](https://github.com/NousResearch/hermes-agent) 一致。
