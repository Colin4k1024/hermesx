import { useEffect, useState } from 'react'
import { Card, Col, Row, Statistic, Typography, Spin } from 'antd'
import { Building2, KeyRound, Activity, DollarSign } from 'lucide-react'
import { useTenants } from '../hooks/useTenants'
import { apiClient } from '@shared/api/client'
import type { ApiKeyListResponse, UsageResponse } from '@shared/types'

const { Title } = Typography

export default function Dashboard() {
  const { data, isLoading } = useTenants()
  const [totalKeys, setTotalKeys] = useState<number | null>(null)
  const [usage, setUsage] = useState<UsageResponse | null>(null)

  useEffect(() => {
    if (!data?.tenants?.length) return
    let cancelled = false
    const fetchKeys = async () => {
      let count = 0
      for (const t of data.tenants) {
        try {
          const res = await apiClient.get<ApiKeyListResponse>(`/admin/v1/tenants/${t.id}/api-keys`, { asAdmin: true })
          count += res.api_keys?.length ?? 0
        } catch { /* skip */ }
      }
      if (!cancelled) setTotalKeys(count)
    }
    const fetchUsage = async () => {
      try {
        const res = await apiClient.get<UsageResponse>('/v1/usage', { asAdmin: true })
        if (!cancelled) setUsage(res)
      } catch { /* skip */ }
    }
    fetchKeys()
    fetchUsage()
    return () => { cancelled = true }
  }, [data])

  if (isLoading) return <Spin style={{ display: 'block', padding: 48 }} />

  const tenantCount = data?.total ?? 0

  return (
    <div style={{ padding: 24 }}>
      <Title level={4} style={{ marginBottom: 24 }}>Dashboard</Title>
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="Tenants"
              value={tenantCount}
              prefix={<Building2 size={18} style={{ marginRight: 4 }} />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="API Keys"
              value={totalKeys ?? '—'}
              prefix={<KeyRound size={18} style={{ marginRight: 4 }} />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="Total Tokens"
              value={usage?.total_tokens ?? '—'}
              prefix={<Activity size={18} style={{ marginRight: 4 }} />}
              formatter={(v) => typeof v === 'number' ? v.toLocaleString() : String(v)}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="Est. Cost (USD)"
              value={usage?.estimated_cost_usd ?? '—'}
              prefix={<DollarSign size={18} style={{ marginRight: 4 }} />}
              precision={usage?.estimated_cost_usd != null ? 2 : undefined}
            />
          </Card>
        </Col>
      </Row>
    </div>
  )
}
