import { Modal, Button, Typography, Alert } from 'antd'
import { DownloadOutlined, FileExcelOutlined, FileImageOutlined } from '@ant-design/icons'
import type { ArtifactItem } from '@shared/hooks/useArtifacts'

const { Text, Title } = Typography

interface FilePreviewProps {
  artifact: ArtifactItem | null
  open: boolean
  onClose: () => void
}

function isImage(name: string): boolean {
  return /\.(png|jpe?g|gif|svg|webp|bmp|ico)$/i.test(name)
}

function isXlsx(name: string): boolean {
  return /\.xlsx?$/i.test(name)
}

function getExtension(name: string): string {
  const idx = name.lastIndexOf('.')
  return idx >= 0 ? name.slice(idx + 1).toLowerCase() : ''
}

/* ------------------------------------------------------------------ */
/*  Main preview modal                                                */
/* ------------------------------------------------------------------ */

export function FilePreview({ artifact, open, onClose }: FilePreviewProps) {
  if (!artifact) return null

  const ext = getExtension(artifact.name)
  const image = isImage(artifact.name)
  const xlsx = isXlsx(artifact.name)
  const docTypes = ['docx', 'doc', 'pptx', 'ppt', 'pdf']
  const isDoc = docTypes.includes(ext)

  return (
    <Modal
      open={open}
      title={artifact.name}
      onCancel={onClose}
      width={image ? 800 : 700}
      footer={[
        <Button
          key="download"
          type="primary"
          icon={<DownloadOutlined />}
          href={artifact.url}
          target="_blank"
          rel="noopener noreferrer"
        >
          下载文件
        </Button>,
        <Button key="close" onClick={onClose}>
          关闭
        </Button>,
      ]}
    >
      {image && (
        <div style={{ textAlign: 'center' }}>
          <img
            src={artifact.url}
            alt={artifact.name}
            style={{ maxWidth: '100%', maxHeight: 500, borderRadius: 8 }}
          />
        </div>
      )}

      {xlsx && (
        <Alert
          type="info"
          message="Excel 文件预览"
          description="暂不支持在线预览 Excel 文件，请下载后使用应用程序打开。"
          showIcon
          icon={<FileExcelOutlined />}
        />
      )}

      {isDoc && !xlsx && (
        <div style={{ textAlign: 'center', padding: '32px 0' }}>
          <FileExcelOutlined style={{ fontSize: 48, color: 'var(--ant-color-text-tertiary)', marginBottom: 16 }} />
          <Title level={5} style={{ color: 'var(--ant-color-text-secondary)' }}>
            暂不支持在线预览 {ext.toUpperCase()} 文件
          </Title>
          <Text type="secondary">请下载文件后使用对应应用程序打开</Text>
        </div>
      )}

      {!image && !xlsx && !isDoc && (
        <div style={{ textAlign: 'center', padding: '32px 0' }}>
          <FileImageOutlined style={{ fontSize: 48, color: 'var(--ant-color-text-tertiary)', marginBottom: 16 }} />
          <Title level={5} style={{ color: 'var(--ant-color-text-secondary)' }}>
            暂不支持预览此类型文件
          </Title>
          <Text type="secondary">请下载文件后查看</Text>
        </div>
      )}
    </Modal>
  )
}
