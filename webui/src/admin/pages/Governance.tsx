import { useEffect, useState } from 'react'
import { Alert, Button, Form, Input, Modal, Popconfirm, Select, Space, Switch, Table, Tabs, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { RefreshCcw, RotateCcw, ShieldCheck } from 'lucide-react'
import dayjs from 'dayjs'
import { ApiError } from '@shared/api/client'
import type { SharingMode, SharingPolicyHistoryEntry } from '@shared/types'
import { useTenants } from '../hooks/useTenants'
import {
  SharedKnowledgeRevokeRequest,
  useGlobalSharingPolicy,
  useGlobalSharingPolicyHistory,
  useRevokeSharedKnowledge,
  useRollbackGlobalSharingPolicy,
  useRollbackTenantSharingPolicy,
  useTenantSharingPolicy,
  useTenantSharingPolicyHistory,
  useUpdateGlobalSharingPolicy,
  useUpdateTenantSharingPolicy,
} from '../hooks/useGovernance'

const { Title, Text } = Typography

const sharingOptions = [
  { label: 'Disabled', value: 'disabled' },
  { label: 'Anonymous', value: 'anonymous' },
  { label: 'Trusted', value: 'trusted' },
] satisfies Array<{ label: string; value: SharingMode }>

function modeColor(mode?: string) {
  if (mode === 'trusted') return 'green'
  if (mode === 'anonymous') return 'blue'
  return 'default'
}

function unavailable(error: unknown) {
  return error instanceof ApiError && error.status === 503
}

interface RollbackTarget {
  scope: 'global' | 'tenant'
  version: number
}

export default function Governance() {
  const { data: tenantsData } = useTenants()
  const [selectedTenant, setSelectedTenant] = useState('')
  const [rollbackTarget, setRollbackTarget] = useState<RollbackTarget | null>(null)
  const [globalForm] = Form.useForm()
  const [tenantForm] = Form.useForm()
  const [revokeForm] = Form.useForm()
  const [rollbackForm] = Form.useForm()

  const globalPolicy = useGlobalSharingPolicy()
  const globalHistory = useGlobalSharingPolicyHistory()
  const updateGlobal = useUpdateGlobalSharingPolicy()
  const rollbackGlobal = useRollbackGlobalSharingPolicy()
  const tenantPolicy = useTenantSharingPolicy(selectedTenant)
  const tenantHistory = useTenantSharingPolicyHistory(selectedTenant)
  const updateTenant = useUpdateTenantSharingPolicy(selectedTenant)
  const rollbackTenant = useRollbackTenantSharingPolicy(selectedTenant)
  const revokeShared = useRevokeSharedKnowledge()

  useEffect(() => {
    const first = tenantsData?.tenants?.[0]
    if (!selectedTenant && first) setSelectedTenant(first.id)
  }, [selectedTenant, tenantsData])

  useEffect(() => {
    if (globalPolicy.data) {
      globalForm.setFieldsValue({ mode: globalPolicy.data.mode, reason: '' })
    }
  }, [globalForm, globalPolicy.data])

  useEffect(() => {
    if (tenantPolicy.data) {
      tenantForm.setFieldsValue({
        consume_shared: tenantPolicy.data.consume_shared,
        contribution_mode: tenantPolicy.data.contribution_mode,
        labels: tenantPolicy.data.labels ?? [],
        reason: '',
      })
    }
  }, [tenantForm, tenantPolicy.data])

  const historyColumns = (scope: 'global' | 'tenant'): ColumnsType<SharingPolicyHistoryEntry> => [
    { title: 'Version', dataIndex: 'version', key: 'version', width: 100 },
    {
      title: 'Mode',
      key: 'mode',
      width: 140,
      render: (_, row) => {
        const mode = row.mode ?? row.contribution_mode
        return mode ? <Tag color={modeColor(mode)}>{mode}</Tag> : <Text type="secondary">mixed</Text>
      },
    },
    {
      title: 'Consume',
      dataIndex: 'consume_shared',
      key: 'consume_shared',
      width: 100,
      render: (v?: boolean) => (v == null ? <Text type="secondary">-</Text> : <Tag color={v ? 'green' : 'default'}>{v ? 'yes' : 'no'}</Tag>),
    },
    { title: 'Reason', dataIndex: 'reason', key: 'reason', width: 240, ellipsis: true },
    { title: 'Changed', dataIndex: 'changed_at', key: 'changed_at', width: 170, render: (v: string) => dayjs(v).format('YYYY-MM-DD HH:mm') },
    {
      title: '',
      key: 'actions',
      width: 120,
      render: (_, row) => (
        <Button size="small" icon={<RotateCcw size={14} />} onClick={() => setRollbackTarget({ scope, version: row.version })}>
          Rollback
        </Button>
      ),
    },
  ]

  const saveGlobal = async () => {
    try {
      const values = await globalForm.validateFields()
      await updateGlobal.mutateAsync(values)
      message.success('Global policy saved')
    } catch (e) {
      if (e instanceof Error) message.error(e.message)
    }
  }

  const saveTenant = async () => {
    if (!selectedTenant) return
    try {
      const values = await tenantForm.validateFields()
      await updateTenant.mutateAsync(values)
      message.success('Tenant policy saved')
    } catch (e) {
      if (e instanceof Error) message.error(e.message)
    }
  }

  const submitRevoke = async () => {
    try {
      const values = await revokeForm.validateFields()
      const body: SharedKnowledgeRevokeRequest = {
        task_class: values.task_class || undefined,
        source_tenant: values.source_tenant || undefined,
        source: values.source || undefined,
        from: values.from || undefined,
        to: values.to || undefined,
        confirm_all: values.confirm_all,
        reason: values.reason || undefined,
      }
      const hasCriteria = body.task_class || body.source_tenant || body.source || body.from || body.to || body.confirm_all
      if (!hasCriteria) {
        message.error('Provide criteria or enable confirm all')
        return
      }
      const result = await revokeShared.mutateAsync(body)
      message.success(`Revoked ${result.deleted} shared records`)
      revokeForm.resetFields()
    } catch (e) {
      if (e instanceof Error) message.error(e.message)
    }
  }

  const submitRollback = async () => {
    if (!rollbackTarget) return
    try {
      const values = await rollbackForm.validateFields()
      const body = { version: rollbackTarget.version, reason: values.reason || undefined }
      if (rollbackTarget.scope === 'global') {
        await rollbackGlobal.mutateAsync(body)
      } else {
        await rollbackTenant.mutateAsync(body)
      }
      message.success('Rollback complete')
      setRollbackTarget(null)
      rollbackForm.resetFields()
    } catch (e) {
      if (e instanceof Error) message.error(e.message)
    }
  }

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }} align="start">
        <div>
          <Title level={4} style={{ margin: 0 }}>Governance</Title>
          <Space wrap style={{ marginTop: 8 }}>
            <Tag icon={<ShieldCheck size={13} />} color={modeColor(globalPolicy.data?.mode)}>
              global {globalPolicy.data?.mode ?? 'unavailable'}
            </Tag>
            {globalPolicy.data && <Tag>v{globalPolicy.data.version}</Tag>}
          </Space>
        </div>
        <Button icon={<RefreshCcw size={16} />} onClick={() => {
          globalPolicy.refetch()
          globalHistory.refetch()
          tenantPolicy.refetch()
          tenantHistory.refetch()
        }}>
          Refresh
        </Button>
      </Space>

      {unavailable(globalPolicy.error) && <Alert type="warning" showIcon style={{ marginBottom: 16 }} message="Evolution governance store is not configured on this API node" />}

      <Tabs
        items={[
          {
            key: 'global',
            label: 'Global Policy',
            children: (
              <Space direction="vertical" style={{ width: '100%' }}>
                <Form form={globalForm} layout="inline">
                  <Form.Item name="mode" label="Mode" rules={[{ required: true }]}>
                    <Select style={{ width: 180 }} options={sharingOptions} />
                  </Form.Item>
                  <Form.Item name="reason" label="Reason">
                    <Input style={{ width: 360 }} placeholder="operator note" />
                  </Form.Item>
                  <Button type="primary" onClick={saveGlobal} loading={updateGlobal.isPending}>Save</Button>
                </Form>
                <Table
                  rowKey="version"
                  size="middle"
                  loading={globalHistory.isLoading}
                  dataSource={globalHistory.data?.entries ?? []}
                  columns={historyColumns('global')}
                  scroll={{ x: 870 }}
                />
              </Space>
            ),
          },
          {
            key: 'tenant',
            label: 'Tenant Policy',
            children: (
              <Space direction="vertical" style={{ width: '100%' }}>
                <Select
                  placeholder="Select tenant"
                  style={{ width: 300 }}
                  value={selectedTenant || undefined}
                  onChange={setSelectedTenant}
                  options={tenantsData?.tenants.map((t) => ({ label: t.name, value: t.id })) ?? []}
                />
                {tenantPolicy.data && (
                  <Space wrap>
                    <Tag>v{tenantPolicy.data.version}</Tag>
                    <Tag color={modeColor(tenantPolicy.data.global_mode)}>global {tenantPolicy.data.global_mode}</Tag>
                    <Tag color={modeColor(tenantPolicy.data.effective_contribution_mode)}>effective {tenantPolicy.data.effective_contribution_mode}</Tag>
                  </Space>
                )}
                <Form form={tenantForm} layout="inline">
                  <Form.Item name="consume_shared" label="Consume" valuePropName="checked">
                    <Switch />
                  </Form.Item>
                  <Form.Item name="contribution_mode" label="Contribution" rules={[{ required: true }]}>
                    <Select style={{ width: 180 }} options={sharingOptions} />
                  </Form.Item>
                  <Form.Item name="labels" label="Labels">
                    <Select mode="tags" style={{ width: 260 }} tokenSeparators={[',']} />
                  </Form.Item>
                  <Form.Item name="reason" label="Reason">
                    <Input style={{ width: 300 }} placeholder="operator note" />
                  </Form.Item>
                  <Button type="primary" onClick={saveTenant} loading={updateTenant.isPending} disabled={!selectedTenant}>Save</Button>
                </Form>
                <Table
                  rowKey="version"
                  size="middle"
                  loading={tenantHistory.isLoading}
                  dataSource={tenantHistory.data?.entries ?? []}
                  columns={historyColumns('tenant')}
                  scroll={{ x: 870 }}
                />
              </Space>
            ),
          },
          {
            key: 'revoke',
            label: 'Shared Revoke',
            children: (
              <Form form={revokeForm} layout="vertical" style={{ maxWidth: 760 }} initialValues={{ confirm_all: false }}>
                <Space wrap align="start">
                  <Form.Item name="task_class" label="Task Class">
                    <Input style={{ width: 220 }} />
                  </Form.Item>
                  <Form.Item name="source_tenant" label="Source Tenant">
                    <Input style={{ width: 220 }} />
                  </Form.Item>
                  <Form.Item name="source" label="Source">
                    <Input style={{ width: 220 }} />
                  </Form.Item>
                  <Form.Item name="from" label="From">
                    <Input style={{ width: 220 }} placeholder="2026-06-01T00:00:00Z" />
                  </Form.Item>
                  <Form.Item name="to" label="To">
                    <Input style={{ width: 220 }} placeholder="2026-06-09T00:00:00Z" />
                  </Form.Item>
                  <Form.Item name="confirm_all" label="Confirm All" valuePropName="checked">
                    <Switch />
                  </Form.Item>
                </Space>
                <Form.Item name="reason" label="Reason">
                  <Input.TextArea rows={3} />
                </Form.Item>
                <Popconfirm title="Revoke matching shared knowledge?" onConfirm={submitRevoke}>
                  <Button danger loading={revokeShared.isPending}>Revoke</Button>
                </Popconfirm>
              </Form>
            ),
          },
        ]}
      />

      <Modal
        title={`Rollback ${rollbackTarget?.scope ?? ''} policy`}
        open={!!rollbackTarget}
        onOk={submitRollback}
        onCancel={() => setRollbackTarget(null)}
        confirmLoading={rollbackGlobal.isPending || rollbackTenant.isPending}
      >
        <Form form={rollbackForm} layout="vertical" style={{ marginTop: 16 }}>
          <Text>Version {rollbackTarget?.version}</Text>
          <Form.Item name="reason" label="Reason" style={{ marginTop: 16 }}>
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
