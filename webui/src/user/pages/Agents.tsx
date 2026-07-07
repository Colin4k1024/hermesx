import { useState } from 'react'
import { Card, Button, Space, Typography, Tag, Modal, Form, Input, Select, Empty, Popconfirm, message, Drawer } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, StarOutlined, StarFilled, RobotOutlined } from '@ant-design/icons'
import {
  useAgentProfiles,
  useCreateAgentProfile,
  useUpdateAgentProfile,
  useDeleteAgentProfile,
  useSetDefaultAgentProfile,
  useUpdateAgentSoul,
  type AgentProfile,
} from '../hooks/useAgentProfiles'

const { Title, Text, Paragraph } = Typography
const { TextArea } = Input

const MODEL_OPTIONS = [
  { label: 'Default', value: '' },
  { label: 'Claude Sonnet', value: 'claude-sonnet-4-20250514' },
  { label: 'Claude Opus', value: 'claude-opus-4-20250514' },
  { label: 'GPT-4o', value: 'gpt-4o' },
  { label: 'DeepSeek', value: 'deepseek-chat' },
]

export default function Agents() {
  const { data, isLoading } = useAgentProfiles()
  const createMutation = useCreateAgentProfile()
  const updateMutation = useUpdateAgentProfile()
  const deleteMutation = useDeleteAgentProfile()
  const setDefaultMutation = useSetDefaultAgentProfile()
  const updateSoulMutation = useUpdateAgentSoul()

  const [createOpen, setCreateOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [soulOpen, setSoulOpen] = useState(false)
  const [editingProfile, setEditingProfile] = useState<AgentProfile | null>(null)
  const [soulContent, setSoulContent] = useState('')

  const [form] = Form.useForm()
  const [editForm] = Form.useForm()

  const profiles = data?.profiles ?? []

  const handleCreate = async () => {
    try {
      const values = await form.validateFields()
      await createMutation.mutateAsync({
        name: values.name,
        description: values.description || '',
        model: values.model || '',
        selected_skills: values.selected_skills || [],
      })
      message.success('Agent created')
      setCreateOpen(false)
      form.resetFields()
    } catch {
      // validation error
    }
  }

  const handleEdit = (profile: AgentProfile) => {
    setEditingProfile(profile)
    editForm.setFieldsValue({
      name: profile.name,
      description: profile.description,
      model: profile.model,
      selected_skills: profile.selected_skills,
    })
    setEditOpen(true)
  }

  const handleUpdate = async () => {
    if (!editingProfile) return
    try {
      const values = await editForm.validateFields()
      await updateMutation.mutateAsync({
        id: editingProfile.id,
        name: values.name,
        description: values.description || '',
        model: values.model || '',
        selected_skills: values.selected_skills || [],
      })
      message.success('Agent updated')
      setEditOpen(false)
      setEditingProfile(null)
    } catch {
      // validation error
    }
  }

  const handleDelete = async (id: string) => {
    await deleteMutation.mutateAsync(id)
    message.success('Agent deleted')
  }

  const handleSetDefault = async (id: string) => {
    await setDefaultMutation.mutateAsync(id)
    message.success('Default agent set')
  }

  const handleEditSoul = (profile: AgentProfile) => {
    setEditingProfile(profile)
    // Load soul content via the detail endpoint
    fetch(`/v1/agent-profiles/${profile.id}`, { credentials: 'include' })
      .then((res) => res.json())
      .then((data) => {
        setSoulContent(data.soul_content || '')
        setSoulOpen(true)
      })
      .catch(() => {
        setSoulContent('')
        setSoulOpen(true)
      })
  }

  const handleSaveSoul = async () => {
    if (!editingProfile) return
    await updateSoulMutation.mutateAsync({ id: editingProfile.id, content: soulContent })
    message.success('SOUL.md updated')
    setSoulOpen(false)
  }

  return (
    <div style={{ padding: 24 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <Title level={4} style={{ margin: 0 }}>Agent Profiles</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          Create Agent
        </Button>
      </div>

      {profiles.length === 0 && !isLoading && (
        <Empty description="No agents yet. Create your first agent to get started." />
      )}

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: 16 }}>
        {profiles.map((profile) => (
          <Card
            key={profile.id}
            hoverable
            actions={[
              profile.is_default ? (
                <StarFilled key="default" style={{ color: '#faad14' }} />
              ) : (
                <StarOutlined key="default" onClick={() => handleSetDefault(profile.id)} title="Set as default" />
              ),
              <EditOutlined key="edit" onClick={() => handleEdit(profile)} />,
              <RobotOutlined key="soul" onClick={() => handleEditSoul(profile)} title="Edit SOUL.md" />,
              <Popconfirm key="delete" title="Delete this agent?" onConfirm={() => handleDelete(profile.id)}>
                <DeleteOutlined />
              </Popconfirm>,
            ]}
          >
            <Card.Meta
              title={
                <Space>
                  {profile.name}
                  {profile.is_default && <Tag color="gold">Default</Tag>}
                </Space>
              }
              description={
                <Space direction="vertical" size="small" style={{ width: '100%' }}>
                  <Paragraph ellipsis={{ rows: 2 }} style={{ margin: 0 }}>
                    {profile.description || 'No description'}
                  </Paragraph>
                  <Space size="small">
                    {profile.model && <Tag>{profile.model}</Tag>}
                    {profile.selected_skills?.length > 0 && (
                      <Tag color="blue">{profile.selected_skills.length} skills</Tag>
                    )}
                  </Space>
                </Space>
              }
            />
          </Card>
        ))}
      </div>

      {/* Create Agent Modal */}
      <Modal
        title="Create Agent"
        open={createOpen}
        onOk={handleCreate}
        onCancel={() => { setCreateOpen(false); form.resetFields() }}
        confirmLoading={createMutation.isPending}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="Name" rules={[{ required: true, message: 'Please enter a name' }]}>
            <Input placeholder="My Coding Agent" />
          </Form.Item>
          <Form.Item name="description" label="Description">
            <TextArea rows={2} placeholder="What does this agent do?" />
          </Form.Item>
          <Form.Item name="model" label="Model">
            <Select options={MODEL_OPTIONS} placeholder="Use default model" />
          </Form.Item>
          <Form.Item name="selected_skills" label="Skills (leave empty for all)">
            <Select mode="tags" placeholder="Type skill names" />
          </Form.Item>
        </Form>
      </Modal>

      {/* Edit Agent Modal */}
      <Modal
        title="Edit Agent"
        open={editOpen}
        onOk={handleUpdate}
        onCancel={() => { setEditOpen(false); setEditingProfile(null) }}
        confirmLoading={updateMutation.isPending}
      >
        <Form form={editForm} layout="vertical">
          <Form.Item name="name" label="Name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="description" label="Description">
            <TextArea rows={2} />
          </Form.Item>
          <Form.Item name="model" label="Model">
            <Select options={MODEL_OPTIONS} placeholder="Use default model" />
          </Form.Item>
          <Form.Item name="selected_skills" label="Skills">
            <Select mode="tags" placeholder="Type skill names" />
          </Form.Item>
        </Form>
      </Modal>

      {/* SOUL.md Editor Drawer */}
      <Drawer
        title={`Edit SOUL.md — ${editingProfile?.name ?? ''}`}
        open={soulOpen}
        onClose={() => setSoulOpen(false)}
        width={600}
        extra={
          <Button type="primary" onClick={handleSaveSoul} loading={updateSoulMutation.isPending}>
            Save
          </Button>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
          Define this agent's persona and behavior. This content is injected into the system prompt.
        </Text>
        <TextArea
          value={soulContent}
          onChange={(e) => setSoulContent(e.target.value)}
          rows={20}
          style={{ fontFamily: 'monospace' }}
          placeholder="# My Agent&#10;&#10;You are a helpful assistant..."
        />
      </Drawer>
    </div>
  )
}
