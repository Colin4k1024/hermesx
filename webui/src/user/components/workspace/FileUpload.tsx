import { useCallback, useRef, useState } from 'react'
import { Button, message, Progress } from 'antd'
import { Upload, Inbox } from 'lucide-react'
import { useUploadFile } from '@shared/hooks/useFiles'

interface FileUploadProps {
  /** Called after a successful upload to trigger list refresh. */
  onUploaded?: () => void
  /** Compact mode for inline placement. */
  compact?: boolean
}

/**
 * FileUpload provides drag-and-drop and click-to-upload for workspace files.
 * Uses POST /v1/files/upload with FormData.
 */
export function FileUpload({ onUploaded, compact }: FileUploadProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [isDragging, setIsDragging] = useState(false)
  const upload = useUploadFile()

  const handleFiles = useCallback(
    async (files: FileList | File[]) => {
      const arr = Array.from(files)
      if (arr.length === 0) return

      let successCount = 0
      for (const file of arr) {
        try {
          await upload.mutateAsync({ file })
          successCount++
        } catch (err) {
          message.error(`上传 ${file.name} 失败: ${err instanceof Error ? err.message : '未知错误'}`)
        }
      }

      if (successCount > 0) {
        message.success(`成功上传 ${successCount} 个文件`)
        onUploaded?.()
      }
    },
    [upload, onUploaded],
  )

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(true)
  }, [])

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(false)
  }, [])

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault()
      e.stopPropagation()
      setIsDragging(false)
      if (e.dataTransfer.files.length > 0) {
        handleFiles(e.dataTransfer.files)
      }
    },
    [handleFiles],
  )

  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      if (e.target.files && e.target.files.length > 0) {
        handleFiles(e.target.files)
        e.target.value = ''
      }
    },
    [handleFiles],
  )

  if (compact) {
    return (
      <>
        <Button
          size="small"
          icon={<Upload size={14} />}
          onClick={() => inputRef.current?.click()}
          loading={upload.isPending}
        >
          上传文件
        </Button>
        <input
          ref={inputRef}
          type="file"
          multiple
          style={{ display: 'none' }}
          onChange={handleInputChange}
        />
      </>
    )
  }

  return (
    <div
      role="button"
      tabIndex={0}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
      onClick={() => inputRef.current?.click()}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          inputRef.current?.click()
        }
      }}
      style={{
        border: `2px dashed ${isDragging ? 'var(--ant-color-primary)' : 'var(--ant-color-border)'}`,
        borderRadius: 8,
        padding: compact ? 12 : 24,
        textAlign: 'center',
        cursor: 'pointer',
        background: isDragging
          ? 'var(--ant-color-primary-bg)'
          : 'var(--ant-color-bg-elevated)',
        transition: 'all 0.2s',
      }}
    >
      <Inbox
        size={compact ? 24 : 40}
        style={{ color: 'var(--ant-color-text-tertiary)', marginBottom: 8 }}
      />
      <div style={{ fontSize: 13, color: 'var(--ant-color-text-secondary)' }}>
        拖拽文件到此处，或点击选择文件
      </div>
      <div style={{ fontSize: 11, color: 'var(--ant-color-text-quaternary)', marginTop: 4 }}>
        最大 100MB
      </div>
      {upload.isPending && (
        <div style={{ marginTop: 8 }}>
          <Progress percent={100} size="small" status="active" />
        </div>
      )}
      <input
        ref={inputRef}
        type="file"
        multiple
        style={{ display: 'none' }}
        onChange={handleInputChange}
      />
    </div>
  )
}
