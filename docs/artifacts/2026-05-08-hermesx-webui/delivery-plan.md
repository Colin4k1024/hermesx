# Delivery Plan: hermesx-webui

**版本**: v0.1  
**日期**: 2026-05-08  
**Owner**: tech-lead  
**关联 PRD**: docs/artifacts/2026-05-08-hermesx-webui/prd.md  
**关联架构**: docs/artifacts/2026-05-08-hermesx-webui/arch-design.md  
**状态**: handoff-ready（challenge ✓，design review ✓，ADR ✓）

---

## 版本目标

| 目标 | 放行标准 |
|------|---------|
| Admin Console 全功能 | 5 个 admin 模块（租户/Key/审计/定价/沙箱）全部可用，操作 ≤ 3 步 |
| User Agent Portal 全功能 | SSE chat、会话管理、记忆、技能浏览、用量统计均可用 |
| 登录 + Bootstrap | User/Admin 独立登录页，Bootstrap 引导页完整流程 |
| 后端接口补全 | GET /admin/v1/tenants/{id}/api-keys、POST /admin/v1/bootstrap、GET /admin/v1/bootstrap/status |
| 旧 HTML 下线 | chat.html/admin.html/isolation-test.html/regression.html 从镜像中移除 |
| 独立部署 | `npm run build` 产出静态文件，Dockerfile nginx:alpine，K8s NodePort 30081 |
| CI | lint + typecheck + build 集成到 GitHub Actions |

---

## 需求挑战会结论（2026-05-08）

| # | 挑战人 | 挑战内容 | 结论 |
|---|--------|---------|------|
| 1 | product-manager | PRD 指定 React 18，但 webui/ 已有 Vue 3 工作代码 | 采纳 Vue 3 增量演进（ADR-003）|
| 2 | product-manager | localStorage 安全风险 | 现有代码已用 sessionStorage ✓，无需修改 |
| 3 | project-manager | GET /admin/v1/tenants/{id}/api-keys 缺口必须是 Phase 0，不能并行 | 已排入 Phase 0 后端任务 |
| 4 | project-manager | isolation-test.html 可能是 CI 依赖，下线前需排查 | Phase 3 前执行 CI 依赖排查 |
| 5 | architect | ACP Token 是编辑器协议，需独立 Bootstrap 端点 | 新增 POST /admin/v1/bootstrap（ADR-005）|
| 6 | architect | X-Hermes-Tenant-Id 不应由前端发送，tenant_id 从 API key 推导 | 现有 useApi.ts 已正确实现 ✓ |
| 7 | architect | SSE 需 nginx proxy_buffering off + proxy_read_timeout 300s | 已写入 arch-design.md + ADR-004 |
| 8 | architect | Vite multi-page 优于单 SPA | 采纳（ADR-004）|

**2 个已解决的硬阻塞：**
1. ~~Tech stack ADR 未决~~ → ADR-003 已定：Vue 3 增量
2. ~~ACP Token 误用为 admin bootstrap~~ → ADR-005 已定：独立 bootstrap 端点

---

## 工作拆解

### Phase 0 — 基础 & 后端补全（1 周）

**Owner**: backend-engineer（后端）+ frontend-engineer（前端脚手架）  
**目标**: 所有后续阶段的地基，阻塞后续工作的接口必须在此阶段完成。

| ID | 任务 | 主责 | 依赖 | 验收标准 |
|----|------|------|------|---------|
| P0-B1 | 实现 GET /admin/v1/bootstrap/status | backend | — | 无 admin key 时返回 `{bootstrap_required: true}` |
| P0-B2 | 实现 POST /admin/v1/bootstrap | backend | P0-B1 | ACP token 验证，原子检查，返回明文 key 一次 |
| P0-B3 | 实现 GET /admin/v1/tenants/{id}/api-keys | backend | — | 返回掩码 key 列表，key_hash 不出现在响应 |
| P0-F1 | Vite multi-page 重组（ADR-004）| frontend | — | `npm run build` 产出 index.html + admin.html |
| P0-F2 | 安装依赖：@tanstack/vue-query v5 + tailwindcss | frontend | P0-F1 | `npm run typecheck` 通过 |
| P0-F3 | auth.ts: acpToken → adminApiKey 重命名 | frontend | P0-F2 | 全量搜索 acpToken 为空 |
| P0-F4 | User Portal 登录页（LoginPage.vue）| frontend | P0-F2 | 输入 key+uid → POST /v1/me → 跳转 /chat |
| P0-F5 | Admin Console 登录页（AdminLoginPage.vue）| frontend | P0-F2 | 输入 admin key → 验证 roles["admin"] → 跳转 /tenants |
| P0-F6 | Bootstrap 引导页骨架（BootstrapPage.vue）| frontend | P0-B1,P0-B2 | bootstrap/status 检查 → 展示引导或跳转登录 |
| P0-CI | GitHub Actions: lint + typecheck + build | frontend | P0-F1 | PR CI 通过 |

**Phase 0 验收门禁：**
- [ ] GET /admin/v1/bootstrap/status + POST /admin/v1/bootstrap 可联调
- [ ] GET /admin/v1/tenants/{id}/api-keys 返回正确结构
- [ ] `npm run build` 产出两个入口文件
- [ ] 两个登录页可完成登录流程

---

### Phase 1 — Admin Console（1.5 周）

**Owner**: frontend-engineer  
**依赖**: Phase 0 全部完成  
**目标**: 5 个 Admin 模块全部上线，所有操作 ≤ 3 步完成。

| ID | 任务 | 描述 | 验收标准 |
|----|------|------|---------|
| P1-A1 | 租户管理页（TenantsPage.vue）| 列表分页 + 创建（Drawer）+ 编辑 + 删除（二次确认）| 对接 /v1/tenants CRUD，TanStack Query 缓存刷新 |
| P1-A2 | API Key 管理页（ApiKeysPage.vue）| 按租户筛选 + 创建 + Rotate + Revoke | 对接 P0-B3 + 现有 POST/DELETE 端点，prefix 掩码展示 |
| P1-A3 | 审计日志页（AuditLogsPage.vue）| 时间范围 + 租户筛选 + 分页表格 | 对接 GET /admin/v1/audit-logs，加载 < 2s |
| P1-A4 | 定价规则页（PricingPage.vue）| 规则列表 + 新增 + 修改 + 删除 | 对接 /admin/v1/pricing-rules |
| P1-A5 | 沙箱策略页（SandboxPage.vue）| 按租户查看/设置/清除 sandbox policy | 对接 /admin/v1/tenants/{id}/sandbox-policy |
| P1-A6 | Bootstrap 引导页完整流程 | ACP token 输入 + 创建 key + 一次性展示 | 对接 P0-B1/B2，key 展示后无法再查 |

**Phase 1 验收门禁：**
- [ ] 所有 Admin 页面 loading/empty/error/success 四态完整
- [ ] 删除/Revoke 操作均有二次确认
- [ ] 敏感 key 全部掩码展示

---

### Phase 2 — User Portal（1.5 周）

**Owner**: frontend-engineer  
**依赖**: Phase 0 完成（共享 auth/API 层）  
**目标**: SSE chat + 完整用户功能。

| ID | 任务 | 描述 | 验收标准 |
|----|------|------|---------|
| P2-U1 | SSE Chat（ChatPage.vue 重构）| fetch+ReadableStream，token 逐步渲染，AbortController 停止 | 首 token < 200ms 显示，Markdown 渲染 |
| P2-U2 | 会话列表侧栏 | 左侧列表 + 新建 + 切换 + 删除 | 对接 GET/DELETE /v1/sessions |
| P2-U3 | 记忆管理页（MemoriesPage.vue）| 列表 + 删除指定记忆 | 对接 GET/DELETE /v1/memories |
| P2-U4 | 技能浏览页（SkillsPage.vue）| 列表 + 详情 Modal | 对接 GET /v1/skills, /v1/skills/{name} |
| P2-U5 | 身份与用量页（UsagePage.vue）| tenant_id/roles/scopes + 本月 token 用量 | 对接 GET /v1/me + GET /v1/usage |

**Phase 2 验收门禁：**
- [ ] SSE 连接中断后自动重连（最多 3 次）
- [ ] 会话切换不丢失消息历史
- [ ] 所有页面四态完整

---

### Phase 3 — 集成 & 下线（0.5 周）

**Owner**: frontend-engineer + backend-engineer + devops  
**依赖**: Phase 1 + Phase 2 完成，验收通过

| ID | 任务 | 主责 | 验收标准 |
|----|------|------|---------|
| P3-1 | nginx.conf SSE 配置（proxy_buffering off 等）| frontend | SSE 在 nginx 代理后正常流式 |
| P3-2 | Dockerfile（nginx:alpine）+ K8s NodePort 30081 | frontend | `docker build + run` 可访问两个入口 |
| P3-3 | CI 依赖排查：isolation-test.html/regression.html 是否被 CI 引用 | devops | 确认无 CI 依赖，或替换 CI 测试用例 |
| P3-4 | 旧 HTML 下线（chat.html/admin.html/isolation-test.html/regression.html）| backend | Dockerfile.saas 不再 COPY 旧 HTML |
| P3-5 | 后端 index.html 重定向新 UI | backend | GET / → 302/301 到新 UI 入口 |
| P3-6 | CORS 配置：dev port 加入 SAAS_ALLOWED_ORIGINS | devops | 开发环境 Admin API 无跨域错误 |
| P3-7 | 全链路验收（Admin + User + Bootstrap）| qa | launch-acceptance.md 完成 |

---

## 角色分工

| 角色 | 职责 |
|------|------|
| tech-lead | ADR 决策、风险收口、Phase 0 门禁放行、下线决策 |
| backend-engineer | P0-B1/B2/B3（3 个新端点），P3-4/P3-5（旧 HTML 下线）|
| frontend-engineer | P0-F1~F6、P1-A1~A6、P2-U1~U5、P3-1/P3-2 |
| devops-engineer | P3-3（CI 依赖排查）、P3-6（CORS）|
| qa-engineer | Phase 1 + Phase 2 四态验收、P3-7 全链路验收 |

---

## 节点检查

| 节点 | 条件 | 日期（目标）|
|------|------|------------|
| Phase 0 完成 | 后端 3 接口可用 + 前端双入口 + 两个登录页 | W+1 |
| Phase 1 完成 | Admin 5 模块全部验收通过 | W+2.5 |
| Phase 2 完成 | User Portal 全部验收通过 | W+4 |
| Phase 3 完成 + 旧 HTML 下线 | CI 绿、全链路验收通过、旧 HTML 不可访问 | W+4.5 |

---

## 风险与缓解

| 风险 | 影响 | 缓解 | Owner |
|------|------|------|-------|
| isolation-test.html 被 CI 引用 | Phase 3 下线阻塞 | Phase 3 前执行 grep 排查，先替换 CI 用例再下线 | devops |
| SSE 在 K8s ingress 断流 | 用户体验降级 | 实现 useReconnectingSse（最多 3 次重试 + 指数退避）| frontend |
| CORS 开发阶段阻塞 | Admin API 无法测试 | 本地开发：vite.config.ts proxy 配置转发 /v1 和 /admin/v1 | frontend |
| POST /admin/v1/bootstrap 并发竞争 | 创建多个 admin key | 后端 DB 事务 + 原子检查（see ADR-005）| backend |
| Naive UI TypeScript 类型缺失 | `npm run typecheck` 报错 | `@ts-expect-error` 局部标注，不影响 strict 模式整体 | frontend |

---

## Implementation Readiness（执行前提证据）

- [x] PRD v0.1 ready-for-plan（docs/artifacts/2026-05-08-hermesx-webui/prd.md）
- [x] 需求挑战会完成（3 个角色，8 条挑战全部收敛）
- [x] ADR-003（Vue 3 vs React 18）已决策并记录
- [x] ADR-004（Vite multi-page）已决策并记录
- [x] ADR-005（Bootstrap 端点）已决策并记录
- [x] arch-design.md 完成（系统边界、数据流、接口约定、nginx 配置）
- [x] 2 个硬阻塞均已解决
- [ ] Phase 0 后端 API 联调完成（Phase 1 准入条件）
- [ ] Phase 0 前端双入口构建验证（Phase 1 准入条件）

**就绪状态**: `handoff-ready` — 可进入 `/team-execute Phase 0`

---

## 交接记录

**当前阶段**: plan  
**目标阶段**: execute  
**accepted_by**: frontend-engineer + backend-engineer  
**阻塞项**: 无（所有硬阻塞已在挑战会中解决）

**下游质疑记录（接收方）：**
- 质疑：Bootstrap 端点安全性 — ACP token 在前端输入是否泄漏风险？  
  结论：接受原方案。ACP token 仅用于单次 POST，不存入 sessionStorage，Bootstrap 完成后不再需要。运维人员掌握 ACP token，通过 kubectl secret 管理。
- 质疑：Vue 3 multi-page 后 Pinia store 是否跨入口共享？  
  结论：Pinia store 在各自入口独立实例化，不跨 HTML 文件共享。auth store 在 user/main.ts 和 admin/main.ts 中分别创建，行为独立。
