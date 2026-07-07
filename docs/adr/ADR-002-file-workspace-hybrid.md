# ADR-002: File Workspace Hybrid Model and MinIO Key Design

## 决策信息

| 字段 | 值 |
|------|-----|
| 编号 | ADR-002-file |
| 标题 | 文件工作区混合模型与 MinIO Key 设计 |
| 状态 | Accepted |
| 日期 | 2026-07-06 |
| Owner | architect |
| 关联需求 | 用户持久化工作区文件 + 每任务沙箱文件隔离 |

---

## 背景与约束

### 当前问题

现有文件工具 (`internal/tools/files.go`) 直接操作宿主文件系统，无租户/会话隔离：

```go
func handleReadFile(ctx context.Context, args map[string]any, tctx *ToolContext) string {
    filePath = absPath(filePath)
    data, err := os.ReadFile(filePath)
    // ...
}
```

问题：
- Agent 的 `read_file` / `write_file` / `patch` / `search_files` 直接读写宿主 OS 路径
- 无 tenant 隔离，跨租户文件可被任意访问
- 无 session 沙箱，一次任务产生的临时文件污染全局
- 无配额管控，单租户可无限写入

### 业务目标

- 用户需要跨 session 的**持久化工作区**（workspace），如上传的知识文件、agent 产出的报告
- 每次 agent 任务需要**沙箱隔离**（session sandbox），临时文件在 session 结束后自动清理
- 用户可以将 session 临时文件"晋升"到持久化工作区
- 多租户严格隔离，PG RLS + MinIO key prefix 双重保障

### 约束条件

- MinIO 已集成 (`internal/objstore/`)，接口为 `ObjectStore`（Put/Get/Delete/List/Exists）
- 多租户隔离通过 `tenant_id` + PG RLS 实施
- 配额系统已有 `governance.Quota.MaxStorageMB` 字段
- Agent session 已有 `Session.ID`、`Session.TenantID`、`Session.UserID` 完整标识

### 非目标

- 不支持文件夹层级权限（ACL）——v1 只做 owner-level 访问
- 不支持文件版本控制（versioning）
- 不支持跨租户文件共享
- 不支持大文件分片上传（>100MB 文件延后支持）

---

## 备选方案

### 方案 A：纯数据库存储（PostgreSQL BYTEA / LO）

- **适用条件**：文件体积小（<1MB），数量有限
- **优点**：无需额外存储组件，事务一致性天然保证
- **风险**：大文件导致数据库膨胀，备份恢复变慢，MVCC 压力大
- **不选原因**：agent 产出文件可能较大（代码仓快照、PDF 报告），不适合数据库存储

### 方案 B：纯 MinIO 存储（无 PG 元数据）

- **适用条件**：不需要复杂查询、标签和关联
- **优点**：架构极简，MinIO LIST 即目录
- **风险**：无法做文件级元数据查询（按标签、按类型、按大小筛选）；配额检查需遍历 LIST
- **不选原因**：workspace 文件需要元数据索引（标题、MIME、标签、关联 session）供前端展示和搜索

### 方案 C：混合模型——MinIO 存文件体 + PG 存元数据（采用）

- **适用条件**：需要既高效存储大文件，又支持结构化查询
- **优点**：各取所长——MinIO 承担 blob 存储和 TTL 生命周期；PG 承担元数据、权限和配额查询
- **风险**：PG 与 MinIO 之间需保持一致性（orphan cleanup job）
- **采用原因**：session 文件仅存 MinIO（自动 TTL 清理，无 PG 负担）；workspace 文件 MinIO + PG 双写（支持元数据查询、配额精确计算）

---

## 决策结果

### 采用方案

方案 C：混合模型。将文件分为两类：

| 类型 | 存储位置 | 生命周期 | PG 记录 |
|------|----------|----------|---------|
| Session sandbox 文件 | MinIO only | MinIO lifecycle TTL（默认 7 天） | 无 |
| Workspace 持久文件 | MinIO + PG `file_entries` | 永久（用户删除或配额回收） | 有 |

### MinIO Key Layout

```
{bucket}/
  {tenant_id}/
    {user_id}/
      workspace/
        {relative_path}          # 持久化工作区文件
      sessions/
        {session_id}/
          {relative_path}        # 会话沙箱临时文件
```

示例：
```
hermesx-files/
  t_abc123/
    u_user01/
      workspace/reports/weekly-summary.md
      workspace/uploads/data.csv
      sessions/sess_xyz789/scratch/output.json
      sessions/sess_xyz789/generated/chart.png
```

Key 设计原则：
- Prefix 按 `tenant_id/user_id` 分区，便于 MinIO lifecycle rule 和 LIST 操作
- Session sandbox 与 workspace 分开，lifecycle rule 只作用于 `sessions/` 前缀
- Relative path 由 agent 指定，但必须经过安全校验（禁止 `..`、绝对路径、符号链接）

### FileEntry 模型（仅 workspace 文件写入 PG）

```go
// FileEntry represents a promoted/uploaded file in the user workspace.
// Only workspace files are indexed in PG; session sandbox files exist only in MinIO.
type FileEntry struct {
    ID            string     `json:"id" db:"id"`
    TenantID      string     `json:"tenant_id" db:"tenant_id"`
    UserID        string     `json:"user_id" db:"user_id"`
    Path          string     `json:"path" db:"path"`            // relative path within workspace/
    MinIOKey      string     `json:"minio_key" db:"minio_key"`  // full MinIO object key
    SizeBytes     int64      `json:"size_bytes" db:"size_bytes"`
    MIMEType      string     `json:"mime_type" db:"mime_type"`
    SHA256        string     `json:"sha256" db:"sha256"`
    SourceSession string     `json:"source_session,omitempty" db:"source_session"` // session that promoted it
    Tags          []string   `json:"tags,omitempty" db:"tags"`
    CreatedAt     time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
    DeletedAt     *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}
```

PG RLS policy：`tenant_id = current_setting('app.tenant_id')`

### Session 文件生命周期

```
Agent 任务开始
  │
  ├─ file tool 写入 → MinIO key: {tenant}/{user}/sessions/{session_id}/{path}
  │                    无 PG 记录
  │
  ├─ Agent/用户 promote → copy to workspace/ key + INSERT file_entries
  │
  └─ Session 结束
       │
       └─ MinIO lifecycle rule (prefix: */sessions/*, expiry: 7d)
            自动删除过期 session 文件
```

MinIO lifecycle rule 配置：
```json
{
  "Rules": [{
    "ID": "session-file-ttl",
    "Status": "Enabled",
    "Filter": { "Prefix": "" },
    "Expiration": { "Days": 7 },
    "TagFilters": [{ "Key": "hermesx-scope", "Value": "session" }]
  }]
}
```

备选方案：使用 prefix filter `*/sessions/` 或 object tagging（`hermesx-scope=session`）。推荐 tagging 方案，因为 prefix filter 无法跨 tenant 统一配置。

### Promote 操作

```go
// Promote copies a session file to the user's workspace and creates a FileEntry.
func (s *FileService) Promote(ctx context.Context, req PromoteRequest) (*FileEntry, error) {
    // 1. Check quota: current usage + file size <= MaxStorageMB
    // 2. Copy object: sessions/{session_id}/{path} → workspace/{dest_path}
    // 3. INSERT file_entries (with SHA256, size, MIME)
    // 4. Tag source object for immediate TTL (optional: delete after copy)
    // 5. Emit audit log
}

type PromoteRequest struct {
    TenantID    string
    UserID      string
    SessionID   string
    SourcePath  string // relative path in session sandbox
    DestPath    string // relative path in workspace (optional, defaults to same)
}
```

### 配额强制

配额检查点：
1. **写入前**：file tool `write_file` 执行前，计算 `current_usage + new_file_size`
2. **Promote 前**：promote 操作检查 workspace 总量
3. **周期性回收**：cron job 扫描 `SUM(size_bytes) WHERE tenant_id = ?`，对超限租户发出告警

配额来源：`governance.Quota.MaxStorageMB`

```go
func (s *FileService) checkQuota(ctx context.Context, tenantID, userID string, additionalBytes int64) error {
    quota, err := s.gov.GetTenantQuota(ctx, tenantID)
    if err != nil { return err }

    currentBytes, err := s.store.GetUserStorageUsage(ctx, tenantID, userID)
    if err != nil { return err }

    if currentBytes + additionalBytes > int64(quota.MaxStorageMB) * 1024 * 1024 {
        return ErrStorageQuotaExceeded
    }
    return nil
}
```

### 安全：沙箱路径解析

File tools 必须将所有路径解析为 session 虚拟根目录：

```go
func resolveSessionPath(sessionID, userPath string) (string, error) {
    // 1. Clean path
    cleaned := filepath.Clean(userPath)

    // 2. Reject absolute paths
    if filepath.IsAbs(cleaned) {
        return "", ErrAbsolutePathForbidden
    }

    // 3. Reject path traversal
    if strings.Contains(cleaned, "..") {
        return "", ErrPathTraversalForbidden
    }

    // 4. Reject reserved prefixes
    if strings.HasPrefix(cleaned, "workspace/") || strings.HasPrefix(cleaned, "sessions/") {
        return "", ErrReservedPrefix
    }

    // 5. Construct MinIO key (never touches host FS)
    return cleaned, nil
}
```

关键安全原则：
- File tool handler **不再调用** `os.ReadFile` / `os.WriteFile`
- 所有 IO 通过 `ObjectStore` 接口，key 由 `resolveSessionPath` 构造
- Agent 只能操作当前 session 的 sandbox；读取 workspace 需要显式 `read_workspace_file` tool
- Host filesystem 完全不可达

### 迁移计划

| 阶段 | 内容 | 影响 |
|------|------|------|
| Phase 1 | 新增 `FileService` + `FileEntry` 表迁移；file tools 内部切换到 MinIO 后端 | 现有 host-fs 文件不再可访问（breaking change） |
| Phase 2 | 配置 MinIO lifecycle rule；实现 promote API | Session 文件自动回收 |
| Phase 3 | 前端文件管理器 UI；workspace CRUD API | 用户可见 |
| Phase 4 | 配额强制 + orphan cleanup job | 治理闭环 |

Breaking change 处理：
- Phase 1 部署前通知用户：现有 host-fs 路径的文件引用将失效
- 提供一次性迁移脚本：扫描 session 历史中的文件路径引用，将对应文件上传到 workspace

---

## 企业内控补充

| 项目 | 说明 |
|------|------|
| 应用等级 | T3（SaaS 平台核心存储路径） |
| 技术架构等级 | T3（MinIO 高可用 + PG RLS 隔离） |
| 关键组件偏离 | 无——MinIO 为已批准对象存储组件 |
| 资源隔离 | 租户级 key prefix 隔离 + PG RLS；不共享 bucket 读写权限 |
| 资产文档入口 | `internal/objstore/` (MinIO client), `internal/store/types.go` (models) |

---

## 后续动作

| 动作 | 主责 | 完成条件 |
|------|------|----------|
| 新增 `file_entries` 表迁移脚本 | backend-engineer | 迁移可在 dev 环境回放 |
| 实现 `FileService` + path resolver | backend-engineer | 单测覆盖路径遍历攻击向量 |
| 配置 MinIO lifecycle rule (IaC) | devops-engineer | Terraform/Helm 可重复部署 |
| 更新 file tools handler 切换 MinIO 后端 | backend-engineer | 集成测试通过 |
| 新增 `promote` API endpoint | backend-engineer | API contract + 验收测试 |
| 通知 `frontend-engineer` 文件管理器 API 契约 | architect | API contract 文档交付 |
| 配额强制中间件 | backend-engineer | 超限时返回 429 |
| Orphan cleanup cron job | devops-engineer | 7 天无 FileEntry 的 workspace key 被清理 |
