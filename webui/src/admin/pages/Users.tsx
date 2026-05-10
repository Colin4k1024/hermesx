import { Typography, Table, Tag } from 'antd'
import { useTenants } from '../hooks/useTenants'

const { Title, Text } = Typography

export default function Users() {
  const { data, isLoading } = useTenants()

  const columns = [
    { title: 'Tenant', dataIndex: 'name', key: 'name' },
    {
      title: 'Tenant ID',
      dataIndex: 'id',
      key: 'id',
      render: (v: string) => <code style={{ fontSize: 11 }}>{v.slice(0, 8)}...</code>,
    },
    {
      title: 'Status',
      key: 'status',
      render: () => <Tag color="green">Active</Tag>,
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Title level={4}>Users</Title>
      <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
        User activity is derived from tenant session data. Per-user management requires additional backend endpoints.
      </Text>
      <Table
        dataSource={data?.tenants ?? []}
        columns={columns}
        rowKey="id"
        loading={isLoading}
        pagination={false}
        size="middle"
      />
    </div>
  )
}
