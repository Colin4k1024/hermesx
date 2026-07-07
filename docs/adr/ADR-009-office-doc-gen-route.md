# ADR-009: Office 文档生成技术路线 (Go + Python Hybrid)

## 决策信息

| 项目 | 内容 |
|------|------|
| 编号 | ADR-009 |
| 标题 | Office 文档生成采用 Go excelize (xlsx) + Python sandbox (docx/pptx) 混合路线 |
| 状态 | Accepted |
| 日期 | 2026-07-06 |
| Owner | tech-lead |
| 关联需求 | 文档生成能力（xlsx/docx/pptx），SaaS 商业许可合规 |

---

## 背景与约束

### 当前问题

HermesX 需要为租户 Agent 提供 `.xlsx`、`.docx`、`.pptx` 三类 Office 文档生成能力。当前平台已具备：

- `internal/tools/code_exec.go`：沙箱代码执行工具，支持 `docker` / `k8s-job` 两种隔离模式
- `skills/productivity/ocr-and-documents/`：已集成 `python-docx` 用于 DOCX 解析
- `skills/productivity/powerpoint/`：已集成 `python-pptx` 及 XML 级操作用于 PPTX 处理

需要为这三类文档确定生成路线，并注册为平台 tool 供 Agent 调用。

### 业务目标

- Agent 能根据用户指令生成结构化 Office 文档并返回下载链接
- 生成延迟可控（xlsx < 2s, docx/pptx < 10s），满足交互式场景
- 所有依赖库许可证兼容商业 SaaS 分发（禁止 AGPL）

### 约束条件

- 商业 SaaS 产品，任何 AGPL 依赖将导致整个服务开源义务
- 沙箱已有 cgroup 限制：内存 512MB、CPU 1 核、超时 120s
- Python 沙箱已具备完整 pip 环境（`python-docx`、`python-pptx` 已预装）
- 文件产物必须上传 MinIO/S3 后返回 artifact URL，不允许持久化到本地磁盘

### 非目标

- 不支持 PDF 生成（已由 `nano-pdf` skill 覆盖）
- 不支持复杂排版/母版定制（属于 skill 层逻辑，不在 tool 层承担）
- 不引入 LibreOffice / headless 转换（当前不需要格式互转）

---

## 备选方案

### 方案 A：Go excelize (xlsx) + Python sandbox (docx/pptx)（采用）

| 维度 | xlsx | docx / pptx |
|------|------|-------------|
| 实现 | Go in-process (`excelize`) | Python sandbox (`python-docx` / `python-pptx`) |
| 许可证 | Apache-2.0 | MIT |
| 调用开销 | 无进程切换，纳秒级 | sandbox 启动 + 执行，典型 2-8s |
| 成熟度 | 17k+ stars, 活跃维护 | 各 3k+ stars, 生态成熟 |

- **适用条件**：xlsx 高频且结构简单，适合 in-process；docx/pptx 复杂度高且已有 Python 生态集成
- **优点**：xlsx 零延迟、docx/pptx 复用现有 sandbox 和 skill 资产、无新增部署组件
- **风险**：Python sandbox 受 cgroup 限制，超大文档可能 OOM
- **缓解**：通过行数/页数上限做输入校验，超限时返回友好错误

### 方案 B：Go native 全覆盖（excelize + unioffice）

- **适用条件**：希望所有文档生成都在 Go 进程内完成
- **优点**：统一技术栈，无 sandbox 开销
- **风险**：unioffice 采用 AGPL-3.0 许可证，商业 SaaS 使用需要购买商业授权（年费 $299+），且功能覆盖度不如 python-docx/python-pptx
- **不选原因**：AGPL 与 SaaS 模式不兼容，商业授权增加持续成本且 vendor lock-in 风险高

### 方案 C：MCP 委托外部服务

- **适用条件**：存在成熟的 Office 生成 MCP server
- **优点**：解耦、可独立扩展
- **风险**：截至 2026-07，社区无成熟的 Office doc generation MCP server；自建等于多维护一个微服务
- **不选原因**：增加网络延迟（+50-200ms RTT）和运维复杂度，无实际收益

### 方案 D：独立 Python sidecar 进程

- **适用条件**：sandbox 不存在或不可复用
- **优点**：长驻进程可避免冷启动
- **风险**：新增部署单元、端口管理、健康检查、资源隔离需要独立实现
- **不选原因**：`execute_code` sandbox 已完整覆盖隔离、超时、资源限制需求，sidecar 是重复建设

---

## 决策结果

### 采用方案

**方案 A：Go excelize (xlsx) + Python sandbox (docx/pptx)**

### 原因

1. **xlsx 用 Go excelize**：Apache-2.0 许可证完全兼容商业 SaaS；in-process 执行无序列化/反序列化开销；17k+ stars 社区活跃、API 稳定
2. **docx/pptx 用 Python sandbox**：MIT 许可证无合规风险；`python-docx` / `python-pptx` 已在 skills 中验证可用；复用已有 `execute_code` 基础设施，零新增部署组件
3. **不用 unioffice**：AGPL-3.0 与 SaaS 发行模式法律冲突，即使购买商业授权也存在 vendor lock-in
4. **不用 MCP 委托**：无社区 server 可用，自建等价于独立微服务且增加延迟
5. **不用 Python sidecar**：sandbox 已提供完整隔离能力（docker / k8s-job），sidecar 是冗余抽象

### 影响范围

- 新增 3 个 tool 注册（`generate_xlsx`、`generate_docx`、`generate_pptx`）
- `internal/tools/` 新增对应 handler 文件
- `internal/objstore/` 已有 MinIO 上传能力，无需新增

### 兼容性

- 不影响现有 `execute_code` tool 行为
- 不影响现有 skill 的文档解析路径

---

## 技术设计

### Tool 注册

在 `internal/tools/` 新增三个 tool entry，遵循现有 `init()` + `Register()` 模式：

```go
// internal/tools/generate_xlsx.go
func init() {
    Register(&ToolEntry{
        Name:    "generate_xlsx",
        Toolset: "document_generation",
        Schema:  generateXlsxSchema,
        Handler: handleGenerateXlsx,
        Emoji:   "📊",
    })
}

// internal/tools/generate_docx.go
func init() {
    Register(&ToolEntry{
        Name:    "generate_docx",
        Toolset: "document_generation",
        Schema:  generateDocxSchema,
        Handler: handleGenerateDocx,
        Emoji:   "📄",
    })
}

// internal/tools/generate_pptx.go
func init() {
    Register(&ToolEntry{
        Name:    "generate_pptx",
        Toolset: "document_generation",
        Schema:  generatePptxSchema,
        Handler: handleGeneratePptx,
        Emoji:   "📑",
    })
}
```

### 执行路径

#### xlsx (Go in-process)

```
Agent → generate_xlsx handler → excelize API → 写入临时文件
    → objstore.Upload(ctx, tmpFile, "artifacts/{tenant}/{task}/output.xlsx")
    → 返回 {"artifact_url": "https://..."}
```

#### docx / pptx (Python sandbox)

```
Agent → generate_docx handler → 构造 Python 脚本
    → executeViaEnvironment(ctx, sandboxMode, "python", script, cfg)
    → sandbox 内: python-docx 生成 → 写入 /tmp/output.docx
    → handler 读取 sandbox 输出路径
    → objstore.Upload(ctx, outputBytes, "artifacts/{tenant}/{task}/output.docx")
    → 返回 {"artifact_url": "https://..."}
```

### 文件产出流

```
┌─────────────────────────────────────────────────────────┐
│  Go Process                                             │
│  ┌───────────┐    ┌──────────────┐    ┌─────────────┐  │
│  │  Handler  │───▶│  excelize /  │───▶│  objstore   │  │
│  │           │    │  sandbox     │    │  .Upload()  │  │
│  └───────────┘    └──────────────┘    └──────┬──────┘  │
└──────────────────────────────────────────────┼──────────┘
                                               │
                                               ▼
                                        ┌─────────────┐
                                        │    MinIO     │
                                        │  (S3 API)   │
                                        └──────┬──────┘
                                               │
                                               ▼
                                        artifact URL
                                        返回给 Agent
```

### 资源限制与错误处理

| 约束项 | xlsx (in-process) | docx/pptx (sandbox) |
|--------|-------------------|---------------------|
| 内存 | Go 进程堆内存，受 pod limits 约束 | cgroup 512MB |
| CPU | 共享 Go runtime | cgroup 1 core |
| 超时 | 30s (context deadline) | 120s (sandbox max) |
| 文件大小 | 上传前校验 <= 50MB | sandbox tmpfs 限制 |
| 行/页数 | xlsx <= 100,000 行 | docx <= 200 页, pptx <= 100 slides |

错误处理策略：

1. **输入校验失败**：返回 `{"error": "...", "hint": "..."}` 格式，附带建议
2. **sandbox 超时**：返回超时错误 + 建议减少数据量
3. **sandbox OOM**：捕获 exit code 137，返回内存不足提示
4. **上传失败**：重试 1 次，仍失败则返回临时错误码供 Agent 重试
5. **excelize 写入错误**：直接返回结构化错误，Agent 可据此调整参数

---

## 企业内控补充

| 项目 | 说明 |
|------|------|
| 应用等级 | T3（SaaS 平台核心能力，但非主营链路唯一入口） |
| 技术架构等级 | T3（单实例 + 水平扩容，无跨地域要求） |
| 关键组件偏离 | 无偏离；excelize (Apache-2.0) 和 python-docx/python-pptx (MIT) 均为社区主流选择 |
| 资源隔离 | Python 执行走已有 sandbox 隔离（docker/k8s-job），Go 侧走 pod resource limits |
| 资产文档入口 | 本 ADR + tool schema 自描述 |

---

## 后续动作

| 动作 | Owner | 完成条件 |
|------|-------|----------|
| 实现 `generate_xlsx` tool handler | backend-engineer | 单元测试通过 + schema 注册 |
| 实现 `generate_docx` tool handler | backend-engineer | sandbox 集成测试通过 |
| 实现 `generate_pptx` tool handler | backend-engineer | sandbox 集成测试通过 |
| 预装 python-docx/python-pptx 到 sandbox 镜像 | devops-engineer | 镜像构建 CI 通过 |
| 更新 skills/productivity/ 文档引用新 tool | backend-engineer | SKILL.md 指向 tool name |
| 补充 generate_* tool 的集成测试 | qa-engineer | 覆盖正常/超时/OOM 三类路径 |
