import { useQuery } from '@tanstack/react-query'
import { apiClient } from '@shared/api/client'
import type { SkillListResponse } from '@shared/types'

export function useSkills() {
  return useQuery({
    queryKey: ['user', 'skills'],
    queryFn: () => apiClient.get<SkillListResponse>('/v1/skills'),
  })
}
