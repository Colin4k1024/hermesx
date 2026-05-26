import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Alert, Button, Form, Input, InputNumber, Modal, Select, Space, Table, Tabs, Tag, Typography, message } from 'antd'
import { Plus, RefreshCcw } from 'lucide-react'
import { apiClient, ApiError } from '@shared/api/client'
import { useTenants } from '../hooks/useTenants'

const { Title, Text } = Typography

interface SecretPattern {
  name: string
  pattern: string
  severity: string
}

interface CanaryToken {
  id: string
  tenant_id: string
  created_at: string
  expires_at?: string
}

interface SafetyRule {
  tenant_id: string
  mode: string
  input_patterns?: unknown[]
  output_rules?: unknown[]
}

interface EgressRule {
  id: string
  tenant_id: string
  host_pattern: string
  path_prefix: string
  action: string
  priority: number
}

interface EgressBlockedLog {
  note?: string
  rules?: EgressRule[]
}

function useAdminQuery<T>(key: unknown[], path: string) {
  return useQuery({
    queryKey: key,
    queryFn: () => apiClient.get<T>(path, { asAdmin: true }),
    retry: false,
  })
}

function unavailable(error: unknown) {
  return error instanceof ApiError && error.status === 503
}

export default function Security() {
  const qc = useQueryClient()
  const { data: tenantsData } = useTenants()
  const [selectedTenant, setSelectedTenant] = useState('')
  const [patternOpen, setPatternOpen] = useState(false)
  const [egressOpen, setEgressOpen] = useState(false)
  const [patternForm] = Form.useForm()
  const [egressForm] = Form.useForm()

  const patterns = useAdminQuery<SecretPattern[]>(['admin', 'security', 'patterns'], '/admin/v1/secrets/patterns')
  const canaries = useAdminQuery<CanaryToken[]>(['admin', 'security', 'canaries'], '/admin/v1/secrets/canary-tokens')
  const safetyRules = useAdminQuery<SafetyRule[]>(['admin', 'security', 'safety'], '/admin/v1/safety/rules')
  const egressPath = selectedTenant
    ? `/admin/v1/egress/allowlist?tenant_id=${encodeURIComponent(selectedTenant)}`
    : '/admin/v1/egress/allowlist'
  const egressRules = useAdminQuery<EgressRule[]>(['admin', 'security', 'egress', selectedTenant], egressPath)
  const blockedLog = useAdminQuery<EgressBlockedLog>(['admin', 'security', 'egress-blocked', selectedTenant], `/admin/v1/egress/blocked-log${selectedTenant ? `?tenant_id=${encodeURIComponent(selectedTenant)}` : ''}`)

  const createPattern = useMutation({
    mutationFn: (body: SecretPattern) => apiClient.post<SecretPattern>('/admin/v1/secrets/patterns', body, { asAdmin: true }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'security', 'patterns'] })
      setPatternOpen(false)
      patternForm.resetFields()
      message.success('Pattern added')
    },
  })

  const createEgress = useMutation({
    mutationFn: (body: Omit<EgressRule, 'id'>) => apiClient.post<{ id: string }>('/admin/v1/egress/allowlist', body, { asAdmin: true }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'security', 'egress'] })
      qc.invalidateQueries({ queryKey: ['admin', 'security', 'egress-blocked'] })
      setEgressOpen(false)
      egressForm.resetFields()
      message.success('Egress rule added')
    },
  })

  const revokeCanary = useMutation({
    mutationFn: (id: string) => apiClient.del(`/admin/v1/secrets/canary-tokens/${encodeURIComponent(id)}`, { asAdmin: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'security', 'canaries'] }),
  })

  const deleteEgress = useMutation({
    mutationFn: (rule: EgressRule) => apiClient.del(`/admin/v1/egress/allowlist/${encodeURIComponent(rule.id)}?tenant_id=${encodeURIComponent(rule.tenant_id)}`, { asAdmin: true }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'security', 'egress'] })
      qc.invalidateQueries({ queryKey: ['admin', 'security', 'egress-blocked'] })
    },
  })

  const tenantOptions = tenantsData?.tenants.map((t) => ({ label: t.name, value: t.id })) ?? []
  const openEgressModal = () => {
    egressForm.setFieldsValue({ tenant_id: selectedTenant, path_prefix: '/', action: 'allow', priority: 100 })
    setEgressOpen(true)
  }

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }}>
        <div>
          <Title level={4} style={{ margin: 0 }}>Security Governance</Title>
          <Text type="secondary">Secret detection, canary tokens, safety rules, and egress allowlists</Text>
        </div>
        <Button icon={<RefreshCcw size={16} />} onClick={() => qc.invalidateQueries({ queryKey: ['admin', 'security'] })}>
          Refresh
        </Button>
      </Space>

      <Tabs
        items={[
          {
            key: 'patterns',
            label: 'Secret Patterns',
            children: (
              <Space direction="vertical" style={{ width: '100%' }}>
                {unavailable(patterns.error) && <Alert type="warning" showIcon message="Leak scanner is not configured on this API node" />}
                <Button type="primary" icon={<Plus size={16} />} onClick={() => setPatternOpen(true)}>Add Pattern</Button>
                <Table
                  rowKey="name"
                  size="middle"
                  loading={patterns.isLoading}
                  dataSource={patterns.data ?? []}
                  columns={[
                    { title: 'Name', dataIndex: 'name', key: 'name', render: (v: string) => <code>{v}</code> },
                    { title: 'Severity', dataIndex: 'severity', key: 'severity', width: 120, render: (v: string) => <Tag color={v === 'critical' ? 'red' : v === 'high' ? 'orange' : 'blue'}>{v}</Tag> },
                    { title: 'Pattern', dataIndex: 'pattern', key: 'pattern', ellipsis: true },
                  ]}
                />
              </Space>
            ),
          },
          {
            key: 'canaries',
            label: 'Canary Tokens',
            children: (
              <Space direction="vertical" style={{ width: '100%' }}>
                {unavailable(canaries.error) && <Alert type="warning" showIcon message="Canary detector is not configured on this API node" />}
                <Table
                  rowKey="id"
                  size="middle"
                  loading={canaries.isLoading}
                  dataSource={canaries.data ?? []}
                  columns={[
                    { title: 'ID', dataIndex: 'id', key: 'id', render: (v: string) => <code>{v}</code> },
                    { title: 'Tenant', dataIndex: 'tenant_id', key: 'tenant_id', render: (v: string) => <code>{v}</code> },
                    { title: 'Created', dataIndex: 'created_at', key: 'created_at' },
                    { title: '', key: 'actions', width: 120, render: (_: unknown, row: CanaryToken) => <Button size="small" danger onClick={() => revokeCanary.mutate(row.id)}>Revoke</Button> },
                  ]}
                />
              </Space>
            ),
          },
          {
            key: 'safety',
            label: 'Safety Rules',
            children: (
              <Space direction="vertical" style={{ width: '100%' }}>
                {unavailable(safetyRules.error) && <Alert type="warning" showIcon message="Safety policy store is not configured on this API node" />}
                <Table
                  rowKey="tenant_id"
                  size="middle"
                  loading={safetyRules.isLoading}
                  dataSource={safetyRules.data ?? []}
                  columns={[
                    { title: 'Tenant', dataIndex: 'tenant_id', key: 'tenant_id', render: (v: string) => <code>{v}</code> },
                    { title: 'Mode', dataIndex: 'mode', key: 'mode', width: 130, render: (v: string) => <Tag>{v}</Tag> },
                    { title: 'Input Patterns', dataIndex: 'input_patterns', key: 'input_patterns', render: (v?: unknown[]) => v?.length ?? 0 },
                    { title: 'Output Rules', dataIndex: 'output_rules', key: 'output_rules', render: (v?: unknown[]) => v?.length ?? 0 },
                  ]}
                />
              </Space>
            ),
          },
          {
            key: 'egress',
            label: 'Egress',
            children: (
              <Space direction="vertical" style={{ width: '100%' }}>
                <Space wrap>
                  <Select
                    allowClear
                    placeholder="All tenants"
                    style={{ width: 280 }}
                    value={selectedTenant || undefined}
                    onChange={(v) => setSelectedTenant(v ?? '')}
                    options={tenantOptions}
                  />
                  <Button type="primary" icon={<Plus size={16} />} onClick={openEgressModal}>Add Rule</Button>
                </Space>
                {blockedLog.data?.note && <Alert type="info" showIcon message={blockedLog.data.note} />}
                <Table
                  rowKey="id"
                  size="middle"
                  loading={egressRules.isLoading}
                  dataSource={egressRules.data ?? blockedLog.data?.rules ?? []}
                  columns={[
                    { title: 'Host', dataIndex: 'host_pattern', key: 'host_pattern', render: (v: string) => <code>{v}</code> },
                    { title: 'Path', dataIndex: 'path_prefix', key: 'path_prefix', render: (v: string) => <code>{v}</code> },
                    { title: 'Action', dataIndex: 'action', key: 'action', width: 100, render: (v: string) => <Tag color={v === 'deny' ? 'red' : 'green'}>{v}</Tag> },
                    { title: 'Priority', dataIndex: 'priority', key: 'priority', width: 100 },
                    { title: 'Tenant', dataIndex: 'tenant_id', key: 'tenant_id', render: (v: string) => <code>{v}</code> },
                    { title: '', key: 'actions', width: 100, render: (_: unknown, row: EgressRule) => <Button size="small" danger onClick={() => deleteEgress.mutate(row)}>Delete</Button> },
                  ]}
                />
              </Space>
            ),
          },
        ]}
      />

      <Modal title="Secret Pattern" open={patternOpen} onOk={() => patternForm.validateFields().then((values) => createPattern.mutate(values))} onCancel={() => setPatternOpen(false)} confirmLoading={createPattern.isPending}>
        <Form form={patternForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label="Name" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="pattern" label="Regex Pattern" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="severity" label="Severity" initialValue="high"><Select options={['critical', 'high', 'medium', 'low'].map((v) => ({ label: v, value: v }))} /></Form.Item>
        </Form>
      </Modal>

      <Modal title="Egress Rule" open={egressOpen} onOk={() => egressForm.validateFields().then((values) => createEgress.mutate(values))} onCancel={() => setEgressOpen(false)} confirmLoading={createEgress.isPending}>
        <Form form={egressForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="tenant_id" label="Tenant" rules={[{ required: true }]}><Select options={tenantOptions} /></Form.Item>
          <Form.Item name="host_pattern" label="Host Pattern" rules={[{ required: true }]}><Input placeholder="api.example.com" /></Form.Item>
          <Form.Item name="path_prefix" label="Path Prefix" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="action" label="Action"><Select options={[{ label: 'allow', value: 'allow' }, { label: 'deny', value: 'deny' }]} /></Form.Item>
          <Form.Item name="priority" label="Priority"><InputNumber style={{ width: '100%' }} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
