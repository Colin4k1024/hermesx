import { Skeleton, Space } from 'antd'

export function PageSkeleton() {
  return (
    <Space direction="vertical" size="large" style={{ width: '100%', padding: 24 }}>
      <Skeleton.Input active style={{ width: 200 }} />
      <Skeleton active paragraph={{ rows: 4 }} />
      <Skeleton active paragraph={{ rows: 4 }} />
    </Space>
  )
}
