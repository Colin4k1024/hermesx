import { useState, useEffect } from 'react'
import { Table, Button, Space, Typography, Modal, Input, Select, message, Tag, Popconfirm, Alert } from 'antd'
import { Plus } from 'lucide-react'
import { useApiKeys, useCreateApiKey, useRevokeApiKey, useRotateApiKey } from '../hooks/useApiKeys'
import { useTenants } from '../hooks/useTenants'
import type { ApiKeyItem } from '@shared/types'
import dayjs from 'dayjs'

const { Title, Text } = Typography

export default function ApiKeys() {
  const { data: tenantsData } = useTenants()
  const [selectedTenant, setSelectedTenant] = useState('')

  useEffect(() => {
    const first = tenantsData?.tenants?.[0]
    if (!selectedTenant && first) {
      setSelectedTenant(first.id)
    }
  }, [tenantsData, selectedTenant])
  const { data, isLoading } = useApiKeys(selectedTenant)
  const createKey = useCreateApiKey(selectedTenant)
  const revokeKey = useRevokeApiKey(selectedTenant)
  const rotateKey = useRotateApiKey(selectedTenant)
  const [modalOpen, setModalOpen] = useState(false)
  const [keyName, setKeyName] = useState('')
  const [createdKey, setCreatedKey] = useState('')

  const handleCreate = async () => {
    if (!keyName.trim()) return
    try {
      const result = await createKey.mutateAsync({ name: keyName.trim(), roles: ['chat'], scopes: ['chat', 'read'] })
      setCreatedKey(result.key)
      setKeyName('')
      message.success('API key created')
    } catch (e) {
      message.error(e instanceof Error ? e.message : 'Failed')
    }
  }

  const handleRotate = async (keyId: string) => {
    try {
      const result = await rotateKey.mutateAsync(keyId)
      setCreatedKey(result.key)
      message.success('Key rotated')
    } catch (e) {
      message.error(e instanceof Error ? e.message : 'Failed')
    }
  }

  const columns = [
    { title: 'Name', dataIndex: 'name', key: 'name' },
    {
      title: 'Prefix',
      dataIndex: 'prefix',
      key: 'prefix',
      render: (v: string) => <code>{v}...</code>,
    },
    {
      title: 'Roles',
      dataIndex: 'roles',
      key: 'roles',
      render: (roles: string[]) => roles.map((r) => <Tag key={r}>{r}</Tag>),
    },
    {
      title: 'Created',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (v: string) => dayjs(v).format('YYYY-MM-DD'),
    },
    {
      title: 'Status',
      key: 'status',
      render: (_: unknown, r: ApiKeyItem) =>
        r.revoked_at ? <Tag color="red">Revoked</Tag> : <Tag color="green">Active</Tag>,
    },
    {
      title: '',
      key: 'actions',
      width: 160,
      render: (_: unknown, record: ApiKeyItem) =>
        !record.revoked_at && (
          <Space size="small">
            <Button size="small" onClick={() => handleRotate(record.id)}>Rotate</Button>
            <Popconfirm title="Revoke?" onConfirm={() => revokeKey.mutate(record.id)}>
              <Button size="small" danger>Revoke</Button>
            </Popconfirm>
          </Space>
        ),
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Title level={4}>API Keys</Title>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
        <Select
          placeholder="Select tenant"
          style={{ width: 280 }}
          value={selectedTenant || undefined}
          onChange={setSelectedTenant}
          options={tenantsData?.tenants.map((t) => ({ label: t.name, value: t.id })) ?? []}
        />
        <Button type="primary" icon={<Plus size={16} />} disabled={!selectedTenant} onClick={() => setModalOpen(true)}>
          Create Key
        </Button>
      </Space>
      {selectedTenant ? (
        <Table dataSource={data?.api_keys ?? []} columns={columns} rowKey="id" loading={isLoading} pagination={false} size="middle" />
      ) : (
        <Text type="secondary">Select a tenant to manage its API keys</Text>
      )}
      <Modal
        title="Create API Key"
        open={modalOpen}
        onOk={handleCreate}
        onCancel={() => { setModalOpen(false); setKeyName(''); setCreatedKey('') }}
        confirmLoading={createKey.isPending}
        footer={createdKey ? [<Button key="close" type="primary" onClick={() => { setModalOpen(false); setCreatedKey('') }}>Done</Button>] : undefined}
      >
        {createdKey ? (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Alert type="warning" message="Copy this key now. It will not be shown again." showIcon />
            <Input.TextArea value={createdKey} readOnly rows={2} style={{ fontFamily: 'monospace', marginTop: 8 }} />
          </Space>
        ) : (
          <Input placeholder="Key name" value={keyName} onChange={(e) => setKeyName(e.target.value)} onPressEnter={handleCreate} style={{ marginTop: 16 }} />
        )}
      </Modal>
    </div>
  )
}
