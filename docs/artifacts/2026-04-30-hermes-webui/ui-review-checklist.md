# UI Review Checklist — hermes-webui

**Date**: 2026-04-30  
**Reviewer**: frontend-engineer  
**Scope**: hermes-webui v1.0 — ConnectPage, ChatPage, MemoriesPage, SkillsPage, AdminSkillsPage, AdminTenantsPage, AdminKeysPage  

---

## 1. 视觉一致性

- [x] 页面与组件遵循统一设计 token — 全部使用 Naive UI 内建 token（`#18a058` 主色来自 NConfigProvider，间距/圆角来自 NLayout/NCard 默认值）
- [x] 字体、颜色、间距、圆角、阴影没有散装硬编码 — inline style 仅用于布局结构（flex、width 等），颜色值只有 `#18a058`（激活状态 border + background）和 `#f5f5f5`（Assistant bubble），均保持一致
- [x] 核心页面视觉层级清晰 — 侧边栏 + 内容区双栏布局，激活项 `border-left: 3px solid #18a058` 提供层级指引

**已知偏差**：设计 token 体系依赖 Naive UI 默认值而非独立 DESIGN.md 文件，未来如需自定义品牌需补 NConfigProvider themeOverrides。

---

## 2. 交互完整性

- [x] 主路径 loading / empty / error / success 状态完整
  - ConnectPage: loading（NButton :loading）/ error（NAlert）/ success（redirect to /chat）
  - ChatPage: sessionsLoading（NSpin）/ sessionsError（NAlert + retry）/ empty（NEmpty）/ messages（列表）/ sendLoading（bubble spinner）/ sendError（NAlert + dismiss）
  - MemoriesPage: loading / error+retry / empty / data table
  - SkillsPage: listLoading / listError+retry / empty / contentLoading / contentError / content
  - AdminTenantsPage: loading / error+retry / empty / table / create modal（creating + createError）
  - AdminKeysPage: 全部状态 + 一次性密钥展示 modal（closable:false + keyCopied checkbox）
- [x] 提交、删除、保存、切换等关键动作有明确反馈
  - 删除内存：per-key loading state，不锁整表
  - 创建 API Key：二步确认（创建 modal → 一次性密钥 modal，强制勾选"已复制"）
  - 新对话：立即清空消息列表，无延迟
- [x] 导航、返回和弹层关闭路径可预测
  - NModal 关闭：Cancel 按钮 + 外部点击（KeyResult modal 除外，mask-closable:false）
  - 路由守卫：未登录 → /connect；已登录访问 /connect → /chat；非 admin 访问 admin 路由 → /chat

---

## 3. 响应式与布局

- [x] 主体布局 `display:flex; height:100vh` 在大屏（>1024px）表现正常
- [~] 小屏（<768px）：侧边栏固定宽度（220px/260px），无折叠机制。在手机屏幕上侧边栏会压缩内容区。**已知风险，当前版本定位为桌面管理工具，移动端暂不要求**
- [x] 没有非预期横向滚动 — 消息 bubble 有 `wordBreak: break-word`；内容区有 `overflow-y: auto`
- [x] 数据表格（MemoriesPage、AdminTenantsPage、AdminKeysPage）有 `ellipsis:true` 对长字段截断

---

## 4. 可访问性

- [x] 表单字段有显式标签（ConnectPage 使用 NFormItem label；AdminKeysPage 表单同上）
- [x] NButton、NInput、NDataTable 等 Naive UI 组件自带 ARIA 基线
- [~] 图标按钮暂无（当前均使用文字按钮）
- [~] 焦点 ring 使用 Naive UI 默认样式，未做额外强化
- [x] 颜色不是唯一状态信号：激活 session 同时有背景色 + 左侧 border 双重指示；错误同时有 NAlert type="error"（颜色+图标+文字）

---

## 5. 性能

- [x] 生产构建通过：`npm run build` 总 gzip 约 118KB（主包），Naive UI 按需 tree-shaken
- [x] 路由懒加载：Vite 自动代码分割，每页独立 chunk（最大 76KB gzip 24KB）
- [x] 消息列表无虚拟滚动（当前 session 消息量小，可接受）；大量历史消息场景下需补虚拟列表
- [x] `scrollToBottom` 使用 `nextTick` 在 DOM 更新后执行，无布局抖动

---

## 6. 证据

- [x] 生产构建验证：`npm run build` ✅（2821 modules, 0 errors）
- [x] TypeScript 类型检查：`npm run type-check` ✅（0 errors）
- [x] Playwright E2E 回归：**13/13 passed**（含 Soul/Memory/Skill/UI isolation 全套）
- [x] WriteTimeout 变更（60s→150s）无回归

**已知风险与下一步建议**：

| 风险 | 等级 | 建议 |
|------|------|------|
| 侧边栏无移动端折叠 | 低 | 未来版本补 NDrawer 或汉堡菜单 |
| 消息列表无虚拟滚动 | 低 | 历史消息量大时补 vue-virtual-scroller |
| 设计 token 依赖 Naive UI 默认值 | 低 | 需品牌定制时补 themeOverrides |
| 一次性 API Key 显示无自动复制按钮 | 低 | 补 NButton "Copy to clipboard" 提升体验 |

**结论：通过，可进入 QA 或发布阶段。**
