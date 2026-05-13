import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@shared/api/client'
import type { TenantListResponse, TenantItem } from '@shared/types'

export function useTenants() {
  return useQuery({
    queryKey: ['admin', 'tenants'],
    queryFn: () => apiClient.get<TenantListResponse>('/v1/tenants', { asAdmin: true }),
  })
}

export function useCreateTenant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string }) =>
      apiClient.post<TenantItem>('/v1/tenants', data, { asAdmin: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'tenants'] }),
  })
}

export function useDeleteTenant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => apiClient.del(`/v1/tenants/${encodeURIComponent(id)}`, { asAdmin: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'tenants'] }),
  })
}
