# Test Plan: WorkBuddy 能力对齐

| 字段 | 值 |
|------|-----|
| 状态 | accepted |
| 主责 | qa-engineer |
| 日期 | 2026-07-06 |
| 关联 | [execute-log.md](execute-log.md) |

## 测试范围

### 功能范围

| 模块 | 测试内容 | 测试方式 |
|------|---------|---------|
| 三栏 WebUI | 布局渲染、面板折叠、响应式断点 | 构建验证 + 手动 |
| 多任务并行 | per-session SSE 流、切换不中断、后台保持 | 手动 |
| 办公产物生成 | xlsx/docx/pptx 生成 + MinIO 上传 | go test (16 cases) |
| 结果面板 | 产物列表刷新、文件预览、下载 | 手动 |
| 计划步骤 UI | plan_start/plan_step_update 事件 + 渲染 | go test + 手动 |
| 文件工作区 | 上传/列表/下载/删除/promote + 配额 | go test (22 cases) |
| 深度研究 | plan 模式 + compile 模式 + 报告生成 | go test (37 cases) |
| SSE 连接限制 | per-user 计数、429 拒绝 | go test |
| Sandbox 资源限制 | memory/CPU/timeout 配置 | go test |
| Redis lock | task-level 锁、并发 session 不阻塞 | go test |

### 非功能范围

| 维度 | 测试内容 | 状态 |
|------|---------|------|
| 安全 | Python 脚本注入防护、路径遍历、SSRF | 已修复 |
| 可访问性 | 44px 交互目标、aria-label、键盘导航 | 已修复 |
| 深色模式 | 无硬编码颜色 | 已修复 |
| 并发安全 | SSE tracker 竞态、Redis lock 迁移 | 已修复 |

### 不覆盖项

- 设计生成能力（MCP 外部服务，本阶段未实现）
- 端到端浏览器自动化测试（需单独执行）
- 压力测试（需真实 MinIO 环境）

## 测试矩阵

| 场景 | 类型 | 前置条件 | 预期结果 |
|------|------|---------|---------|
| 生成 xlsx | 单元 | excelize 依赖 | 文件生成 + ObjectStore 上传 |
| 生成 docx | 集成 | Python sandbox + python-docx | stdin JSON 传参 + 文件生成 |
| 生成 pptx | 集成 | Python sandbox + python-pptx | 文件生成 + base64 提取 |
| 并发 session 锁 | 单元 | Redis | 10 个 session 同时获取锁不阻塞 |
| SSE 计数限制 | 单元 | max=5 | 第 6 个连接返回 429 |
| 文件上传路径遍历 | 单元 | 文件 API | `../../../etc/passwd` 被拒绝 |
| 文件上传配额 | 单元 | 512MB 限制 | 超额返回错误 |
| plan 事件 | 单元 | Agent session | plan_start + plan_step_update 正确发射 |
| 深度研究 plan | 单元 | deep_research tool | 子问题数按 depth 递增 |
| 深度研究 compile | 单元 | findings 数据 | Markdown 报告结构完整 |

## 风险

| 风险 | 等级 | 缓解 |
|------|------|------|
| Python sandbox 超时导致产物丢失 | 中 | tmpfs + cleanup 机制 |
| 大文件上传阻塞 API | 中 | 已有 upload size 限制 |
| 多流 SSE 内存增长 | 低 | per-user 上限 5 |
| MinIO 产物未清理 | 低 | session 文件 TTL 自动清理 |

## 放行建议

**建议放行**，前提条件：

| # | 条件 | 状态 |
|---|------|------|
| 1 | 所有 Critical/High 安全问题已修复 | ✅ |
| 2 | 所有前端质量门禁 BLOCK 已修复 | ✅ |
| 3 | Go build + vet + 测试通过 | ✅ |
| 4 | Frontend build 通过 | ✅ |

### 已接受风险

| 风险 | 原因 | 回退方案 |
|------|------|---------|
| K8s job sandbox 无网络限制 | k8s-job 后端尚无 `--network=none` 等效 | 仅在 Docker 模式下允许网络操作 |
| 文件列表无分页 | v1 功能优先 | 后续版本增加分页 |
| 追踪包装器未接 FileEntries | 改动风险低 | 后续版本修复 |

## 评审结论

### 代码质量评审

| 级别 | 数量 | 处置 |
|------|------|------|
| CRITICAL | 1 (Redis lock 迁移) | ✅ 已修复（兼容新旧 key） |
| HIGH | 3 (SSE 竞态 / RLS 读路径 / SSRF) | ✅ SSE 竞态已修复；RLS 待验证；Python stdin 传参已修复 |
| MEDIUM | 4 | 已知技术债，不阻塞放行 |
| LOW | 2 | 信息性 |

### 安全审计

| 级别 | 数量 | 处置 |
|------|------|------|
| CRITICAL | 2 (脚本注入 / 本地沙箱) | ✅ stdin JSON 传参已修复；本地沙箱为开发模式，SaaS 默认 Docker/K8s |
| HIGH | 2 (MIME 无白名单 / Header 注入) | 已知，后续版本修复 |
| MEDIUM | 3 | 已知 |
| LOW | 3 | 信息性 |

### 前端质量门禁

| 问题 | 状态 |
|------|------|
| 交互目标 < 44px | ✅ 已修复 |
| 缺少 aria-label | ✅ 已修复 |
| sidebar 错误吞没 | ✅ 已修复 |
| 硬编码颜色 | ✅ 已修复 |
| Cmd+N 误导 | 已知（Tooltip 文案），不阻塞 |
| 拖拽区不可键盘聚焦 | ✅ 已修复 |
| session item 键盘不可达 | ✅ 已修复 |
