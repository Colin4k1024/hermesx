import { useQuery } from '@tanstack/react-query'
import { apiClient } from '@shared/api/client'

export interface ArtifactItem {
  name: string
  type: string
  url: string
  size_bytes: number
  created_at: string
}

export function useArtifacts(sessionId: string | null) {
  return useQuery({
    queryKey: ['artifacts', sessionId],
    queryFn: () => apiClient.get<ArtifactItem[]>(`/v1/sessions/${sessionId}/artifacts`),
    enabled: !!sessionId,
    refetchInterval: (query) => {
      // Poll while session is active (data might change)
      return query.state.status === 'success' ? false : 30_000
    },
  })
}
