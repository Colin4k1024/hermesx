import { useState } from 'react'
import { Tabs, Button, Spin, Alert, Empty, Tooltip, List, Popconfirm, message } from 'antd'
import {
  Package,
  FolderOpen,
  X,
  FileText,
  FileSpreadsheet,
  Presentation,
  FileImage,
  File,
  Download,
  Eye,
  Trash2,
} from 'lucide-react'
import { useWorkspaceStore, selectActiveSession } from '@shared/stores/workspaceStore'
import { useArtifacts, type ArtifactItem } from '@shared/hooks/useArtifacts'
import { useFiles, useDeleteFile, downloadFile, type FileEntry } from '@shared/hooks/useFiles'
import { FilePreview } from './FilePreview'
import { FileUpload } from './FileUpload'

/* ------------------------------------------------------------------ */
/*  ResultsPanel                                                       */
/* ------------------------------------------------------------------ */

export function ResultsPanel() {
  const activeSession = useWorkspaceStore(selectActiveSession)
  const activeTab = useWorkspaceStore((s) => s.resultsPanelActiveTab)
  const setTab = useWorkspaceStore((s) => s.setResultsPanelTab)
  const togglePanel = useWorkspaceStore((s) => s.toggleResultsPanel)

  return (
    <aside
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        background: 'var(--ant-color-bg-container)',
      }}
    >
      {/* Tab Bar */}
      <div
        style={{
          height: 44,
          padding: '0 12px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          borderBottom: '1px solid var(--ant-color-border-secondary)',
          flexShrink: 0,
        }}
      >
        <Tabs
          activeKey={activeTab}
          onChange={(key) => setTab(key as 'artifacts' | 'files')}
          size="small"
          type="line"
          items={[
            { key: 'artifacts', label: '产物' },
            { key: 'files', label: '文件' },
          ]}
          style={{ marginBottom: 0 }}
        />
        <Button
          type="text"
          size="small"
          icon={<X size={14} />}
          onClick={togglePanel}
          aria-label="关闭结果面板"
        />
      </div>

      {/* Content */}
      <div style={{ flex: 1, overflow: 'auto', padding: 12 }}>
        {activeTab === 'artifacts' && <ArtifactsTab sessionId={activeSession?.id ?? null} />}
        {activeTab === 'files' && <FilesTab />}
      </div>
    </aside>
  )
}

/* ------------------------------------------------------------------ */
/*  Artifacts tab                                                      */
/* ------------------------------------------------------------------ */

function ArtifactsTab({ sessionId }: { sessionId: string | null }) {
  const { data: artifacts, isLoading, isError, error, refetch } = useArtifacts(sessionId)
  const [previewTarget, setPreviewTarget] = useState<ArtifactItem | null>(null)

  if (!sessionId) {
    return <ArtifactsEmpty message="选择或创建一个任务开始" />
  }

  if (isLoading) {
    return <LoadingSkeleton />
  }

  if (isError) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        <Alert
          type="error"
          message="加载产物失败"
          description={error instanceof Error ? error.message : '未知错误'}
          showIcon
          action={
            <Button size="small" onClick={() => refetch()}>
              重试
            </Button>
          }
        />
      </div>
    )
  }

  if (!artifacts || artifacts.length === 0) {
    return <ArtifactsEmpty message="任务完成后，产物将在此展示" />
  }

  return (
    <>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {artifacts.map((a) => (
          <ArtifactCard
            key={a.url}
            artifact={a}
            onPreview={() => setPreviewTarget(a)}
          />
        ))}
      </div>
      <FilePreview
        artifact={previewTarget}
        open={!!previewTarget}
        onClose={() => setPreviewTarget(null)}
      />
    </>
  )
}

/* ------------------------------------------------------------------ */
/*  Artifact card                                                      */
/* ------------------------------------------------------------------ */

function ArtifactCard({ artifact, onPreview }: { artifact: ArtifactItem; onPreview: () => void }) {
  const icon = getFileIcon(artifact.name)

  return (
    <div
      style={{
        background: 'var(--ant-color-bg-elevated)',
        border: '1px solid var(--ant-color-border-secondary)',
        borderRadius: 8,
        padding: 12,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 10 }}>
        <div style={{ flexShrink: 0, marginTop: 2 }}>{icon}</div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <Tooltip title={artifact.name}>
            <div
              style={{
                fontSize: 13,
                fontWeight: 500,
                color: 'var(--ant-color-text)',
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              }}
            >
              {artifact.name}
            </div>
          </Tooltip>
          <div style={{ fontSize: 11, color: 'var(--ant-color-text-tertiary)', marginTop: 4 }}>
            {formatSize(artifact.size_bytes)} &middot; {formatRelativeTime(artifact.created_at)}
          </div>
        </div>
      </div>
      <div style={{ display: 'flex', gap: 8, marginTop: 10 }}>
        <Tooltip title="预览">
          <Button size="middle" icon={<Eye size={14} />} onClick={onPreview}>
            预览
          </Button>
        </Tooltip>
        <Tooltip title="下载">
          <Button
            size="middle"
            icon={<Download size={14} />}
            href={artifact.url}
            target="_blank"
            rel="noopener noreferrer"
            aria-label="下载产物"
          >
            下载
          </Button>
        </Tooltip>
      </div>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Files tab                                                          */
/* ------------------------------------------------------------------ */

function FilesTab() {
  const { data: files, isLoading, isError, error, refetch } = useFiles()
  const deleteFile = useDeleteFile()
  const [downloadingId, setDownloadingId] = useState<string | null>(null)

  const handleDownload = async (id: string) => {
    setDownloadingId(id)
    try {
      await downloadFile(id)
    } catch (err) {
      message.error(`下载失败: ${err instanceof Error ? err.message : '未知错误'}`)
    } finally {
      setDownloadingId(null)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await deleteFile.mutateAsync(id)
      message.success('文件已删除')
    } catch (err) {
      message.error(`删除失败: ${err instanceof Error ? err.message : '未知错误'}`)
    }
  }

  if (isLoading) {
    return <LoadingSkeleton />
  }

  if (isError) {
    return (
      <Alert
        type="error"
        message="加载文件列表失败"
        description={error instanceof Error ? error.message : '未知错误'}
        showIcon
        action={
          <Button size="small" onClick={() => refetch()}>
            重试
          </Button>
        }
      />
    )
  }

  if (!files || files.length === 0) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12, alignItems: 'center' }}>
        <FilesEmpty />
        <FileUpload compact onUploaded={refetch} />
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 8 }}>
        <FileUpload compact onUploaded={refetch} />
      </div>
      <List
        size="small"
        dataSource={files}
        renderItem={(item: FileEntry) => (
          <List.Item
            style={{ padding: '6px 0', border: 'none' }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, width: '100%' }}>
              <File size={16} style={{ color: 'var(--ant-color-text-tertiary)', flexShrink: 0 }} />
              <Tooltip title={item.path}>
                <span
                  style={{
                    fontSize: 13,
                    color: 'var(--ant-color-text)',
                    whiteSpace: 'nowrap',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    flex: 1,
                  }}
                >
                  {item.path}
                </span>
              </Tooltip>
              <span style={{ fontSize: 11, color: 'var(--ant-color-text-quaternary)', flexShrink: 0 }}>
                {formatSize(item.size_bytes)}
              </span>
              <div style={{ display: 'flex', gap: 4, flexShrink: 0 }}>
                <Tooltip title="下载">
                  <Button
                    type="text"
                    size="middle"
                    icon={<Download size={14} />}
                    loading={downloadingId === item.id}
                    onClick={() => handleDownload(item.id)}
                    aria-label="下载文件"
                    style={{ minWidth: 44, minHeight: 44 }}
                  />
                </Tooltip>
                <Popconfirm
                  title="确定删除此文件？"
                  onConfirm={() => handleDelete(item.id)}
                  okText="删除"
                  cancelText="取消"
                >
                  <Tooltip title="删除">
                    <Button
                      type="text"
                      size="middle"
                      danger
                      icon={<Trash2 size={14} />}
                      aria-label="删除文件"
                      style={{ minWidth: 44, minHeight: 44 }}
                    />
                  </Tooltip>
                </Popconfirm>
              </div>
            </div>
          </List.Item>
        )}
      />
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Empty states                                                       */
/* ------------------------------------------------------------------ */

function ArtifactsEmpty({ message }: { message: string }) {
  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        height: '100%',
        gap: 12,
      }}
    >
      <Package size={48} style={{ color: 'var(--ant-color-text-tertiary)' }} />
      <span style={{ fontSize: 13, color: 'var(--ant-color-text-tertiary)', textAlign: 'center' }}>
        {message}
      </span>
    </div>
  )
}

function FilesEmpty() {
  return (
    <Empty
      image={<FolderOpen size={48} style={{ color: 'var(--ant-color-text-tertiary)' }} />}
      description={
        <span style={{ fontSize: 13, color: 'var(--ant-color-text-tertiary)' }}>
          暂无工作区文件
        </span>
      }
    />
  )
}

function LoadingSkeleton() {
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        height: '100%',
      }}
    >
      <Spin />
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  File icon mapping                                                  */
/* ------------------------------------------------------------------ */

function getFileIcon(name: string) {
  const ext = name.slice(name.lastIndexOf('.')).toLowerCase()

  switch (ext) {
    case '.docx':
    case '.doc':
      return <FileText size={20} style={{ color: 'var(--ant-color-primary)' }} />
    case '.xlsx':
    case '.xls':
      return <FileSpreadsheet size={20} style={{ color: 'var(--ant-color-success)' }} />
    case '.pptx':
    case '.ppt':
      return <Presentation size={20} style={{ color: 'var(--ant-color-error)' }} />
    case '.pdf':
      return <FileText size={20} style={{ color: 'var(--ant-color-error)' }} />
    case '.png':
    case '.jpg':
    case '.jpeg':
    case '.gif':
    case '.svg':
    case '.webp':
      return <FileImage size={20} style={{ color: 'var(--ant-color-primary)' }} />
    default:
      return <File size={20} style={{ color: 'var(--ant-color-text-tertiary)' }} />
  }
}

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function formatRelativeTime(dateStr: string): string {
  const now = Date.now()
  const then = new Date(dateStr).getTime()
  const diffSec = Math.floor((now - then) / 1000)

  if (diffSec < 60) return '刚刚'
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}分钟前`
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}小时前`
  if (diffSec < 86400 * 7) return `${Math.floor(diffSec / 86400)}天前`

  // Fallback to date string for older files
  const d = new Date(dateStr)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}
