# Arch Design: WorkBuddy 能力对齐

| 字段 | 值 |
|------|-----|
| 状态 | draft |
| 主责 | architect |
| 日期 | 2026-07-06 |

## 系统边界

```
┌────────────────────────────────────────────────────────────────┐
│                        HermesX SaaS API                         │
│                                                                 │
│  ┌─────────┐  ┌──────────┐  ┌──────────┐  ┌───────────────┐  │
│  │ WebUI   │  │ Platform │  │ SDK/API  │  │ Cron Scheduler│  │
│  │/workspace│  │ Adapters │  │ Clients  │  │               │  │
│  └────┬────┘  └────┬─────┘  └────┬─────┘  └───────┬───────┘  │
│       │             │              │                │           │
│  ┌────▼─────────────▼──────────────▼────────────────▼────────┐ │
│  │                   9-Layer Middleware Stack                  │ │
│  └────────────────────────────┬──────────────────────────────┘ │
│                               │                                 │
│  ┌────────────────────────────▼──────────────────────────────┐ │
│  │                      Agent Runtime                         │ │
│  │                                                            │ │
│  │  ┌──────────┐  ┌──────────────┐  ┌────────────────────┐  │ │
│  │  │ Eino     │  │ Tool Registry│  │ Parallel Task Mgr  │  │ │
│  │  │ TurnLoop │  │              │  │ (per-session lock)  │  │ │
│  │  └──────────┘  │ ┌──────────┐ │  └────────────────────┘  │ │
│  │                 │ │ existing │ │                           │ │
│  │                 │ │ 30+ tools│ │                           │ │
│  │                 │ ├──────────┤ │                           │ │
│  │                 │ │ NEW:     │ │                           │ │
│  │                 │ │ gen_xlsx │ │                           │ │
│  │                 │ │ gen_docx │ │                           │ │
│  │                 │ │ gen_pptx │ │                           │ │
│  │                 │ │ research │ │                           │ │
│  │                 │ └──────────┘ │                           │ │
│  │                 └──────────────┘                           │ │
│  └────────────────────────────────────────────────────────────┘ │
│                               │                                 │
│  ┌────────────────────────────▼──────────────────────────────┐ │
│  │                      Storage Layer                         │ │
│  │                                                            │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────────────────┐   │ │
│  │  │PostgreSQL│  │  Redis   │  │     MinIO / S3       │   │ │
│  │  │ (RLS)    │  │  (Lock)  │  │                      │   │ │
│  │  │          │  │          │  │ {t}/{u}/workspace/   │   │ │
│  │  │ tenants  │  │ session  │  │ {t}/{u}/sessions/s/  │   │ │
│  │  │ sessions │  │  locks   │  │                      │   │ │
│  │  │ files*   │  │          │  │ [TTL for sessions]   │   │ │
│  │  └──────────┘  └──────────┘  └──────────────────────┘   │ │
│  └────────────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────┘
```

## 组件拆分

### 新增后端模块

| 模块 | 职责 | 包路径 |
|------|------|--------|
| generate_spreadsheet | Go in-process xlsx 生成 (excelize) | `internal/tools/spreadsheet.go` |
| generate_document | Python sandbox docx 生成 | `internal/tools/document.go` |
| generate_presentation | Python sandbox pptx 生成 | `internal/tools/presentation.go` |
| deep_research | 链式研究工作流编排 | `internal/tools/research.go` |
| file_workspace | Hybrid 文件管理 API | `internal/api/files.go` |
| parallel_task_mgr | 多任务并行控制 | `internal/agent/parallel.go` (已有，需扩展) |

### 新增前端模块

| 模块 | 职责 | 路径 |
|------|------|------|
| WorkspaceLayout | 三栏容器 + Splitter | `webui/src/user/components/WorkspaceLayout.tsx` |
| TaskSidebar | 任务列表 + 分组 + 搜索 | `webui/src/user/components/TaskSidebar.tsx` |
| DialogArea | 对话容器（复用） | `webui/src/user/components/DialogArea.tsx` |
| ResultsPanel | 产物列表 + 预览 + 下载 | `webui/src/user/components/ResultsPanel.tsx` |
| PlanSteps | 任务规划步骤展示 | `webui/src/user/components/PlanSteps.tsx` |
| FilePreview | 多格式文件预览 | `webui/src/user/components/FilePreview.tsx` |
| workspaceStore | 多任务状态管理 | `webui/src/shared/stores/workspaceStore.ts` |

## 关键数据流

### 办公产物生成流

```
User "生成月度报告" → Agent → plan: [搜索数据, 生成xlsx, 生成docx]
                              │
                              ▼
                    Tool: generate_spreadsheet
                              │
                    ┌─────────▼──────────┐
                    │ excelize (in-proc)  │
                    │ → write /tmp/out.xlsx│
                    │ → upload MinIO      │
                    └─────────┬──────────┘
                              │
                              ▼
                    Tool: generate_document
                              │
                    ┌─────────▼──────────┐
                    │ sandbox Python      │
                    │ python-docx script  │
                    │ → write /tmp/out.docx│
                    │ → upload MinIO      │
                    └─────────┬──────────┘
                              │
                              ▼
                    SSE event: {type:"artifact", url:"...", name:"report.docx"}
                              │
                              ▼
                    ResultsPanel: 显示产物 + 预览
```

### 多任务并行流

```
User 创建 Task A ──→ Session A ──→ SSE Stream A (background)
User 创建 Task B ──→ Session B ──→ SSE Stream B (background)
User 切换到 Task A ──→ 恢复 Stream A 的缓存消息
                       Stream B 继续后台执行
```

## 接口约定

### 新增 API

| Method | Path | 说明 |
|--------|------|------|
| GET | /v1/sessions/active | 返回进行中的 session 列表 |
| GET | /v1/sessions/{id}/artifacts | 返回 session 产物列表 |
| POST | /v1/files/upload | 上传文件到 workspace |
| GET | /v1/files | 列出 workspace 文件 |
| DELETE | /v1/files/{id} | 删除 workspace 文件 |
| POST | /v1/files/{id}/promote | session 文件提升到 workspace |
| GET | /v1/files/{id}/download | 获取签名下载 URL |

### SSE 扩展事件类型

| type | 说明 |
|------|------|
| `plan_step` | Agent 规划步骤（id, title, status） |
| `plan_step_update` | 步骤状态更新 |
| `artifact` | 产物生成完成（name, type, url, size） |
| `artifact_progress` | 产物生成进度 |

## 技术选型

| 领域 | 选型 | 原因 |
|------|------|------|
| xlsx 生成 | excelize v2 (Go) | Apache-2.0, 17k stars, in-process |
| docx 生成 | python-docx (Python sandbox) | MIT, 成熟 |
| pptx 生成 | python-pptx (Python sandbox) | MIT, 成熟 |
| 三栏布局 | Ant Design 6 Splitter | 原生支持，无新依赖 |
| 状态管理 | Zustand 5 (workspaceStore) | 已在用，轻量 |
| 文件预览 | react-file-viewer + xlsx-preview | 多格式支持 |
| 对象存储 | MinIO (已集成) | 无新增 |

## 风险与约束

| 风险 | 影响 | 缓解 |
|------|------|------|
| Python sandbox 超时/OOM | 大文件生成失败 | cgroup 限制 + 降级提示 + 分块生成 |
| SSE 连接池 | 并发用户 x 任务数 | spike 评估，必要时引入 multiplexing |
| MinIO 存储增长 | 成本 | session TTL 自动清理 + workspace 配额 |
| 文件预览兼容性 | 部分格式渲染不完美 | 提供下载兜底 + 渐进增强预览 |
