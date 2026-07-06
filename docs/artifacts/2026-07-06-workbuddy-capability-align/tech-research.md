# 技术预研结论

| 字段 | 值 |
|------|-----|
| 状态 | accepted |
| 日期 | 2026-07-06 |
| 决策人 | tech-lead |

## 决策 1: 办公产物生成技术路线

**结论：Go excelize (xlsx) + Python sandbox (docx/pptx)**

| 格式 | 实现方式 | 库 | 许可证 |
|------|---------|-----|--------|
| xlsx | Go 原生 in-process | excelize (Apache-2.0) | 兼容 |
| docx | Python via execute_code | python-docx (MIT) | 兼容 |
| pptx | Python via execute_code | python-pptx (MIT) | 兼容 |

理由：
- 项目已有 `execute_code` 沙箱运行 Python，无新增部署成本
- `skills/productivity/powerpoint/` 和 `ocr-and-documents/` 已验证 Python 路径
- excelize (17k stars, Apache-2.0) 是 Go 生态最成熟的 xlsx 库
- Go 原生 docx (unioffice) 为 AGPL，SaaS 不可用；Go pptx 库不存在

## 决策 2: WebUI 重构策略

**结论：新建 `/workspace` 路由（Strategy B）**

架构：
- 左栏：`TaskSidebar` — 任务/会话列表 + 文件夹分组
- 中栏：`DialogArea` — 复用提取的 MessageList + InputBar
- 右栏：`ResultsPanel` — 工具输出、文件预览、步骤状态

理由：
- Chat.tsx 仅 ~150 行，提取共享组件成本极低
- Ant Design 6 原生 `Splitter` 组件支持可调三栏
- 后端已有 workflow-runs / workflow-tasks API
- 新路由支持 A/B 测试和零宕机迁移

关键风险：
- 多任务 SSE 并发流需 per-session 实例化
- 小屏需 panel 折叠策略

## 决策 3: 文件工作区语义

**结论：Hybrid — 用户 workspace + 任务沙箱（Option C）**

MinIO key 布局：
```
{tenant_id}/{user_id}/workspace/{path}              ← 持久用户文件
{tenant_id}/{user_id}/sessions/{session_id}/{path}  ← 任务临时空间(auto-TTL)
```

数据模型新增：
```go
type FileEntry struct {
    ID        string     `json:"id" db:"id"`
    TenantID  string     `json:"tenant_id" db:"tenant_id"`
    UserID    string     `json:"user_id" db:"user_id"`
    SessionID string     `json:"session_id,omitempty" db:"session_id"`
    Path      string     `json:"path" db:"path"`
    ObjectKey string     `json:"object_key" db:"object_key"`
    SizeBytes int64      `json:"size_bytes" db:"size_bytes"`
    MimeType  string     `json:"mime_type" db:"mime_type"`
    CreatedAt time.Time  `json:"created_at" db:"created_at"`
    ExpiresAt *time.Time `json:"expires_at,omitempty" db:"expires_at"`
}
```

理由：
- 兼顾持久性与隔离性
- 对标 WorkBuddy / Claude Code 模式
- MinIO/S3 接口已就绪，配额机制已有
- session 文件 auto-TTL 自动清理
