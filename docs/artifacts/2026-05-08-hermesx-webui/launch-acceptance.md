# Launch Acceptance — hermesx-webui

**任务**: hermesx-webui  
**日期**: 2026-05-08  
**角色**: qa-engineer  
**状态**: accepted  
**关联**: test-plan.md · delivery-plan.md

---

## 1. 验收概览

| 字段 | 内容 |
|------|------|
| 验收对象 | hermesx-webui v2.1.0 — Admin Console + User Portal |
| 验收时间 | 2026-05-08 |
| 验收角色 | qa-engineer |
| 验收方式 | 代码审查 + 静态分析 + 构建验证 |

---

## 2. 验收范围

### In Scope

- Admin Console 全部 5 页（租户/API Key/审计/定价/沙箱）
- User Portal 全部 4 页（Chat/Memories/Skills/Usage）
- Bootstrap + 登录页
- 后端安全修复（bootstrap.go + server.go）
- webui CI/CD workflow
- 旧静态 HTML 下线（chat.html, admin.html 静态版, isolation-test.html, regression.html）

### Out of Scope

- 生产环境 E2E 真实浏览器测试
- 性能基准测试
- OIDC SSO 集成

---

## 3. 验收证据

| 证据 | 结果 |
|------|------|
| `go build ./internal/api/admin/...` | ✅ 编译通过，无错误 |
| `go build ./internal/api/...` | ✅ 编译通过，无错误 |
| `bootstrap.go` subtle.ConstantTimeCompare | ✅ 字符串 `!=` 已替换 |
| `bootstrap.go` sync.Mutex TOCTOU 守卫 | ✅ mu.Lock() 包裹 check-then-create |
| `auth.ts` sessionStorage 无明文 key | ✅ setItem('hx_user_key') / setItem('hx_admin_key') 已删除 |
| `auth.ts` isAdmin 基于 adminRoles.includes('admin') | ✅ adminRoles ref 已独立追踪 |
| `useApi.ts` Authorization 空 key 守卫 | ✅ if (key) 条件已加 |
| `server.go` CORS Vary: Origin | ✅ w.Header().Add("Vary", "Origin") 已加 |
| `webui.yml` permissions 块 | ✅ contents: read; actions: write |
| `test_web_isolation.sh` Phase 1 清理 | ✅ isolation-test.html 检查已移除 |
| 旧静态文件已删除 | ✅ chat.html/admin.html/isolation-test.html/regression.html 已下线 |

---

## 4. 风险判断

### 已满足项

- 全部 CRITICAL 安全修复落地（4 项）
- 全部 HIGH 修复落地（4 项）
- Go 构建绿灯
- CI workflow 最小权限

### 可接受风险

| 风险 | 评估 | Owner |
|------|------|-------|
| Bootstrap IP 速率限制缺失 | ACP token 为必要前提，随机 32 字节 token 暴力破解不可行；接受 | v2.2.0 补 |
| useSse 401/403 auto-logout 缺失 | SSE 流中只影响错误提示，不影响数据安全；接受 | v2.2.0 补 |
| Bootstrap 跨实例 TOCTOU | 单实例 Mutex 已覆盖；多实例需 DB unique constraint | backlog |
| 页面刷新需重新登录 | 移除 sessionStorage key 后的 UX tradeoff，符合高安全标准 | 已知设计决定 |

### 阻塞项

**无阻塞项。**

---

## 5. 上线结论

**允许上线。**

前提条件：
1. webui CI (`npm run type-check && npm run build`) 在合并前通过
2. 部署后验证 Bootstrap 状态接口 GET /admin/v1/bootstrap/status 返回 200

观察重点（上线后 24h）：
- Admin Console 登录成功率
- Bootstrap 页访问日志确认一次性锁定生效
- SSE 流式聊天 token 接收完整性

确认记录：  
qa-engineer — 2026-05-08 — 已创建 test-plan.md / launch-acceptance.md，全部 CRITICAL/HIGH 修复已验证。
