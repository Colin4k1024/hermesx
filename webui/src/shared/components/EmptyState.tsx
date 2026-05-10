import { Empty } from 'antd'

interface Props {
  description?: string
}

export function EmptyState({ description = 'No data' }: Props) {
  return <Empty description={description} style={{ padding: 48 }} />
}
