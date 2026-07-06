import { Button, Input, Space } from 'antd'
import { Send, Square } from 'lucide-react'

const { TextArea } = Input

interface Props {
  value: string
  onChange: (value: string) => void
  onSend: () => void
  loading?: boolean
  onAbort?: () => void
  placeholder?: string
  disabled?: boolean
}

export function InputBar({
  value,
  onChange,
  onSend,
  loading,
  onAbort,
  placeholder = '描述你的任务...',
  disabled,
}: Props) {
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      onSend()
    }
  }

  return (
    <div
      style={{
        padding: '12px 16px',
        borderTop: '1px solid var(--ant-color-border-secondary)',
      }}
    >
      <Space.Compact style={{ width: '100%' }}>
        <TextArea
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          autoSize={{ minRows: 1, maxRows: 6 }}
          disabled={disabled}
          style={{ borderRadius: '8px 0 0 8px' }}
        />
        {loading ? (
          <Button
            icon={<Square size={16} />}
            onClick={onAbort}
            danger
            style={{ borderRadius: '0 8px 8px 0' }}
          />
        ) : (
          <Button
            type="primary"
            icon={<Send size={16} />}
            onClick={onSend}
            disabled={disabled || !value.trim()}
            style={{ borderRadius: '0 8px 8px 0' }}
          />
        )}
      </Space.Compact>
    </div>
  )
}
