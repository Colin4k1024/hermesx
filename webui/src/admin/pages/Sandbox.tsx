import { useState, useEffect } from 'react'
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

  useEffect(() => {
    const first = tenantsData?.tenants?.[0]
    if (!selectedTenant && first) {
      setSelectedTenant(first.id)
    }
  }, [tenantsData, selectedTenant])
  const { data: policyData } = useSandboxPolicy(selectedTenant)
  const setPolicy = useSetSandboxPolicy(selectedTenant)
  const deletePolicy = useDeleteSandboxPolicy(selectedTenant)
  const [editor, setEditor] = useState('')
  const [jsonError, setJsonError] = useState('')

  useEffect(() => {
    setEditor(policyData?.policy ?? DEFAULT_POLICY)
    setJsonError('')
  }, [policyData])

  const handleSave = async () => {
    try {
      JSON.parse(editor)
    } catch {
      setJsonError('Invalid JSON')
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
          value={selectedTenant || undefined}
          onChange={setSelectedTenant}
          options={tenantsData?.tenants.map((t) => ({ label: t.name, value: t.id })) ?? []}
        />
      </Space>
      {selectedTenant ? (
        <Space direction="vertical" style={{ width: '100%' }}>
          <Input.TextArea
            value={editor}
            onChange={(e) => { setEditor(e.target.value); setJsonError('') }}
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
