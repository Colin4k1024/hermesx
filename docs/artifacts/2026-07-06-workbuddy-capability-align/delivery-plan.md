# Delivery Plan: 对标 WorkBuddy 能力对齐

| 字段 | 值 |
|------|-----|
| 状态 | handoff-ready |
| 主责 | tech-lead |
| 日期 | 2026-07-06 |
| 阶段 | plan |
| 关联 PRD | [prd.md](prd.md) |
| 技术预研 | [tech-research.md](tech-research.md) |

## 版本目标

| 字段 | 值 |
|------|-----|
| 目标版本 | v2.5.0 |
| 里程碑 | WorkBuddy Capability Parity |
| 范围 | 三栏 WebUI + 多任务并行 + 办公产物生成 + 文件工作区 |
| 放行标准 | P0 全部 slice 验收通过，P1 核心 slice 验收通过 |

## 需求挑战会结论

### 核心假设调整

| # | 原假设 | 质疑 | 调整后 |
|---|--------|------|--------|
| 1 | 全量三栏重构为 P0 | 范围过大，阻塞后续 | 拆为骨架(P0a) + 打磨(P0b) |
| 2 | P0/P1 严格串行 | 后端 Tool 不依赖结果面板 | 办公产物后端与前端并行 |
| 3 | 成功指标="覆盖 8/10" | 对标指标非用户价值 | 增加任务完成率 + 留存指标 |
| 4 | FileEntry 全量写 PG | session 文件增加 RLS 复杂度 | session 文件仅 MinIO TTL，promote 时写 PG |
| 5 | SSE 按现有模式扩展 | 多流并发可能耗尽连接池 | 新增 SSE 多路复用 spike |

### 未决项

| # | 内容 | Owner | Deadline |
|---|------|-------|----------|
| 1 | Sandbox cgroup 资源限制具体参数 | architect | Slice-0 |
| 2 | 是否需要 SSE 多路复用 vs per-stream | architect | Slice-0 spike |
| 3 | 用户行为数据验证 WebUI 入口假设 | product-manager | P0b 启动前 |

## 工作拆解 — Story Slices

### Slice-0: 基础设施前置（阻断项解除）

| 字段 | 值 |
|------|-----|
| 目标 | 解除 3 个技术阻断项，使后续 slice 可并行启动 |
| 主责 | `architect` + `backend-engineer` |
| 依赖 | 无 |
| 验收标准 | sandbox 资源限制已配置并测试 / SSE 方案已 spike 并出结论 / Redis lock key 设计已文档化 |

**工作项：**
1. 定义 sandbox cgroup 限制（memory 512MB / CPU 1 core / timeout 120s）+ tmpfs 产物目录 + 失败清理机制
2. SSE 多流方案 spike：评估 per-session 独立流 vs EventSource + multiplexing，出 ADR
3. Redis lock key 重新设计：从 `{tenant}:{user}` 下沉到 `{tenant}:{session}:{tool}`
4. 文件工具安全边界：sandbox 内只允许写 `/tmp/output/`，上传到 MinIO 由宿主完成

---

### Slice-1: 三栏 WebUI 骨架 (P0a)

| 字段 | 值 |
|------|-----|
| 目标 | 新建 /workspace 路由，实现可用的三栏布局骨架 |
| 主责 | `frontend-engineer` |
| 依赖 | Slice-0 (SSE spike 结论) |
| 验收标准 | 三栏可渲染 / 侧栏显示 session 列表 / 中栏复用对话能力 / 右栏占位 / 面板可折叠 |

**工作项：**
1. 新建 `/workspace` 路由 + WorkspaceLayout 组件（Ant Design Splitter）
2. 提取 Chat.tsx → 共享 MessageList + InputBar + SessionItem 组件
3. 左栏 TaskSidebar：调用 GET /v1/sessions，按时间分组展示
4. 中栏 DialogArea：复用提取的对话组件
5. 右栏 ResultsPanel：骨架占位 + "暂无结果"空态
6. 新建 workspaceStore (Zustand)：activeSession / taskList / panelVisibility
7. 面板折叠响应式：< 1024px 自动折叠侧栏

---

### Slice-2: 多任务并行管理 (P0a)

| 字段 | 值 |
|------|-----|
| 目标 | 用户可同时运行多个任务并切换查看 |
| 主责 | `frontend-engineer` + `backend-engineer` |
| 依赖 | Slice-1 (骨架布局) |
| 验收标准 | 可同时运行 3 个独立任务 / 切换不丢失流式响应 / 各任务状态独立 |

**工作项：**
1. 后端：session 并发限制从 quota 配置读取（max_concurrent_sessions）
2. 后端：新增 GET /v1/sessions/active 返回进行中的 session 列表 + 状态
3. 前端：useSse hook 重构为 per-session 实例化（Map<sessionId, AbortController>）
4. 前端：TaskSidebar 实时状态标识（running/completed/failed/pending）
5. 前端：任务切换时保留后台 SSE 流，恢复时重播缓存消息
6. 前端：新任务创建交互（侧栏顶部 + 按钮）

---

### Slice-3: 办公产物生成 — 后端 Tool (P1, 与 Slice-1/2 并行)

| 字段 | 值 |
|------|-----|
| 目标 | 注册 3 个新 Tool，支持 docx/xlsx/pptx 生成 |
| 主责 | `backend-engineer` |
| 依赖 | Slice-0 (sandbox 限制) |
| 验收标准 | 3 个 Tool 注册成功 / 可通过 API 调用生成文件 / 产物存入 MinIO / 集成测试覆盖 |

**工作项：**
1. `generate_spreadsheet` Tool（Go in-process，excelize）
   - 支持 schema 定义 + 数据填充 + 基础图表 + 公式
   - 输出写入 MinIO，返回 object URL
2. `generate_document` Tool（Python sandbox）
   - 调用 python-docx：标题/段落/表格/图片
   - sandbox 内执行，产物从 tmpfs 上传到 MinIO
3. `generate_presentation` Tool（Python sandbox）
   - 调用 python-pptx：幻灯片布局/文本/图片/图表
   - 同上沙箱策略
4. Tool 注册到 registry，关联 agent 可发现
5. 集成测试：各格式生成 + 大文件边界（50 页 pptx）

---

### Slice-4: 结果面板 + 产物展示 (P1)

| 字段 | 值 |
|------|-----|
| 目标 | 右栏 ResultsPanel 展示任务产物，支持预览和下载 |
| 主责 | `frontend-engineer` |
| 依赖 | Slice-1 (骨架) + Slice-3 (产物 API) |
| 验收标准 | 产物列表实时更新 / 支持文件预览（docx/xlsx/pptx/pdf）/ 支持下载 |

**工作项：**
1. 后端：新增 GET /v1/sessions/{id}/artifacts 返回产物列表（name, type, url, size, created_at）
2. 前端：ResultsPanel 文件列表组件（Ant Design List + 文件图标）
3. 前端：文件预览组件（xlsx 渲染表格 / docx 渲染富文本 / pptx 缩略图 / PDF embed）
4. 前端：下载按钮（签名 URL 直连 MinIO）
5. 前端：产物生成中的 loading 态 + 进度指示

---

### Slice-5: 任务规划分解展示 (P1)

| 字段 | 值 |
|------|-----|
| 目标 | Agent 执行时，展示任务分解步骤和实时状态 |
| 主责 | `frontend-engineer` + `backend-engineer` |
| 依赖 | Slice-2 (多任务管理) |
| 验收标准 | 步骤列表可视 / 状态实时更新 / 步骤可展开详情 |

**工作项：**
1. 后端：Agent turn 中注入 planning step 元数据到 SSE 流（type: "plan_step"）
2. 后端：利用现有 workflow-tasks API 暴露 step 状态
3. 前端：DialogArea 内嵌 PlanSteps 组件（步骤卡片列表）
4. 前端：每步显示状态图标 + 耗时 + 工具调用摘要
5. 前端：步骤展开显示详细输入输出

---

### Slice-6: 文件工作区 (P2)

| 字段 | 值 |
|------|-----|
| 目标 | 用户可上传文件到 workspace，AI 可读取和批量处理 |
| 主责 | `backend-engineer` + `frontend-engineer` |
| 依赖 | Slice-0 + Slice-4 |
| 验收标准 | 文件上传/下载 / workspace 持久化 / session 文件 auto-TTL / promote 操作可用 |

**工作项：**
1. 后端：FileEntry model（仅 workspace 文件写 PG）+ migration
2. 后端：MinIO key 布局实现（workspace/ + sessions/）
3. 后端：POST /v1/files/upload + GET /v1/files + DELETE /v1/files/{id}
4. 后端：promote endpoint（session file → workspace）
5. 后端：session cleanup job 增加 MinIO prefix 删除
6. 后端：文件工具重构，路径解析为 session 虚拟根
7. 前端：文件上传组件（拖拽 + 点选）
8. 前端：ResultsPanel 内 workspace 文件浏览器 tab
9. 租户配额：MaxStorageMB 限制 + 超额提示

---

### Slice-7: 深度研究工作流 (P2)

| 字段 | 值 |
|------|-----|
| 目标 | 支持结构化多源研究并输出引用报告 |
| 主责 | `backend-engineer` |
| 依赖 | Slice-3 (文档生成用于报告输出) |
| 验收标准 | 可执行多源搜索 / 输出 markdown 报告 + docx / 包含引用来源 |

**工作项：**
1. `deep_research` Tool：编排 web_search + browse + summarize 链式调用
2. 报告模板：结构化 markdown → 通过 generate_document 输出 docx
3. 引用追踪：每个信息片段关联来源 URL
4. Skill 扩展：注册为可发现的 research skill

---

## 并行化策略

```
Week 1-2:  ┌─ Slice-0 (基础设施) ─────────────────────────┐
           │                                                │
Week 2-4:  ├─ Slice-1 (三栏骨架) ──┐                       │
           │                        ├─ Slice-2 (多任务) ──┐ │
           ├─ Slice-3 (产物后端) ───┤                     │ │
           │                        │                     │ │
Week 4-6:  │                        ├─ Slice-4 (结果面板) │ │
           │                        ├─ Slice-5 (规划展示) │ │
           │                        │                     │ │
Week 6-8:  │                        └─ Slice-6 (文件工作区)│ │
           │                           Slice-7 (深度研究)  │ │
           └───────────────────────────────────────────────┘ │
```

**关键并行点：**
- Slice-1/2（前端）与 Slice-3（后端）完全并行
- Slice-4 在 Slice-1 + Slice-3 完成后启动
- Slice-6/7 与 Slice-4/5 可部分重叠

## 角色分工

| 角色 | Slice 负责 | 交接顺序 |
|------|-----------|----------|
| `architect` | Slice-0 (设计 + ADR) | → backend-engineer |
| `backend-engineer` | Slice-0(实现) / 3 / 5(后端) / 6(后端) / 7 | → frontend-engineer |
| `frontend-engineer` | Slice-1 / 2 / 4 / 5(前端) / 6(前端) | → qa-engineer |
| `qa-engineer` | 每个 Slice 完成后验收 | → tech-lead |
| `tech-lead` | 全程收口、冲突仲裁、放行决策 | - |

## 风险与依赖

| 风险 | 等级 | 缓解 | Owner |
|------|------|------|-------|
| SSE 多流连接池耗尽 | High | Slice-0 spike 出结论，必要时引入 multiplexing | architect |
| Python sandbox OOM | Medium | cgroup 512MB + timeout 120s + 降级提示 | backend-engineer |
| 三栏 UI 范围膨胀 | Medium | P0a 只交付骨架，打磨后置 | tech-lead |
| excelize 不支持复杂图表 | Low | 降级为 Python openpyxl 兜底 | backend-engineer |
| WebUI 不是主入口 | Medium | P0b 前收集用户数据验证 | product-manager |

## 检查节点

| 节点 | 时间 | 内容 | 参与者 |
|------|------|------|--------|
| Slice-0 完成 | Week 2 | 阻断项全部解除，ADR 产出 | architect + tech-lead |
| P0a 交付 | Week 4 | 三栏骨架 + 多任务可用 | frontend + qa + tech-lead |
| P1 后端就绪 | Week 4 | 3 个 Tool 测试通过 | backend + qa |
| P1 集成 | Week 6 | 产物面板 + 规划展示端到端 | 全员 |
| P2 交付 | Week 8 | 文件工作区 + 研究工作流 | 全员 |
| 放行评审 | Week 8 | 全量回归 + 性能 + 安全 | qa + tech-lead |

## 是否需要 ADR

**是。** 需输出以下 ADR：

| ADR | 内容 | 触发 Slice |
|-----|------|-----------|
| ADR-001 | SSE 多流并发方案选型 | Slice-0 |
| ADR-002 | 文件工作区 Hybrid 模型与 MinIO key 设计 | Slice-0 |
| ADR-003 | 办公产物 Tool 技术路线（Go + Python 混合） | Slice-3 |

## 技能装配清单

| 技能 | 类型 | 触发原因 | 主责角色 |
|------|------|---------|---------|
| `frontend-engineering` | shared | WebUI 三栏重构 | frontend-engineer |
| `frontend-ui-ux-system` | shared | 工作台 UI 设计规范 | frontend-engineer |
| `golang-patterns` | shared | 新 Tool 实现 | backend-engineer |
| `api-design` | shared | 新增 API 端点 | architect |
| `tdd-workflow` | ECC | 工具函数覆盖率 | backend-engineer |

## 前端交付物与检查点

| 交付物 | Slice | 状态 |
|--------|-------|------|
| WorkspaceLayout 组件 | Slice-1 | 待实现 |
| TaskSidebar 组件 | Slice-1 | 待实现 |
| DialogArea（提取复用） | Slice-1 | 待实现 |
| ResultsPanel 骨架 | Slice-1 | 待实现 |
| useSse per-session hook | Slice-2 | 待实现 |
| FilePreview 组件 | Slice-4 | 待实现 |
| PlanSteps 组件 | Slice-5 | 待实现 |
| FileUploader 组件 | Slice-6 | 待实现 |
| ui-review-checklist | Slice-4 完成时 | 待提交 |

## Implementation Readiness

| 条件 | 状态 |
|------|------|
| PRD 已锁定 | ✅ |
| 技术预研完成 | ✅ |
| 需求挑战会完成 | ✅ |
| 3 项技术决策已确认 | ✅ |
| 阻断项已识别并有解除路径 | ✅ (Slice-0) |
| 后端 API 契约可先行 | ✅ (workflow API 已有) |
| 前端技术栈确认 | ✅ (React 19 + Ant Design 6 Splitter) |

**结论：Plan handoff-ready，可进入 execute 阶段。Slice-0 为首要启动项。**
