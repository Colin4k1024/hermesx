# Execute Log: WorkBuddy 能力对齐 — Phase 1 (Slice 0-3)

| 字段 | 值 |
|------|-----|
| 状态 | completed |
| 主责 | backend-engineer + frontend-engineer |
| 日期 | 2026-07-06 |
| 阶段 | execute |

## 计划 vs 实际

| 计划内容 | 实际结果 | 偏差 |
|---------|---------|------|
| Slice-0: sandbox 资源限制 | ✅ 新增 4 个环境变量 + cleanup 机制 | 无 |
| Slice-0: Redis lock key 重设计 | ✅ 新增 task-level lock API | 无 |
| Slice-1: 三栏 WebUI 骨架 | ✅ 10 个新文件 + 4 个修改 | 无 |
| Slice-2 后端: sessions/active + artifacts API | ✅ 3 个新文件 + 4 个修改 | 无 |
| Slice-2 后端: SSE 连接计数 | ✅ 集成到 rate limiter | 无 |
| Slice-3: 3 个文档生成工具 | ✅ 5 个新文件 + 16 个测试 | 无 |

## 关键决定

1. **sandbox 产物模式**: Docker 模式使用 `--network=none` 安全隔离；K8s 模式使用 base64 stdout marker protocol 从 job logs 中提取产物
2. **SSE 连接计数**: 使用 `sync.Map` + `atomic.Int32` 实现 lock-free 并发计数器，per-user 限制默认 5
3. **session API 路由复用**: 利用已有的 `/v1/sessions/` catch-all 路由，在 handler 内分发子路径，无需新增路由注册
4. **前端 workspace store**: 使用 Zustand persist middleware 持久化面板偏好到 localStorage

## 阻塞与解决

| 阻塞 | 解决方式 |
|------|---------|
| ADR-001 agent 超时 | 由 tech-lead 直接编写 ADR |
| UI 设计规范 agent 超时 | agent 实际已写入文件（30KB），仅通信超时 |

## 影响面

| 模块 | 变更类型 | 文件 |
|------|---------|------|
| `internal/tools/` | 新增 5 文件 | spreadsheet.go, document.go, presentation.go, docgen_sandbox.go, + tests |
| `internal/tools/code_exec_sandbox.go` | 修改 | sandbox 资源限制 |
| `internal/store/rediscache/redis.go` | 修改 | lock key 重设计 |
| `internal/api/sessions.go` | 新增 | active sessions + artifacts API |
| `internal/middleware/ratelimit.go` | 修改 | SSE 连接计数 |
| `internal/api/agent_chat.go` | 修改 | SSE 计数集成 |
| `webui/src/user/components/workspace/` | 新增 8 文件 | 三栏布局 + 子组件 |
| `webui/src/shared/components/` | 新增 3 文件 | MessageList, InputBar, SessionItem |
| `webui/src/shared/stores/` | 新增 1 文件 | workspaceStore |
| `webui/src/user/pages/Workspace.tsx` | 新增 | workspace 页面 |
| `webui/src/user/router.tsx` | 修改 | 注册 /workspace 路由 |

## 未完成项

| 项 | 原因 | 计划阶段 |
|---|------|---------|
| Slice-6: 文件工作区 | P2 优先级，需 FileEntry migration | Phase 2 |
| Slice-7: 深度研究工作流 | P2 优先级 | Phase 2 |

## Phase 2 执行记录（Slice 2-前端 + Slice 4 + Slice 5）

### 计划 vs 实际

| 计划内容 | 实际结果 | 偏差 |
|---------|---------|------|
| Slice-4: 结果面板产物集成 | ✅ useArtifacts/useFiles hooks + 结果面板重写 + FilePreview + SSE 自动刷新 | 无 |
| Slice-5: 任务规划步骤 UI | ✅ 后端 plan_start/plan_step_update 事件 + PlanSteps 组件 + 自动收起 | 无 |
| Slice-2 前端: useSse 多流管理 | ✅ useSseManager + DialogArea 集成 + 429/错误处理 + 后台流保持 | 无 |

### Phase 2 新增文件

| 文件 | 说明 |
|------|------|
| `webui/src/shared/hooks/useSseManager.ts` | 多流 SSE 管理器 |
| `webui/src/shared/hooks/useArtifacts.ts` | 产物列表查询 hook |
| `webui/src/shared/hooks/useFiles.ts` | 工作区文件查询 hook |
| `webui/src/user/components/workspace/PlanSteps.tsx` | 任务规划步骤组件 |
| `webui/src/user/components/workspace/FilePreview.tsx` | 文件预览组件 |

### Phase 2 修改文件

| 文件 | 说明 |
|------|------|
| `webui/src/user/components/workspace/ResultsPanel.tsx` | 重写：真实数据 + 文件图标 + 时间格式 + 预览/下载 |
| `webui/src/user/components/workspace/DialogArea.tsx` | useSseManager + plan events + artifact 刷新 |
| `webui/src/user/components/workspace/TaskSidebar.tsx` | 流式状态标识 |
| `webui/src/shared/stores/workspaceStore.ts` | PlanStep + streamingSessions |
| `webui/src/shared/hooks/useSse.ts` | onToolResult + plan events 解析 |
| `internal/eino/agent.go` | PlanStep 类型 + OnPlanStart/OnPlanStepUpdate 回调 |
| `internal/api/agent_chat.go` | plan_start + plan_step_update SSE 事件 |
| `internal/api/agent_chat_test.go` | Plan 事件测试 |

## Phase 3 执行记录（Slice 6 + Slice 7）

### 计划 vs 实际

| 计划内容 | 实际结果 | 偏差 |
|---------|---------|------|
| Slice-6: 文件工作区 | ✅ FileEntry model + PG migration + 5 个 API 端点 + FileUpload 组件 + 22 个测试 | 无 |
| Slice-7: 深度研究工作流 | ✅ deep_research 工具（plan + compile 双模式）+ 37 个测试 | 无 |

### Phase 3 新增文件

| 文件 | 说明 |
|------|------|
| `internal/api/files.go` | 文件 API handler（upload/list/download/delete/promote） |
| `internal/api/files_test.go` | 22 个测试用例 |
| `internal/store/pg/file_entries.go` | PG FileEntryStore 实现 + RLS |
| `internal/store/mysql/noop_file_entries.go` | MySQL noop 实现 |
| `internal/tools/research.go` | deep_research 工具（730 行） |
| `internal/tools/research_test.go` | 37 个测试用例 |
| `webui/src/user/components/workspace/FileUpload.tsx` | 拖拽上传组件 |

### Phase 3 修改文件

| 文件 | 说明 |
|------|------|
| `internal/store/types.go` | 新增 FileEntry model |
| `internal/store/store.go` | 新增 FileEntryStore 接口 |
| `internal/store/pg/pg.go` | 接入 pgFileEntryStore |
| `internal/store/pg/migrate.go` | 新增 migration 124-129（file_entries 表 + RLS） |
| `internal/objstore/objstore.go` | 新增 PutObjectWithContentType |
| `internal/objstore/minio.go` | 实现 PutObjectWithContentType |
| `internal/api/server.go` | 注册 /v1/files 路由 |
| `webui/src/shared/hooks/useFiles.ts` | 匹配 API 格式 + upload/delete/download hooks |
| `webui/src/user/components/workspace/ResultsPanel.tsx` | 连接真实文件 API + FileUpload 集成 |
