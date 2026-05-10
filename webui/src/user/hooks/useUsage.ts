import { useQuery } from '@tanstack/react-query'
import { apiClient } from '@shared/api/client'
import type { UsageResponse } from '@shared/types'

export function useUsage() {
  return useQuery({
    queryKey: ['user', 'usage'],
    queryFn: () => apiClient.get<UsageResponse>('/v1/usage'),
  })
}
