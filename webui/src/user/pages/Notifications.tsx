import { Typography, List, Button, Tag, Empty, Space } from 'antd'
import { useNotificationStore } from '@shared/hooks/useNotification'
import dayjs from 'dayjs'

const { Title, Text } = Typography

export default function Notifications() {
  const { items, markRead, markAllRead, clear } = useNotificationStore()

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={4} style={{ margin: 0 }}>Notifications</Title>
        <Space>
          <Button size="small" onClick={markAllRead}>Mark all read</Button>
          <Button size="small" danger onClick={clear}>Clear</Button>
        </Space>
      </Space>
      {items.length === 0 ? (
        <Empty description="No notifications" />
      ) : (
        <List
          dataSource={items}
          renderItem={(item) => (
            <List.Item
              onClick={() => markRead(item.id)}
              style={{ cursor: 'pointer', opacity: item.read ? 0.6 : 1 }}
            >
              <List.Item.Meta
                title={
                  <Space>
                    <Tag color={item.type === 'error' ? 'red' : item.type === 'warning' ? 'orange' : item.type === 'success' ? 'green' : 'blue'}>
                      {item.type}
                    </Tag>
                    <Text strong={!item.read}>{item.title}</Text>
                  </Space>
                }
                description={
                  <Space direction="vertical" size={2}>
                    <Text type="secondary" style={{ fontSize: 13 }}>{item.message}</Text>
                    <Text type="secondary" style={{ fontSize: 11 }}>{dayjs(item.created_at).format('MM-DD HH:mm')}</Text>
                  </Space>
                }
              />
            </List.Item>
          )}
        />
      )}
    </div>
  )
}
