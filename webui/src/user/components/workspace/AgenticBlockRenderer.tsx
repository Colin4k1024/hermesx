import { Card, Typography, Tag, Collapse } from 'antd'
import {
  ToolOutlined,
  BulbOutlined,
  FileTextOutlined,
  CodeOutlined,
  MessageOutlined,
} from '@ant-design/icons'

const { Text, Paragraph } = Typography

interface AgenticBlock {
  type: string
  data?: Record<string, unknown>
}

interface AgenticBlockRendererProps {
  blocks: AgenticBlock[]
}

const blockTypeConfig: Record<
  string,
  { icon: React.ReactNode; color: string; label: string }
> = {
  reasoning: {
    icon: <BulbOutlined />,
    color: 'purple',
    label: '思考过程',
  },
  tool_call: {
    icon: <ToolOutlined />,
    color: 'blue',
    label: '工具调用',
  },
  tool_result: {
    icon: <ToolOutlined />,
    color: 'green',
    label: '工具结果',
  },
  assistant_text: {
    icon: <MessageOutlined />,
    color: 'default',
    label: '助手回复',
  },
  user_input: {
    icon: <MessageOutlined />,
    color: 'cyan',
    label: '用户输入',
  },
  code: {
    icon: <CodeOutlined />,
    color: 'orange',
    label: '代码',
  },
  document: {
    icon: <FileTextOutlined />,
    color: 'geekblue',
    label: '文档',
  },
}

function renderBlockContent(block: AgenticBlock): React.ReactNode {
  if (!block.data) return null

  switch (block.type) {
    case 'reasoning':
      return (
        <Paragraph
          style={{ margin: 0, fontSize: 13, color: 'var(--ant-color-text-secondary)' }}
          ellipsis={{ rows: 3, expandable: true, symbol: '展开' }}
        >
          {String(block.data.text || '')}
        </Paragraph>
      )

    case 'tool_call':
      return (
        <div style={{ fontSize: 13 }}>
          <Text strong>{String(block.data.name || 'unknown')}</Text>
          {block.data.arguments != null && (
            <Paragraph
              code
              style={{ margin: '4px 0 0', fontSize: 12 }}
              ellipsis={{ rows: 2, expandable: true, symbol: '展开' }}
            >
              {typeof block.data.arguments === 'string'
                ? block.data.arguments
                : JSON.stringify(block.data.arguments, null, 2)}
            </Paragraph>
          )}
        </div>
      )

    case 'tool_result':
      return (
        <div style={{ fontSize: 13 }}>
          <Text type="secondary">工具: {String(block.data.name || 'unknown')}</Text>
          {block.data.content != null && (
            <Paragraph
              style={{ margin: '4px 0 0' }}
              ellipsis={{ rows: 3, expandable: true, symbol: '展开' }}
            >
              {String(block.data.content)}
            </Paragraph>
          )}
        </div>
      )

    case 'assistant_text':
    case 'user_input':
      return (
        <Paragraph
          style={{ margin: 0, fontSize: 13 }}
          ellipsis={{ rows: 3, expandable: true, symbol: '展开' }}
        >
          {String(block.data.text || '')}
        </Paragraph>
      )

    default:
      return (
        <Paragraph
          code
          style={{ margin: 0, fontSize: 12 }}
          ellipsis={{ rows: 2, expandable: true, symbol: '展开' }}
        >
          {JSON.stringify(block.data, null, 2)}
        </Paragraph>
      )
  }
}

export function AgenticBlockRenderer({ blocks }: AgenticBlockRendererProps) {
  if (blocks.length === 0) return null

  // Group consecutive blocks of the same type
  const groupedBlocks: Array<{ type: string; blocks: AgenticBlock[] }> = []
  let currentGroup: { type: string; blocks: AgenticBlock[] } | null = null

  for (const block of blocks) {
    if (currentGroup && currentGroup.type === block.type) {
      currentGroup.blocks.push(block)
    } else {
      if (currentGroup) {
        groupedBlocks.push(currentGroup)
      }
      currentGroup = { type: block.type, blocks: [block] }
    }
  }
  if (currentGroup) {
    groupedBlocks.push(currentGroup)
  }

  return (
    <Collapse
      size="small"
      style={{ marginBottom: 8 }}
      items={[
        {
          key: 'agentic-blocks',
          label: (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13 }}>
              <span style={{ fontWeight: 500 }}>Agent 执行详情</span>
              <Tag color="processing">{blocks.length} 个步骤</Tag>
            </div>
          ),
          children: (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {groupedBlocks.map((group, groupIdx) => {
                const config = blockTypeConfig[group.type] || {
                  icon: <ToolOutlined />,
                  color: 'default',
                  label: group.type,
                }
                return (
                  <div key={groupIdx}>
                    {group.blocks.map((block, blockIdx) => (
                      <Card
                        key={`${groupIdx}-${blockIdx}`}
                        size="small"
                        style={{ marginBottom: 4 }}
                        styles={{ body: { padding: '8px 12px' } }}
                      >
                        <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                          <Tag
                            icon={config.icon}
                            color={config.color}
                            style={{ margin: 0, flexShrink: 0 }}
                          >
                            {config.label}
                          </Tag>
                          <div style={{ flex: 1, minWidth: 0 }}>
                            {renderBlockContent(block)}
                          </div>
                        </div>
                      </Card>
                    ))}
                  </div>
                )
              })}
            </div>
          ),
        },
      ]}
    />
  )
}
