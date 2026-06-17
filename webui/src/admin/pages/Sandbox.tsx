import { useState } from 'react'
import { Typography, Select, Button, Space, Input, message, Alert, Popconfirm } from 'antd'
import { useTenants } from '../hooks/useTenants'
import { useSandboxPolicy, useSetSandboxPolicy, useDeleteSandboxPolicy } from '../hooks/useSandboxPolicy'

const { Title, Text } = Typography

const DEFAULT_POLICY = JSON.stringify(
  { enabled: false, max_timeout_seconds: 30, allowed_tools: [], allow_docker: false, restrict_network: true, max_stdout_kb: 64 },
  null,
  2,
)

export default function Sandbox() {
  const { data: tenantsData } = useTenants()
  const [selectedTenant, setSelectedTenant] = useState('')
  const effectiveTenant = selectedTenant || tenantsData?.tenants?.[0]?.id || ''
  const { data: policyData } = useSandboxPolicy(effectiveTenant)
  const setPolicy = useSetSandboxPolicy(effectiveTenant)
  const deletePolicy = useDeleteSandboxPolicy(effectiveTenant)
  const policyText = policyData?.policy ?? DEFAULT_POLICY
  const policySource = `${effectiveTenant}:${policyText}`
  const [editorDraft, setEditorDraft] = useState({ source: '', value: '' })
  const [jsonErrorDraft, setJsonErrorDraft] = useState({ source: '', message: '' })
  const editor = editorDraft.source === policySource ? editorDraft.value : policyText
  const jsonError = jsonErrorDraft.source === policySource ? jsonErrorDraft.message : ''

  const handleSave = async () => {
    try {
      JSON.parse(editor)
    } catch {
      setJsonErrorDraft({ source: policySource, message: 'Invalid JSON' })
      return
    }
    try {
      await setPolicy.mutateAsync(editor)
      message.success('Sandbox policy saved')
    } catch (e) {
      message.error(e instanceof Error ? e.message : 'Failed')
    }
  }

  const handleReset = async () => {
    try {
      await deletePolicy.mutateAsync()
      message.success('Policy reset to default')
    } catch (e) {
      message.error(e instanceof Error ? e.message : 'Failed')
    }
  }

  return (
    <div style={{ padding: 24 }}>
      <Title level={4}>Sandbox Policy</Title>
      <Space style={{ marginBottom: 16 }}>
        <Select
          placeholder="Select tenant"
          style={{ width: 280 }}
          value={effectiveTenant || undefined}
          onChange={setSelectedTenant}
          options={tenantsData?.tenants.map((t) => ({ label: t.name, value: t.id })) ?? []}
        />
      </Space>
      {effectiveTenant ? (
        <Space direction="vertical" style={{ width: '100%' }}>
          <Input.TextArea
            value={editor}
            onChange={(e) => {
              setEditorDraft({ source: policySource, value: e.target.value })
              setJsonErrorDraft({ source: policySource, message: '' })
            }}
            rows={14}
            style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 13 }}
          />
          {jsonError && <Alert type="error" message={jsonError} showIcon />}
          <Space>
            <Button type="primary" onClick={handleSave} loading={setPolicy.isPending}>Save</Button>
            <Popconfirm title="Reset to system default?" onConfirm={handleReset}>
              <Button danger>Reset</Button>
            </Popconfirm>
          </Space>
        </Space>
      ) : (
        <Text type="secondary">Select a tenant to configure its sandbox policy</Text>
      )}
    </div>
  )
}
