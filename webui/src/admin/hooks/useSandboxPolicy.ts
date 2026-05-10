import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@shared/api/client'
import type { SandboxPolicy } from '@shared/types'

export function useSandboxPolicy(tenantId: string) {
  return useQuery({
    queryKey: ['admin', 'sandbox', tenantId],
    queryFn: () => apiClient.get<SandboxPolicy>(`/admin/v1/tenants/${tenantId}/sandbox-policy`, { asAdmin: true }),
    enabled: !!tenantId,
  })
}

export function useSetSandboxPolicy(tenantId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (policy: string) =>
      apiClient.post(`/admin/v1/tenants/${tenantId}/sandbox-policy`, { policy }, { asAdmin: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'sandbox', tenantId] }),
  })
}

export function useDeleteSandboxPolicy(tenantId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => apiClient.del(`/admin/v1/tenants/${tenantId}/sandbox-policy`, { asAdmin: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'sandbox', tenantId] }),
  })
}
