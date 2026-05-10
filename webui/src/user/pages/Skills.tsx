import { Typography, Card, Space, Tag, Empty, Spin, Row, Col, Drawer } from 'antd'
import { Zap } from 'lucide-react'
import { useState } from 'react'
import { useSkills } from '../hooks/useSkills'
import type { SkillItem } from '@shared/types'

const { Title, Text, Paragraph } = Typography

export default function Skills() {
  const { data, isLoading } = useSkills()
  const [selected, setSelected] = useState<SkillItem | null>(null)

  if (isLoading) return <Spin style={{ display: 'block', padding: 48 }} />

  return (
    <div style={{ padding: 24 }}>
      <Title level={4}>Skills</Title>
      {(data?.skills ?? []).length === 0 ? (
        <Empty description="No skills available" />
      ) : (
        <Row gutter={[16, 16]}>
          {(data?.skills ?? []).map((skill) => (
            <Col key={skill.name} xs={24} sm={12} lg={8}>
              <Card hoverable size="small" onClick={() => setSelected(skill)}>
                <Space>
                  <Zap size={16} style={{ color: '#6366f1' }} />
                  <Text strong>{skill.name}</Text>
                </Space>
                {skill.description && (
                  <Paragraph type="secondary" ellipsis={{ rows: 2 }} style={{ margin: '8px 0 0', fontSize: 13 }}>
                    {skill.description}
                  </Paragraph>
                )}
                <Space size={4} style={{ marginTop: 8 }}>
                  {skill.version && <Tag>{skill.version}</Tag>}
                  {skill.user_modified && <Tag color="blue">Modified</Tag>}
                </Space>
              </Card>
            </Col>
          ))}
        </Row>
      )}
      <Drawer title={selected?.name} open={!!selected} onClose={() => setSelected(null)} width={400}>
        {selected && (
          <Space direction="vertical" style={{ width: '100%' }}>
            {selected.description && <Paragraph>{selected.description}</Paragraph>}
            {selected.version && <Text type="secondary">Version: {selected.version}</Text>}
            {selected.source && <Text type="secondary">Source: {selected.source}</Text>}
          </Space>
        )}
      </Drawer>
    </div>
  )
}
