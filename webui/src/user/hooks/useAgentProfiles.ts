import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@shared/api/client'

export interface AgentProfile {
  id: string
  tenant_id: string
  user_id: string
  name: string
  description: string
  model: string
  selected_skills: string[]
  is_default: boolean
  created_at: string
  updated_at: string
}

interface AgentProfilesResponse {
  profiles: AgentProfile[]
  count: number
}

interface AgentProfileDetailResponse {
  profile: AgentProfile
  soul_content: string
}

export function useAgentProfiles() {
  return useQuery({
    queryKey: ['agent-profiles'],
    queryFn: () => apiClient.get<AgentProfilesResponse>('/v1/agent-profiles'),
  })
}

export function useAgentProfile(id: string) {
  return useQuery({
    queryKey: ['agent-profiles', id],
    queryFn: () => apiClient.get<AgentProfileDetailResponse>(`/v1/agent-profiles/${id}`),
    enabled: !!id,
  })
}

export function useCreateAgentProfile() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string; description?: string; model?: string; selected_skills?: string[]; soul_content?: string }) =>
      apiClient.post<{ profile: AgentProfile }>('/v1/agent-profiles', data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-profiles'] }),
  })
}

export function useUpdateAgentProfile() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string; name: string; description?: string; model?: string; selected_skills?: string[] }) =>
      apiClient.put<{ profile: AgentProfile }>(`/v1/agent-profiles/${id}`, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-profiles'] }),
  })
}

export function useDeleteAgentProfile() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => apiClient.del(`/v1/agent-profiles/${id}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-profiles'] }),
  })
}

export function useSetDefaultAgentProfile() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => apiClient.put(`/v1/agent-profiles/${id}/default`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-profiles'] }),
  })
}

export function useUpdateAgentSoul() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, content }: { id: string; content: string }) =>
      apiClient.put(`/v1/agent-profiles/${id}/soul`, { content }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agent-profiles'] }),
  })
}
