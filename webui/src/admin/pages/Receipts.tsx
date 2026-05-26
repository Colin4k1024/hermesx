import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Input, Select, Space, Table, Tag, Typography } from 'antd'
import { apiClient } from '@shared/api/client'

const { Title, Text } = Typography
const { Search } = Input

interface ExecutionReceipt {
  id: string
  tenant_id: string
  session_id: string
  user_id: string
  tool_name: string
  input: string
  output: string
  status: string
  duration_ms: number
  trace_id?: string
  created_at: string
}

interface ReceiptListResponse {
  execution_receipts: ExecutionReceipt[]
  total: number
}

export default function Receipts() {
  const [page, setPage] = useState(0)
  const [toolName, setToolName] = useState('')
  const [status, setStatus] = useState('')

  const qs = new URLSearchParams({ limit: '50', offset: String(page * 50) })
  if (toolName) qs.set('tool_name', toolName)
  if (status) qs.set('status', status)

  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'receipts', page, toolName, status],
    queryFn: () => apiClient.get<ReceiptListResponse>(`/v1/execution-receipts?${qs.toString()}`, { asAdmin: true }),
  })

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }}>
        <div>
          <Title level={4} style={{ margin: 0 }}>Execution Receipts</Title>
          <Text type="secondary">Auditable tool executions across agent and workflow runs</Text>
        </div>
      </Space>
      <Space wrap style={{ marginBottom: 16 }}>
        <Search placeholder="Tool name" allowClear onSearch={(v) => { setToolName(v); setPage(0) }} style={{ width: 220 }} />
        <Select
          allowClear
          placeholder="Status"
          style={{ width: 160 }}
          value={status || undefined}
          onChange={(v) => { setStatus(v ?? ''); setPage(0) }}
          options={['success', 'error', 'timeout'].map((v) => ({ label: v, value: v }))}
        />
      </Space>
      <Table
        rowKey="id"
        size="middle"
        loading={isLoading}
        dataSource={data?.execution_receipts ?? []}
        columns={[
          { title: 'Tool', dataIndex: 'tool_name', key: 'tool_name', render: (v: string) => <code>{v}</code> },
          { title: 'Status', dataIndex: 'status', key: 'status', width: 100, render: (v: string) => <Tag color={v === 'success' ? 'green' : 'red'}>{v}</Tag> },
          { title: 'Duration', dataIndex: 'duration_ms', key: 'duration_ms', width: 110, render: (v: number) => `${v} ms` },
          { title: 'Session', dataIndex: 'session_id', key: 'session_id', render: (v: string) => <code>{v}</code> },
          { title: 'Tenant', dataIndex: 'tenant_id', key: 'tenant_id', render: (v: string) => <code>{v}</code> },
          { title: 'Input', dataIndex: 'input', key: 'input', ellipsis: true },
          { title: 'Output', dataIndex: 'output', key: 'output', ellipsis: true },
          { title: 'Created', dataIndex: 'created_at', key: 'created_at', width: 190 },
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
