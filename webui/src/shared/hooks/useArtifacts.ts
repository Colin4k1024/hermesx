import { useQuery } from '@tanstack/react-query'
import { apiClient } from '@shared/api/client'

export interface ArtifactItem {
  name: string
  type: string
  url: string
  size_bytes: number
  created_at: string
}

interface ArtifactsResponse {
  artifacts: ArtifactItem[]
  count: number
}

export function useArtifacts(sessionId: string | null) {
  return useQuery({
    queryKey: ['artifacts', sessionId],
    queryFn: async () => {
      const res = await apiClient.get<ArtifactsResponse>(`/v1/sessions/${sessionId}/artifacts`)
      return res.artifacts ?? []
    },
    enabled: !!sessionId,
    refetchInterval: (query) => {
      // Poll while session is active (data might change)
      return query.state.status === 'success' ? false : 30_000
    },
  })
}
