import { Typography, Card, Space, Input, Popconfirm, Button, Empty, Spin, Row, Col } from 'antd'
import { Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useMemories, useDeleteMemory } from '../hooks/useMemories'

const { Title, Text, Paragraph } = Typography
const { Search } = Input

export default function Memories() {
  const { data, isLoading } = useMemories()
  const deleteMemory = useDeleteMemory()
  const [search, setSearch] = useState('')

  const filtered = (data?.memories ?? []).filter(
    (m) => m.key.toLowerCase().includes(search.toLowerCase()) || m.content.toLowerCase().includes(search.toLowerCase()),
  )

  if (isLoading) return <Spin style={{ display: 'block', padding: 48 }} />

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={4} style={{ margin: 0 }}>Memories</Title>
        <Search placeholder="Search memories" allowClear onSearch={setSearch} onChange={(e) => setSearch(e.target.value)} style={{ width: 240 }} />
      </Space>
      {filtered.length === 0 ? (
        <Empty description="No memories found" />
      ) : (
        <Row gutter={[16, 16]}>
          {filtered.map((m) => (
            <Col key={m.key} xs={24} sm={12} lg={8}>
              <Card
                size="small"
                title={<Text strong style={{ fontSize: 13 }}>{m.key}</Text>}
                extra={
                  <Popconfirm title="Delete this memory?" onConfirm={() => deleteMemory.mutate(m.key)}>
                    <Button type="text" size="small" icon={<Trash2 size={14} />} danger />
                  </Popconfirm>
                }
              >
                <Paragraph ellipsis={{ rows: 3 }} style={{ margin: 0, fontSize: 13 }}>{m.content}</Paragraph>
              </Card>
            </Col>
          ))}
        </Row>
      )}
    </div>
  )
}
