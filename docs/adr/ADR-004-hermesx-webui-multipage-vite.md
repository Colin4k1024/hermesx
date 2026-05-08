# ADR-004: hermesx-webui Vite Multi-Page 架构

## 决策信息

| 字段 | 值 |
|------|-----|
| 编号 | ADR-004 |
| 标题 | Vite Multi-Page：User Portal (index.html) + Admin Console (admin.html) |
| 状态 | Accepted |
| 日期 | 2026-05-08 |
| Owner | architect |
| 关联需求 | docs/artifacts/2026-05-08-hermesx-webui/prd.md |

## 背景与约束

- PRD 要求 Admin Console（`/admin/`）和 User Agent Portal（`/`）共存于同一仓库。
- 挑战会架构师建议 Vite multi-page 优于单 SPA + 路由守卫，主要原因：
  - 两个 portal 的认证机制不同（user key vs admin key）
  - bundle 独立，不因 admin 依赖污染 user bundle
  - 与现有后端 `/static/` 服务模式（`index.html` + `admin.html`）对齐
- 非目标：不拆分为两个独立 npm 项目（维护成本过高）。

## 备选方案

### A: 单 SPA + 路由守卫（不采纳）

所有页面在同一 Vue 应用中，通过 `/admin/*` 路由和路由守卫区分权限。

**不采纳原因：**
- Admin 和 User bundle 无法分离，admin 依赖（如 Audit Log 大型表格）污染 user bundle
- 路由守卫仅在前端守卫，无 bundle 级隔离
- 单 SPA 需要运行时判断"当前是 admin 还是 user"，耦合度高

### B: Vite Multi-Page（**采纳**）

两个独立 HTML 入口点：

```
webui/
├── index.html          ← User Portal 入口（打包为 /index.html）
├── admin.html          ← Admin Console 入口（打包为 /admin.html 或 /admin/index.html）
├── src/
│   ├── user/
│   │   ├── main.ts     ← User Portal app 实例
│   │   ├── App.vue
│   │   └── router.ts
│   ├── admin/
│   │   ├── main.ts     ← Admin Console app 实例
│   │   ├── App.vue
│   │   └── router.ts
│   ├── shared/         ← 两个入口共享的代码
│   │   ├── api/        ← useApi.ts, useSse.ts
│   │   ├── stores/     ← auth.ts (Pinia, 各自实例化)
│   │   ├── types/      ← TypeScript 类型
│   │   └── components/ ← 共享 UI 组件
│   └── pages/          ← 页面组件（按 user/ 和 admin/ 分目录）
└── vite.config.ts      ← build.rollupOptions.input 配置两个入口
```

vite.config.ts 关键配置：
```typescript
build: {
  rollupOptions: {
    input: {
      main: resolve(__dirname, 'index.html'),
      admin: resolve(__dirname, 'admin.html'),
    },
  },
}
```

**优点：**
- 独立 bundle，User Portal 不加载任何 Admin 代码
- 天然权限隔离：admin.html 加载 admin 应用实例，永远不与 user 混用
- nginx 路由清晰：`/admin` → `admin.html`，`/` → `index.html`
- 与后端现有 `/static/` 服务模式完全兼容

## 决策结果

**采纳方案 B：Vite Multi-Page。**

nginx 路由配置：
```nginx
location /admin {
    try_files $uri $uri/ /admin.html;
}
location / {
    try_files $uri $uri/ /index.html;
}
```

目录迁移计划（Phase 0）：
1. 将现有 `src/` 内容重组到 `src/shared/` + `src/user/` + `src/admin/`
2. 创建 `admin.html` 入口
3. 更新 `vite.config.ts` 为 multi-page

## 后续动作

| 动作 | Owner | 完成条件 |
|------|-------|---------|
| 重组目录结构，配置 vite.config.ts | frontend-engineer | Phase 0 |
| 更新 nginx.conf（SSE 配置见 ADR-006）| frontend-engineer | Phase 0 |
| 验证 `npm run build` 产出两个 HTML | qa-engineer | Phase 0 验收 |
