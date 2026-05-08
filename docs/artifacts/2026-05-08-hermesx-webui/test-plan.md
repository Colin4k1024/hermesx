# Test Plan — hermesx-webui

**任务**: hermesx-webui  
**日期**: 2026-05-08  
**角色**: qa-engineer  
**状态**: draft → review  
**关联交付**: delivery-plan.md · arch-design.md

---

## 1. 测试范围

### In Scope

| 类别 | 覆盖内容 |
|------|---------|
| Admin Console | 租户 CRUD、API Key 管理、轮换/撤销、审计日志过滤、定价规则 upsert、沙箱策略编辑 |
| User Portal | SSE 流式聊天、多会话管理、记忆列表/删除、技能展示、用量统计 |
| Bootstrap 页 | 状态检查 → 创建初始管理员 Key 全链路 |
| 登录页 | 用户登录、管理员登录、连接失败处理 |
| 安全边界 | ACP token 对比（常量时间）、sessionStorage 无明文 key、isAdmin role 校验 |
| CORS | Vary: Origin 响应头、credentials + specific origin 组合 |
| CI | webui.yml 最小权限 build 通过 |

### Out of Scope

- OIDC SSO 集成（v1.3.0 独立测试）
- 性能压测（另立专项）
- 移动端 PWA 安装测试

---

## 2. 测试矩阵

### 2.1 Admin Console — 功能

| 场景 | 类型 | 前置条件 | 预期结果 |
|------|------|---------|---------|
| 创建租户 | 功能 | 已登录 admin | 返回 201，列表出现新租户 |
| 删除租户 | 功能 | 租户存在 | Popconfirm 确认后行消失 |
| 为租户创建 API Key | 功能 | 租户存在 | 一次性密钥 Alert 显示完整 key，后续列表只显示 prefix |
| 轮换 API Key | 功能 | Key 存在 | 旧 key 失效，新 key Alert 显示一次 |
| 撤销 API Key | 功能 | Key 未撤销 | revoked_at 有值，行标记撤销 |
| 查看审计日志 | 功能 | 有审计记录 | 50 条/页，点行展开 detail modal |
| 更新定价规则 | 功能 | - | PUT 成功，列表即时刷新 |
| 沙箱策略 404 → 空态 | 边界 | 无策略 | 显示空态提示而非报错 |
| 未登录访问 /admin 路由 | 安全 | - | 重定向到 /login |
| 普通 user key 访问 admin | 安全 | user key | 403 → 自动跳转 /login |

### 2.2 User Portal — 功能

| 场景 | 类型 | 前置条件 | 预期结果 |
|------|------|---------|---------|
| SSE 流式聊天 | 功能 | 已连接用户 key | token 逐字追加，finish 后 Stop 按钮隐藏 |
| 中止 SSE 流 | 功能 | 流式中 | 点 Stop 后 loading=false，内容截断 |
| 多会话切换 | 功能 | 已有多会话 | 侧栏选择后聊天区更新 |
| 删除记忆 | 功能 | 记忆存在 | Popconfirm 确认后项消失 |
| 技能详情 modal | 功能 | 技能列表非空 | 点击 card 弹 modal 显示详情 |
| 用量统计卡片 | 功能 | token 有消耗 | /v1/me + /v1/usage 并行返回，卡片显示正确数据 |
| 页面刷新后重新登录 | 边界 | 已登录 | key 不在 sessionStorage，显示登录页 |

### 2.3 Bootstrap 页

| 场景 | 类型 | 预期结果 |
|------|------|---------|
| GET /admin/v1/bootstrap/status → true | 功能 | 显示 setup form |
| GET /admin/v1/bootstrap/status → false | 功能 | 跳转 /login |
| POST bootstrap 成功 | 功能 | 返回完整 key，保存并跳转 /login |
| 并发两次 POST bootstrap | 安全 | 第二次返回 403 "already completed" |

### 2.4 安全

| 检查项 | 验证方式 | 预期 |
|--------|---------|------|
| ACP token 使用 subtle.ConstantTimeCompare | 代码审查 | 无字符串 `!=` 比较 |
| sessionStorage 无明文 API key | 浏览器 DevTools | hx_user_key / hx_admin_key 不存在 |
| isAdmin 依赖 roles 数组 | 代码审查 | adminRoles.value.includes('admin') |
| Authorization header 为空时不发送 Bearer | 代码审查 | if (key) 守卫 |
| CORS Vary: Origin | curl/DevTools | 响应头含 Vary: Origin |
| webui.yml permissions 最小权限 | CI 检查 | contents: read; actions: write |

---

## 3. 高风险路径

1. **SSE 流中途网络断开** — onError 处理 + abort 清理
2. **Bootstrap TOCTOU** — sync.Mutex 序列化已在代码中落地，但跨实例场景仍依赖 DB 唯一约束
3. **sessionStorage 残留值** — 老会话升级后 hx_user_key 可能仍存在于 storage，disconnect 函数需清除

---

## 4. 回归关注点

- `server_test.go` spaFallback call sites 已修复为双参数签名
- `test_web_isolation.sh` Phase 1 不再检查 isolation-test.html

---

## 5. 放行建议

| 条件 | 状态 |
|------|------|
| CRITICAL 安全修复（subtle.ConstantTimeCompare, Mutex, sessionStorage, isAdmin roles） | ✅ 已修复 |
| HIGH：空 Authorization 守卫 | ✅ 已修复 |
| HIGH：webui.yml 最小权限 | ✅ 已修复 |
| HIGH：CORS Vary: Origin | ✅ 已修复 |
| Go build 编译通过 | ✅ 已验证 |
| MEDIUM：Bootstrap IP 限速（无 middleware 覆盖） | ⚠️ 遗留风险，建议下一版本处理 |
| MEDIUM：useSse 401/403 auto-logout | ⚠️ 遗留风险，影响 SSE 异常态 |

**放行建议：允许合并，但须在 v2.2.0 前关闭 MEDIUM 遗留项。**

---

## 已接受风险

- **Bootstrap 跨实例 TOCTOU**：单实例部署场景 Mutex 已足够；多实例需 DB unique constraint，列入 backlog。
- **sessionStorage 清除旧 key**：disconnect 函数调用时清除；浏览器已有旧会话的用户登出后会清理。
- **Bootstrap 端点无速率限制**：ACP token 为必要前提，暴力破解需 32 字节 hex 随机值；中期补 IP 限流。
