import { Typography, Card, Statistic, Row, Col, Spin } from 'antd'
import { useUsage } from '../hooks/useUsage'

const { Title } = Typography

export default function Usage() {
  const { data, isLoading } = useUsage()

  if (isLoading) return <Spin style={{ display: 'block', padding: 48 }} />

  return (
    <div style={{ padding: 24 }}>
      <Title level={4}>Usage</Title>
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={6}>
          <Card><Statistic title="Input Tokens" value={data?.input_tokens ?? 0} /></Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card><Statistic title="Output Tokens" value={data?.output_tokens ?? 0} /></Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card><Statistic title="Total Tokens" value={data?.total_tokens ?? 0} /></Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card><Statistic title="Estimated Cost" value={data?.estimated_cost_usd ?? 0} prefix="$" precision={4} /></Card>
        </Col>
      </Row>
    </div>
  )
}
