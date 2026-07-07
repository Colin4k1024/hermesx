# Launch Acceptance: WorkBuddy 能力对齐

| 字段 | 值 |
|------|-----|
| 状态 | accepted |
| 主责 | qa-engineer + tech-lead |
| 日期 | 2026-07-06 |

## 验收概览

| 字段 | 值 |
|------|-----|
| 验收对象 | WorkBuddy 能力对齐 — Slice 0-7 |
| 验收方式 | 构建验证 + 单元测试 + 代码审查 + 安全审计 + 前端质量门禁 |
| 验收角色 | qa-engineer, tech-lead |

## 验收范围

### 功能验收

| 能力 | 验收方式 | 结论 |
|------|---------|------|
| 三栏 WebUI 骨架 | 构建通过 + 组件结构审查 | ✅ 通过 |
| 多任务并行管理 | SSE 计数测试 + hook 设计审查 | ✅ 通过 |
| 办公产物生成 (xlsx/docx/pptx) | 16 个单元测试 + 安全修复验证 | ✅ 通过 |
| 结果面板 + 产物展示 | 组件审查 + API 集成验证 | ✅ 通过 |
| 任务规划步骤展示 | 事件发射测试 + UI 组件审查 | ✅ 通过 |
| 文件工作区 | 22 个测试 + 路径遍历防护 + RLS migration | ✅ 通过 |
| 深度研究工作流 | 37 个测试 + plan/compile 双模式验证 | ✅ 通过 |
| SSE 连接限制 | 计数器测试 + 429 拒绝验证 | ✅ 通过 |
| Sandbox 资源限制 | 配置加载测试 + cleanup 机制 | ✅ 通过 |

### 非功能验收

| 维度 | 验收方式 | 结论 |
|------|---------|------|
| Go build + vet | CI 验证 | ✅ 通过 |
| Frontend build | CI 验证 | ✅ 通过 |
| 安全审计 | 自动审查 + 7 项修复 | ✅ 通过（2 CRITICAL 已修复） |
| 前端质量门禁 | 组件审查 + 7 项修复 | ✅ 通过（4 BLOCK 已修复） |
| 可访问性 | aria-label + 44px + 键盘 | ✅ 通过 |
| 深色模式 | 无硬编码颜色 | ✅ 通过 |

## 风险判断

### 已满足项

- [x] 所有 CRITICAL 安全问题已修复
- [x] 所有 BLOCK 前端门禁已修复
- [x] Go build / vet / 测试全部通过
- [x] Frontend build 通过
- [x] 代码审查完成，结构合理
- [x] 数据库 migration 包含 RLS 策略

### 可接受风险

| 风险 | 影响 | 接受理由 | 监控方式 |
|------|------|---------|---------|
| K8s job sandbox 无网络隔离 | SSRF 风险 | SaaS 默认 Docker 模式，K8s 待补 | K8s 部署时 NetworkPolicy |
| 文件上传无 MIME 白名单 | 恶意文件上传 | 后续版本补白名单 | 上传日志监控 |
| 文件列表无分页 | 大量文件响应慢 | v1 功能优先 | API 响应时间监控 |
| TracedStore 未接 FileEntries | 链路追踪缺失 | 不影响功能 | 后续修复 |
| Python 包未固定版本 | 供应链风险 | 仅沙箱内安装 | 后续 pin 版本 |

### 阻塞项

**无。**

## 上线结论

**允许上线。**

前提条件全部满足，阻塞项为零。已接受风险均在可控范围内，有明确的后续修复路径和监控手段。

### 上线后观察重点

1. SSE 连接数指标（Prometheus sse_active_connections）是否在预期范围
2. 办公产物生成成功率（tool_call success rate）
3. 文件上传/下载延迟（P95 < 500ms）
4. 前端 Workspace 页面 LCP（< 2s）

### 后续版本修复清单

1. K8s job sandbox NetworkPolicy
2. MIME type 白名单
3. 文件列表分页
4. TracedStore FileEntries 接入
5. Python 包版本固定
6. Content-Disposition header 加固
7. 文件下载流式传输（io.Reader）
8. 重复工具函数合并
