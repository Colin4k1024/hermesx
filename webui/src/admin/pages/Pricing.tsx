import { useState } from 'react'
import { Table, Button, Space, Typography, Modal, Form, Input, InputNumber, message, Popconfirm } from 'antd'
import { Plus } from 'lucide-react'
import { usePricingRules, useUpsertPricingRule, useDeletePricingRule } from '../hooks/usePricingRules'
import type { PricingRule } from '@shared/types'
import dayjs from 'dayjs'

const { Title } = Typography

export default function Pricing() {
  const { data, isLoading } = usePricingRules()
  const upsert = useUpsertPricingRule()
  const deleteRule = useDeletePricingRule()
  const [modalOpen, setModalOpen] = useState(false)
  const [form] = Form.useForm()

  const handleSave = async () => {
    try {
      const values = await form.validateFields()
      await upsert.mutateAsync(values)
      message.success('Pricing rule saved')
      setModalOpen(false)
      form.resetFields()
    } catch (e) {
      if (e instanceof Error) message.error(e.message)
    }
  }

  const openEdit = (rule: PricingRule) => {
    form.setFieldsValue(rule)
    setModalOpen(true)
  }

  const columns = [
    { title: 'Model', dataIndex: 'model_key', key: 'model_key', render: (v: string) => <code>{v}</code> },
    { title: 'Input/1K', dataIndex: 'input_per_1k', key: 'input', render: (v: number) => `$${v.toFixed(4)}` },
    { title: 'Output/1K', dataIndex: 'output_per_1k', key: 'output', render: (v: number) => `$${v.toFixed(4)}` },
    { title: 'Cache/1K', dataIndex: 'cache_read_per_1k', key: 'cache', render: (v: number) => `$${v.toFixed(4)}` },
    { title: 'Updated', dataIndex: 'updated_at', key: 'updated', render: (v: string) => dayjs(v).format('YYYY-MM-DD') },
    {
      title: '',
      key: 'actions',
      width: 140,
      render: (_: unknown, record: PricingRule) => (
        <Space size="small">
          <Button size="small" onClick={() => openEdit(record)}>Edit</Button>
          <Popconfirm title="Delete?" onConfirm={() => deleteRule.mutate(record.model_key)}>
            <Button size="small" danger>Delete</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={4} style={{ margin: 0 }}>Pricing Rules</Title>
        <Button type="primary" icon={<Plus size={16} />} onClick={() => { form.resetFields(); setModalOpen(true) }}>
          Add Rule
        </Button>
      </Space>
      <Table dataSource={data?.rules ?? []} columns={columns} rowKey="model_key" loading={isLoading} pagination={false} size="middle" />
      <Modal title="Pricing Rule" open={modalOpen} onOk={handleSave} onCancel={() => setModalOpen(false)} confirmLoading={upsert.isPending}>
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="model_key" label="Model Key" rules={[{ required: true }]}>
            <Input placeholder="e.g. gpt-4o" />
          </Form.Item>
          <Form.Item name="input_per_1k" label="Input per 1K tokens ($)" rules={[{ required: true }]}>
            <InputNumber min={0} step={0.0001} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="output_per_1k" label="Output per 1K tokens ($)" rules={[{ required: true }]}>
            <InputNumber min={0} step={0.0001} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="cache_read_per_1k" label="Cache read per 1K ($)" rules={[{ required: true }]}>
            <InputNumber min={0} step={0.0001} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
