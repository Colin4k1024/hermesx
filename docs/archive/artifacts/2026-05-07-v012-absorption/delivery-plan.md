# Delivery Plan: Hermes Agent v0.12.0 能力吸收

**版本目标**: v1.4.0  
**来源**: [NousResearch/hermes-agent v2026.4.30](https://github.com/NousResearch/hermes-agent/releases/tag/v2026.4.30)  
**范围**: P0 (5项) + P1 (5项) = 10 项能力  
**交付节奏**: 3 Sprints，每 sprint 约 3-4 项能力

---

## Sprint 1 — 架构基础 (Plugin Gateway + Model Catalog + CJK Search)

### 1.1 Pluggable Gateway Platform Architecture

**目标**: 将 `gateway/platforms/` 从硬编码适配器重构为 plugin-host 模式。

**当前状态**: 15 个平台全部编译在 `gateway/platforms/` 中，添加新平台需改核心代码。

**实施方案**:
- 定义 `PlatformPlugin` 接口 (Init/Start/Stop/HandleMessage/SendMessage)
- 抽取现有适配器实现为该接口的内置实现
- 新增 plugin registry，支持通过配置文件启用/禁用平台
- 支持 `pre_gateway_dispatch` 和 `post_delivery` hook 点
- 保留现有 `PlatformAdapter` 接口兼容性

**影响面**: `internal/gateway/runner.go`, `internal/gateway/platforms/`, `internal/plugins/`

**验证标准**:
- 现有 15 个平台功能不变
- 新平台可通过实现接口 + 注册即可接入
- gateway_test.go 全绿

### 1.2 Remote Model Catalog Manifest

**目标**: 模型元数据从远程 manifest 加载，不发版即可上新模型。

**当前状态**: `llm/models.go` 中 `KnownModels` 为编译时硬编码 map。

**实施方案**:
- 新增 `ModelCatalog` 结构，支持 embedded default + remote override
- 远程 manifest 为 JSON 文件 (hosted on S3/CDN)，定期拉取 (默认 1h)
- 本地缓存 + fallback 到 embedded defaults
- 新增 `hermes models refresh` CLI 命令手动刷新
- 保留 `KnownModels` 作为 fallback 编译时默认值

**影响面**: `internal/llm/models.go`, `internal/llm/model_discovery.go`, `internal/config/`

**验证标准**:
- 启动时加载远程 manifest 成功
- manifest 不可达时 fallback 到本地
- 新增模型后不需重新编译

### 1.3 Trigram FTS for CJK Search (pg_trgm)

**目标**: 多租户中日韩全文搜索替换 LIKE 查询。

**当前状态**: session/memory 搜索使用 LIKE '%keyword%'，对 CJK 无分词能力。

**实施方案**:
- PostgreSQL 启用 `pg_trgm` + `gin_trgm_ops` 索引
- 对 `memories.content`, `messages.content`, `sessions.title` 建 GIN trigram 索引
- 搜索查询改用 `similarity()` + `%` operator
- 可选: 结合 `ts_vector` 做英文分词 + trigram 做 CJK
- Migration 文件 + 向后兼容 (索引 IF NOT EXISTS)

**影响面**: `internal/store/pg/`, migrations, `internal/api/` 搜索端点

**验证标准**:
- 中文关键词搜索准确率 > 90%
- 搜索延迟 < 100ms (10万条记录)
- 集成测试覆盖

---

## Sprint 2 — 运营效率 (Cron Enhancement + Cache TTL + Compression + Image Routing)

### 2.1 Cron Per-job Workdir + Context Chaining

**目标**: Cron 任务支持项目级工作目录和任务间上下文传递。

**当前状态**: `internal/cron/scheduler.go` 只有基础定时调度，无 workdir 和 context 概念。

**实施方案**:
- `CronJob` 结构新增 `Workdir string` 和 `ContextFrom string` 字段
- 执行时 chdir 到指定 workdir（通过 agent context 传递）
- `ContextFrom` 指定前序 job ID，运行时将其最近一次输出注入 system prompt
- 输出存储到 `logs/cron/{jobID}/{timestamp}.json`
- API 支持查询 job 历史输出

**影响面**: `internal/cron/`, `internal/agent/`, job store schema

**验证标准**:
- job 可指定 workdir 执行
- job B 可读取 job A 的最近输出
- scheduler_test.go 覆盖新字段

### 2.2 Configurable Prompt Cache TTL

**目标**: 支持按 provider 配置 prompt cache 生命周期，降低推理成本。

**当前状态**: agent cache 使用固定 TTL，无 prompt-level cache 配置。

**实施方案**:
- config 新增 `inference.prompt_cache_ttl` (默认 5m, 可配 1m-1h)
- `llm/client.go` 的 system prompt 构建增加 cache_control breakpoint 标记
- Anthropic transport 的 `cache_control: {type: "ephemeral"}` 根据 TTL 策略注入
- agentCache LRU TTL 与 prompt cache TTL 对齐
- 新增 metric: `hermes_prompt_cache_hit_total` / `hermes_prompt_cache_miss_total`

**影响面**: `internal/config/`, `internal/llm/anthropic.go`, `internal/llm/client.go`

**验证标准**:
- TTL 配置生效
- metric 可观测 cache hit/miss ratio
- 单元测试覆盖配置变更

### 2.3 Compression Retry with Main Model Fallback

**目标**: 上下文压缩失败时 fallback 到主模型重试。

**当前状态**: `internal/state/` 压缩逻辑无 fallback，失败即报错。

**实施方案**:
- 新增 `CompressionConfig` 支持 `auxiliary_model` + `fallback_to_main` 配置
- 压缩流程: aux model → 失败 → retry main model → 失败 → 通知用户
- aux model 失败时在 delivery 层通知用户 "using fallback model for compression"
- 压缩 token budget 预留 system + tools headroom

**影响面**: `internal/state/`, `internal/llm/client.go`, `internal/config/`

**验证标准**:
- aux model 500 时 fallback 到 main 成功压缩
- 通知消息送达
- 单元测试 mock 双模型场景

### 2.4 Native Multimodal Image Routing

**目标**: 基于模型 vision 能力路由图片，避免浪费 token。

**当前状态**: `llm/client.go` 无条件将图片内容发送给所有模型。

**实施方案**:
- 利用 `ModelMeta.SupportsVision` 判断模型是否支持图片
- 不支持 vision 的模型: 图片内容转为 `[Image: {description}]` 文本描述
- 可选: 对支持 vision 但 context 有限的模型做图片降分辨率
- routing 决策在 `client.go` 的 message 构建阶段完成

**影响面**: `internal/llm/client.go`, `internal/llm/models.go`

**验证标准**:
- deepseek (no vision) 不收到 base64 图片
- claude (vision) 正常收到图片
- 单元测试覆盖路由逻辑

---

## Sprint 3 — 智能自治 (Curator + Self-improvement + Media Parity + Gateway Hooks)

### 3.1 Autonomous Curator (技能自动维护)

**目标**: 后台 agent 定期评级和清理 skill library。

**当前状态**: `internal/skills/` 有 loader + hub + guard 但无自动维护机制。

**实施方案**:
- 新增 `internal/curator/` 包
- Curator 作为 cron job 运行 (7 天周期)，持有独立 agent context
- 评级维度: usage count, last_used, quality score (model-based)
- 动作: consolidate (合并相关 skill), prune (归档低使用 skill), report
- 输出: `logs/curator/run.json` + `REPORT.md`
- defense-in-depth: pinned/builtin skills 不可修改
- `hermes curator status` CLI 查看排名

**影响面**: 新包 `internal/curator/`, `internal/cron/`, `internal/skills/`

**验证标准**:
- Curator 按周期运行并产出报告
- pinned skill 不被修改
- CLI 命令可查看状态

### 3.2 Self-improvement Loop (背景审查)

**目标**: 每轮对话后 background goroutine 审查并更新记忆/技能。

**当前状态**: 无类似机制，用户需手动管理记忆和技能。

**实施方案**:
- 新增 `internal/agent/reviewer.go`
- 对话 turn 结束后 spawn background goroutine
- 审查上下文: 当前 turn 的对话内容 (排除 tool messages)
- 决策类型: save_memory / update_skill / skip
- 使用 rubric-based prompt (非 free-form)
- 受限 toolset: 仅 memory + skill 操作
- 清理: goroutine 超时退出 (30s)，memory provider 正确 shutdown

**影响面**: `internal/agent/`, `internal/skills/`, memory store

**验证标准**:
- 对话后 background 审查触发
- 仅使用 memory/skill toolset
- goroutine 不泄漏 (race test 通过)

### 3.3 Gateway Media Parity (统一多媒体路由)

**目标**: 跨平台统一多图发送和音频路由。

**当前状态**: 各平台独立处理媒体，行为不一致。

**实施方案**:
- 新增 `internal/gateway/media_router.go` 统一媒体路由层
- 多图: 按平台能力拆分 (Telegram: album, Discord: embeds, Slack: blocks)
- 音频: 统一音频格式转换 (FLAC → platform-specific)
- media_cache 增加 format adaptation 能力
- 平台能力声明: 每个 adapter 声明支持的 media types 和 limits

**影响面**: `internal/gateway/`, `internal/gateway/platforms/`

**验证标准**:
- 同一条多图消息在 Telegram/Discord/Slack 正确展示
- 音频消息跨平台送达
- gateway_test.go 覆盖

### 3.4 Extended Gateway Hook Points

**目标**: 扩展 hook 生态，支持更多非侵入式扩展点。

**当前状态**: `gateway/hooks.go` 有基础 hook registry，覆盖面有限。

**实施方案**:
- 新增 hook 点: `pre_approval_request`, `post_approval_response`, `post_tool_call` (含 duration_ms)
- Hook 支持 async 执行 (不阻塞主流程)
- Hook 支持 filter (按 platform/event type 过滤)
- Hook 结果可注入 agent context (如 Langfuse trace ID)

**影响面**: `internal/gateway/hooks.go`, `internal/plugins/hooks.go`

**验证标准**:
- 新 hook 点触发正确
- async hook 不阻塞主流程
- plugins_test.go 覆盖

---

## 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| Pluggable gateway 重构导致现有平台回归 | HIGH | 先 interface 抽取，保持内置注册，逐步迁移 |
| Remote model catalog 网络不可达 | MEDIUM | embedded defaults 作为 fallback，graceful degradation |
| Curator 误删有用 skill | HIGH | defense-in-depth gates + pin 机制 + dry-run 模式 |
| Self-improvement goroutine leak | MEDIUM | 30s 超时 + context.WithCancel + race detector CI |
| pg_trgm 大表索引创建耗时 | LOW | CONCURRENTLY 创建，migration 可重入 |

## 放行标准

- 全量测试通过 (≥1470 用例)
- Race detector 无新增竞态
- 新增代码测试覆盖 ≥ 80%
- Breaking changes 有 migration path
- CI pipeline 全绿 (lint + test + build + docker)

## 依赖

- PostgreSQL: pg_trgm extension (Sprint 1.3)
- CDN/S3: model catalog manifest hosting (Sprint 1.2)
- 无新增外部 Go 依赖 (尽可能复用现有)
