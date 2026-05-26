import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Button, Input, Select, Space, Table, Tag, Typography, message } from 'antd'
import { RefreshCcw } from 'lucide-react'
import { apiClient } from '@shared/api/client'

const { Title, Text } = Typography
const { Search } = Input

interface WorkflowRun {
  id: string
  tenant_id: string
  definition_id: string
  version_id: string
  status: string
  started_by?: string
  error?: string
  started_at: string
  completed_at?: string | null
  updated_at: string
}

interface WorkflowRunListResponse {
  workflow_runs: WorkflowRun[]
  total: number
}

const statusColor: Record<string, string> = {
  running: 'blue',
  waiting: 'gold',
  paused: 'orange',
  completed: 'green',
  cancelled: 'default',
  failed: 'red',
}

export default function WorkflowRuns() {
  const [page, setPage] = useState(0)
  const [definitionID, setDefinitionID] = useState('')
  const [status, setStatus] = useState('')

  const qs = new URLSearchParams({ limit: '50', offset: String(page * 50) })
  if (definitionID) qs.set('definition_id', definitionID)
  if (status) qs.set('status', status)

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['admin', 'workflow-runs', page, definitionID, status],
    queryFn: () => apiClient.get<WorkflowRunListResponse>(`/v1/workflow-runs?${qs.toString()}`, { asAdmin: true }),
  })

  const cancelRun = async (id: string) => {
    try {
      await apiClient.post(`/v1/workflow-runs/${encodeURIComponent(id)}/cancel`, {}, { asAdmin: true })
      message.success('Workflow run cancelled')
      refetch()
    } catch (e) {
      message.error(e instanceof Error ? e.message : 'Cancel failed')
    }
  }

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }}>
        <div>
          <Title level={4} style={{ margin: 0 }}>Workflow Runs</Title>
          <Text type="secondary">Runtime status for fixed SOP workflows</Text>
        </div>
        <Button icon={<RefreshCcw size={16} />} onClick={() => refetch()}>Refresh</Button>
      </Space>
      <Space wrap style={{ marginBottom: 16 }}>
        <Search placeholder="Definition ID" allowClear onSearch={(v) => { setDefinitionID(v); setPage(0) }} style={{ width: 260 }} />
        <Select
          allowClear
          placeholder="Status"
          style={{ width: 180 }}
          value={status || undefined}
          onChange={(v) => { setStatus(v ?? ''); setPage(0) }}
          options={['running', 'waiting', 'paused', 'completed', 'cancelled'].map((v) => ({ label: v, value: v }))}
        />
      </Space>
      <Table
        rowKey="id"
        size="middle"
        loading={isLoading}
        dataSource={data?.workflow_runs ?? []}
        columns={[
          { title: 'Run', dataIndex: 'id', key: 'id', render: (v: string) => <code>{v}</code> },
          { title: 'Status', dataIndex: 'status', key: 'status', width: 120, render: (v: string) => <Tag color={statusColor[v] ?? 'default'}>{v}</Tag> },
          { title: 'Definition', dataIndex: 'definition_id', key: 'definition_id', render: (v: string) => <code>{v}</code> },
          { title: 'Version', dataIndex: 'version_id', key: 'version_id', render: (v: string) => <code>{v}</code> },
          { title: 'Started By', dataIndex: 'started_by', key: 'started_by' },
          { title: 'Error', dataIndex: 'error', key: 'error', ellipsis: true },
          { title: 'Updated', dataIndex: 'updated_at', key: 'updated_at', width: 190 },
          {
            title: '',
            key: 'actions',
            width: 110,
            render: (_: unknown, row: WorkflowRun) =>
              row.status === 'running' || row.status === 'waiting' || row.status === 'paused'
                ? <Button size="small" danger onClick={() => cancelRun(row.id)}>Cancel</Button>
                : null,
          },
        ]}
        pagination={{
          total: data?.total ?? 0,
          pageSize: 50,
          current: page + 1,
          onChange: (p) => setPage(p - 1),
          showSizeChanger: false,
        }}
      />
    </div>
  )
}
