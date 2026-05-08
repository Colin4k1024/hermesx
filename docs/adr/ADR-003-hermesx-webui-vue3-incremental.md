# ADR-003: hermesx-webui 技术栈选型 — Vue 3 增量演进 vs React 18 重写

## 决策信息

| 字段 | 值 |
|------|-----|
| 编号 | ADR-003 |
| 标题 | hermesx-webui 前端技术栈：Vue 3 增量演进 |
| 状态 | Accepted |
| 日期 | 2026-05-08 |
| Owner | tech-lead |
| 关联需求 | docs/artifacts/2026-05-08-hermesx-webui/prd.md |

## 背景与约束

- PRD 原始技术范围指定 React 18 + Zustand + TanStack Query + Tailwind。
- 在 /team-plan 挑战会中发现 `webui/` 目录已存在 Vue 3 + Pinia + Naive UI + Vue Router v4 项目（~484 行），具备：
  - 可用的 auth store（已使用 sessionStorage，已从 `/v1/me` 响应获取 tenant_id，未错误注入 header）
  - useApi composable（Bearer token + X-Hermes-User-Id，401/403 自动跳转）
  - AppShell 布局、ChatPage、MemoriesPage、SkillsPage、AdminTenantsPage 骨架
  - Vue Router v4 路由守卫结构
- PRD 写作时尚未探知该目录存在，React 18 选型建立在"从零开始"的假设上。
- 非目标：本 ADR 不影响后端 Go 实现。

## 备选方案

### A: Vue 3 增量演进（**采纳**）

复用现有 `webui/` 基础，做以下调整：

| 变更点 | 内容 |
|--------|------|
| 保留 | Vue 3 + Pinia + Vue Router v4 + Naive UI + Vite + TypeScript |
| 新增 | `@tanstack/vue-query` v5（替代 stores 中的裸 fetch 状态管理）|
| 新增 | Tailwind CSS v4（补充 Naive UI 覆盖不到的自定义样式区域）|
| 重命名 | auth store 中 `acpToken` → `adminApiKey`（ACP 是编辑器协议，不是 admin key）|
| 结构调整 | Vite multi-page 模式（见 ADR-004）|

**优点：**
- 减少 2–3 天基础设施重建工作
- 现有 auth/API 层已解决挑战会提出的安全问题（sessionStorage ✓, tenant_id from response ✓）
- Naive UI DataTable/Form/Modal 对企业级 Admin Console 覆盖优于裸 Tailwind 组件
- `@tanstack/vue-query` v5 与 React Query API 基本一致，学习成本低
- Vue 3 Composition API 与 React hooks 概念对齐，切换成本低

**风险：**
- Naive UI 偶有 TypeScript 类型不完整问题（可通过 `// @ts-expect-error` 局部处理）
- `@tanstack/vue-query` 社区规模略小于 React Query（文档完整，可接受）

### B: React 18 重写

**原因不采纳：**
- 484 行骨架代码须全部废弃，所有页面从零构建
- PRD 规格基于错误前提（"从零开始"），发现既有代码后原因已失效
- Tailwind 需要自建 DataTable、Modal、Form 等组件，增加 1–2 周工作量
- 在功能对等前提下，两个方案的最终产品体验无可见差异

## 决策结果

**采纳方案 A：Vue 3 增量演进。**

最终技术栈：
```
Vue 3 + Pinia + Vue Router v4 + Naive UI
+ @tanstack/vue-query v5
+ Tailwind CSS v4
+ Vite 6 (multi-page mode)
+ TypeScript strict
```

影响范围：
- `webui/package.json`：新增 `@tanstack/vue-query`、`tailwindcss`
- `webui/src/stores/auth.ts`：`acpToken` 字段重命名为 `adminApiKey`，更新相关引用
- `webui/src/composables/useApi.ts`：admin 请求从 `acpToken` 切换到 `adminApiKey`
- 兼容性：现有 useApi / auth 模式不破坏，只扩展

## 企业内控补充

- 应用等级：内部工具（T4），无强制集团框架约束
- 无集团前端框架白皮书限制

## 后续动作

| 动作 | Owner | 完成条件 |
|------|-------|---------|
| 更新 auth.ts: acpToken → adminApiKey | frontend-engineer | Phase 0 完成 |
| 安装 @tanstack/vue-query + tailwindcss | frontend-engineer | Phase 0 完成 |
| 配置 Vite multi-page（见 ADR-004）| frontend-engineer | Phase 0 完成 |
