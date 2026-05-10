import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@shared/api/client'
import type { MemoryListResponse } from '@shared/types'

export function useMemories() {
  return useQuery({
    queryKey: ['user', 'memories'],
    queryFn: () => apiClient.get<MemoryListResponse>('/v1/memories'),
  })
}

export function useDeleteMemory() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (key: string) => apiClient.del(`/v1/memories/${encodeURIComponent(key)}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['user', 'memories'] }),
  })
}
