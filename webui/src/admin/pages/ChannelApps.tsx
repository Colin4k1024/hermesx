import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Alert, Button, Form, Input, Modal, Popconfirm, Select, Space,
  Switch, Table, Tabs, Tag, Typography, message,
} from 'antd'
import { Plus, RefreshCcw } from 'lucide-react'
import { apiClient, ApiError } from '@shared/api/client'
import { useTenants } from '../hooks/useTenants'

const { Title, Text } = Typography

interface ChannelApp {
  id: string
  tenant_id: string
  platform: string
  app_key: string
  app_secret_ref: string
  oauth_secret_ref: string
  webhook_secret_ref: string
  enabled: boolean
  created_at: string
  updated_at: string
}

interface ChannelBinding {
  id: string
  tenant_id: string
  channel_app_id: string
  platform: string
  provider_user_hash: string
  provider_display_name: string
  user_id: string
  created_at: string
  last_login_at?: string
  revoked_at?: string
}

interface ListResponse<T> {
  items: T[]
  total: number
  limit: number
  offset: number
}

const PLATFORMS = ['feishu', 'wecom', 'weixin']

function unavailable(error: unknown) {
  return error instanceof ApiError && error.status === 503
}

export default function ChannelApps() {
  const qc = useQueryClient()
  const { data: tenantsData } = useTenants()
  const [selectedTenant, setSelectedTenant] = useState('')
  const [appModalOpen, setAppModalOpen] = useState(false)
  const [editingApp, setEditingApp] = useState<ChannelApp | null>(null)
  const [appForm] = Form.useForm()

  const tenantOptions = tenantsData?.tenants.map((t) => ({ label: t.name, value: t.id })) ?? []

  const appsQuery = useQuery({
    queryKey: ['admin', 'channel-apps', selectedTenant],
    queryFn: () =>
      apiClient.get<ListResponse<ChannelApp>>(
        `/admin/v1/channel-apps?tenant_id=${encodeURIComponent(selectedTenant)}&limit=200`,
        { asAdmin: true },
      ),
    enabled: !!selectedTenant,
  })

  const bindingsQuery = useQuery({
    queryKey: ['admin', 'channel-bindings', selectedTenant],
    queryFn: () =>
      apiClient.get<ListResponse<ChannelBinding>>(
        `/admin/v1/channel-bindings?tenant_id=${encodeURIComponent(selectedTenant)}&limit=200`,
        { asAdmin: true },
      ),
    enabled: !!selectedTenant,
  })

  const createApp = useMutation({
    mutationFn: (body: Omit<ChannelApp, 'id' | 'created_at' | 'updated_at'>) =>
      apiClient.post<ChannelApp>('/admin/v1/channel-apps', body, { asAdmin: true }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'channel-apps'] })
      setAppModalOpen(false)
      appForm.resetFields()
      message.success('Channel app created')
    },
    onError: () => message.error('Failed to create channel app'),
  })

  const updateApp = useMutation({
    mutationFn: ({ id, tenantId, body }: { id: string; tenantId: string; body: Partial<ChannelApp> }) =>
      apiClient.patch<ChannelApp>(
        `/admin/v1/channel-apps/${encodeURIComponent(id)}?tenant_id=${encodeURIComponent(tenantId)}`,
        body,
        { asAdmin: true },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'channel-apps'] })
      setAppModalOpen(false)
      setEditingApp(null)
      appForm.resetFields()
      message.success('Channel app updated')
    },
    onError: () => message.error('Failed to update channel app'),
  })

  const deleteApp = useMutation({
    mutationFn: ({ id, tenantId }: { id: string; tenantId: string }) =>
      apiClient.del(
        `/admin/v1/channel-apps/${encodeURIComponent(id)}?tenant_id=${encodeURIComponent(tenantId)}`,
        { asAdmin: true },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'channel-apps'] })
      message.success('Channel app deleted')
    },
    onError: () => message.error('Failed to delete channel app'),
  })

  const revokeBinding = useMutation({
    mutationFn: ({ id, tenantId }: { id: string; tenantId: string }) =>
      apiClient.del(
        `/admin/v1/channel-bindings/${encodeURIComponent(id)}?tenant_id=${encodeURIComponent(tenantId)}`,
        { asAdmin: true },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'channel-bindings'] })
      message.success('Binding revoked')
    },
    onError: () => message.error('Failed to revoke binding'),
  })

  const openCreate = () => {
    setEditingApp(null)
    appForm.resetFields()
    appForm.setFieldsValue({ tenant_id: selectedTenant, enabled: true })
    setAppModalOpen(true)
  }

  const openEdit = (app: ChannelApp) => {
    setEditingApp(app)
    appForm.setFieldsValue(app)
    setAppModalOpen(true)
  }

  const handleAppSubmit = async () => {
    const values = await appForm.validateFields()
    if (editingApp) {
      updateApp.mutate({ id: editingApp.id, tenantId: editingApp.tenant_id, body: values })
    } else {
      createApp.mutate(values)
    }
  }

  const refreshAll = () => {
    qc.invalidateQueries({ queryKey: ['admin', 'channel-apps'] })
    qc.invalidateQueries({ queryKey: ['admin', 'channel-bindings'] })
  }

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }}>
        <div>
          <Title level={4} style={{ margin: 0 }}>Channel Apps</Title>
          <Text type="secondary">Manage trusted channel login apps and user bindings</Text>
        </div>
        <Button icon={<RefreshCcw size={16} />} onClick={refreshAll}>Refresh</Button>
      </Space>

      <Space style={{ marginBottom: 16 }}>
        <Select
          placeholder="Select tenant"
          style={{ width: 280 }}
          value={selectedTenant || undefined}
          onChange={(v) => setSelectedTenant(v ?? '')}
          options={tenantOptions}
        />
      </Space>

      {!selectedTenant && (
        <Alert type="info" showIcon message="Select a tenant to manage channel apps and bindings." />
      )}

      {selectedTenant && (
        <Tabs
          items={[
            {
              key: 'apps',
              label: 'Channel Apps',
              children: (
                <Space direction="vertical" style={{ width: '100%' }}>
                  {unavailable(appsQuery.error) && (
                    <Alert type="warning" showIcon message="Channel store is not configured on this node." />
                  )}
                  <Button type="primary" icon={<Plus size={16} />} onClick={openCreate}>
                    Add App
                  </Button>
                  <Table
                    rowKey="id"
                    size="middle"
                    loading={appsQuery.isLoading}
                    dataSource={appsQuery.data?.items ?? []}
                    columns={[
                      {
                        title: 'Platform',
                        dataIndex: 'platform',
                        key: 'platform',
                        width: 110,
                        render: (v: string) => <Tag color="blue">{v}</Tag>,
                      },
                      {
                        title: 'App Key',
                        dataIndex: 'app_key',
                        key: 'app_key',
                        render: (v: string) => <code>{v}</code>,
                      },
                      {
                        title: 'App Secret Ref',
                        dataIndex: 'app_secret_ref',
                        key: 'app_secret_ref',
                        render: (v: string) => v ? <code>{v}</code> : <Text type="secondary">—</Text>,
                      },
                      {
                        title: 'Enabled',
                        dataIndex: 'enabled',
                        key: 'enabled',
                        width: 90,
                        render: (v: boolean, row: ChannelApp) => (
                          <Switch
                            size="small"
                            checked={v}
                            onChange={(checked) =>
                              updateApp.mutate({ id: row.id, tenantId: row.tenant_id, body: { enabled: checked } })
                            }
                          />
                        ),
                      },
                      {
                        title: 'Created',
                        dataIndex: 'created_at',
                        key: 'created_at',
                        width: 180,
                        render: (v: string) => new Date(v).toLocaleString(),
                      },
                      {
                        title: '',
                        key: 'actions',
                        width: 130,
                        render: (_: unknown, row: ChannelApp) => (
                          <Space size="small">
                            <Button size="small" onClick={() => openEdit(row)}>Edit</Button>
                            <Popconfirm
                              title="Delete this channel app?"
                              onConfirm={() => deleteApp.mutate({ id: row.id, tenantId: row.tenant_id })}
                            >
                              <Button size="small" danger>Delete</Button>
                            </Popconfirm>
                          </Space>
                        ),
                      },
                    ]}
                  />
                </Space>
              ),
            },
            {
              key: 'bindings',
              label: 'Channel Bindings',
              children: (
                <Space direction="vertical" style={{ width: '100%' }}>
                  {unavailable(bindingsQuery.error) && (
                    <Alert type="warning" showIcon message="Channel store is not configured on this node." />
                  )}
                  <Table
                    rowKey="id"
                    size="middle"
                    loading={bindingsQuery.isLoading}
                    dataSource={bindingsQuery.data?.items ?? []}
                    columns={[
                      {
                        title: 'Platform',
                        dataIndex: 'platform',
                        key: 'platform',
                        width: 110,
                        render: (v: string) => <Tag color="blue">{v}</Tag>,
                      },
                      {
                        title: 'Display Name',
                        dataIndex: 'provider_display_name',
                        key: 'provider_display_name',
                        render: (v: string) => v || <Text type="secondary">—</Text>,
                      },
                      {
                        title: 'User ID',
                        dataIndex: 'user_id',
                        key: 'user_id',
                        render: (v: string) => <code>{v}</code>,
                      },
                      {
                        title: 'Last Login',
                        dataIndex: 'last_login_at',
                        key: 'last_login_at',
                        width: 180,
                        render: (v?: string) => v ? new Date(v).toLocaleString() : <Text type="secondary">Never</Text>,
                      },
                      {
                        title: 'Status',
                        dataIndex: 'revoked_at',
                        key: 'revoked_at',
                        width: 100,
                        render: (v?: string) => v
                          ? <Tag color="red">Revoked</Tag>
                          : <Tag color="green">Active</Tag>,
                      },
                      {
                        title: '',
                        key: 'actions',
                        width: 100,
                        render: (_: unknown, row: ChannelBinding) =>
                          !row.revoked_at && (
                            <Popconfirm
                              title="Revoke this binding? The user will be logged out."
                              onConfirm={() => revokeBinding.mutate({ id: row.id, tenantId: row.tenant_id })}
                            >
                              <Button size="small" danger>Revoke</Button>
                            </Popconfirm>
                          ),
                      },
                    ]}
                  />
                </Space>
              ),
            },
          ]}
        />
      )}

      <Modal
        title={editingApp ? 'Edit Channel App' : 'Add Channel App'}
        open={appModalOpen}
        onOk={handleAppSubmit}
        onCancel={() => { setAppModalOpen(false); setEditingApp(null); appForm.resetFields() }}
        confirmLoading={createApp.isPending || updateApp.isPending}
      >
        <Form form={appForm} layout="vertical" style={{ marginTop: 16 }}>
          {!editingApp && (
            <Form.Item name="tenant_id" label="Tenant" rules={[{ required: true }]}>
              <Select options={tenantOptions} />
            </Form.Item>
          )}
          <Form.Item name="platform" label="Platform" rules={[{ required: true }]}>
            <Select options={PLATFORMS.map((p) => ({ label: p, value: p }))} disabled={!!editingApp} />
          </Form.Item>
          <Form.Item name="app_key" label="App Key" rules={[{ required: true }]}>
            <Input placeholder="e.g. cli_xxxx or corpid/agentid" disabled={!!editingApp} />
          </Form.Item>
          <Form.Item name="app_secret_ref" label="App Secret Ref">
            <Input placeholder="Secret reference key (not the raw secret)" />
          </Form.Item>
          <Form.Item name="oauth_secret_ref" label="OAuth Secret Ref">
            <Input placeholder="OAuth secret reference key" />
          </Form.Item>
          <Form.Item name="webhook_secret_ref" label="Webhook Secret Ref">
            <Input placeholder="Webhook verification token ref" />
          </Form.Item>
          <Form.Item name="enabled" label="Enabled" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
