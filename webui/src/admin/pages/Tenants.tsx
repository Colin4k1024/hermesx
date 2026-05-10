import { useState } from 'react'
import { Table, Button, Space, Typography, Modal, Input, message, Popconfirm, Tag } from 'antd'
import { Plus } from 'lucide-react'
import { useTenants, useCreateTenant, useDeleteTenant } from '../hooks/useTenants'
import type { TenantItem } from '@shared/types'
import dayjs from 'dayjs'

const { Title } = Typography

export default function Tenants() {
  const { data, isLoading } = useTenants()
  const createTenant = useCreateTenant()
  const deleteTenant = useDeleteTenant()
  const [modalOpen, setModalOpen] = useState(false)
  const [name, setName] = useState('')

  const handleCreate = async () => {
    if (!name.trim()) return
    try {
      await createTenant.mutateAsync({ name: name.trim() })
      message.success('Tenant created')
      setModalOpen(false)
      setName('')
    } catch (e) {
      message.error(e instanceof Error ? e.message : 'Failed to create tenant')
    }
  }

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      render: (id: string) => <code style={{ fontSize: 12 }}>{id}</code>,
    },
    { title: 'Name', dataIndex: 'name', key: 'name' },
    {
      title: 'Created',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (v: string) => dayjs(v).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: 'Status',
      key: 'status',
      render: () => <Tag color="green">Active</Tag>,
    },
    {
      title: '',
      key: 'actions',
      width: 80,
      render: (_: unknown, record: TenantItem) => (
        <Popconfirm title="Delete this tenant?" onConfirm={() => deleteTenant.mutate(record.id)}>
          <Button type="text" danger size="small">Delete</Button>
        </Popconfirm>
      ),
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={4} style={{ margin: 0 }}>Tenants</Title>
        <Button type="primary" icon={<Plus size={16} />} onClick={() => setModalOpen(true)}>
          New Tenant
        </Button>
      </Space>
      <Table
        dataSource={data?.tenants ?? []}
        columns={columns}
        rowKey="id"
        loading={isLoading}
        pagination={false}
        size="middle"
      />
      <Modal
        title="Create Tenant"
        open={modalOpen}
        onOk={handleCreate}
        onCancel={() => { setModalOpen(false); setName('') }}
        confirmLoading={createTenant.isPending}
      >
        <Input
          placeholder="Tenant name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          onPressEnter={handleCreate}
          style={{ marginTop: 16 }}
        />
      </Modal>
    </div>
  )
}
