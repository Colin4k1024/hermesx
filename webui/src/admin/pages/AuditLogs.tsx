import { useState } from 'react'
import { Table, Typography, Input, Space, Tag, DatePicker } from 'antd'
import { useAuditLogs } from '../hooks/useAuditLogs'
import type { AuditLogItem } from '@shared/types'
import dayjs from 'dayjs'

const { Title } = Typography
const { Search } = Input

export default function AuditLogs() {
  const [page, setPage] = useState(0)
  const [action, setAction] = useState('')
  const { data, isLoading } = useAuditLogs({ limit: 50, offset: page * 50, action: action || undefined })

  const columns = [
    {
      title: 'Time',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (v: string) => dayjs(v).format('MM-DD HH:mm:ss'),
    },
    {
      title: 'Action',
      dataIndex: 'action',
      key: 'action',
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: 'Tenant',
      dataIndex: 'tenant_id',
      key: 'tenant_id',
      render: (v: string) => <code style={{ fontSize: 11 }}>{v?.slice(0, 8)}</code>,
    },
    { title: 'User', dataIndex: 'user_id', key: 'user_id' },
    {
      title: 'Status',
      dataIndex: 'status_code',
      key: 'status_code',
      width: 80,
      render: (v: number | null) =>
        v ? <Tag color={v < 400 ? 'green' : v < 500 ? 'orange' : 'red'}>{v}</Tag> : '—',
    },
    { title: 'Detail', dataIndex: 'detail', key: 'detail', ellipsis: true },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Title level={4}>Audit Logs</Title>
      <Space style={{ marginBottom: 16 }}>
        <Search
          placeholder="Filter by action"
          allowClear
          onSearch={setAction}
          style={{ width: 200 }}
        />
        <DatePicker.RangePicker size="middle" />
      </Space>
      <Table
        dataSource={data?.logs ?? []}
        columns={columns}
        rowKey={(r: AuditLogItem) => `${r.id}`}
        loading={isLoading}
        size="middle"
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
