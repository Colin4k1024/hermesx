import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@shared/api/client'
import type { ApiKeyListResponse, ApiKeyCreateResponse } from '@shared/types'

export function useApiKeys(tenantId: string) {
  return useQuery({
    queryKey: ['admin', 'apikeys', tenantId],
    queryFn: () => apiClient.get<ApiKeyListResponse>(`/admin/v1/tenants/${tenantId}/api-keys`, { asAdmin: true }),
    enabled: !!tenantId,
  })
}

export function useCreateApiKey(tenantId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string; roles?: string[]; scopes?: string[] }) =>
      apiClient.post<ApiKeyCreateResponse>(`/admin/v1/tenants/${tenantId}/api-keys`, data, { asAdmin: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'apikeys', tenantId] }),
  })
}

export function useRotateApiKey(tenantId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (keyId: string) =>
      apiClient.post<ApiKeyCreateResponse>(`/admin/v1/tenants/${tenantId}/api-keys/${keyId}/rotate`, {}, { asAdmin: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'apikeys', tenantId] }),
  })
}

export function useRevokeApiKey(tenantId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (keyId: string) =>
      apiClient.del(`/admin/v1/tenants/${tenantId}/api-keys/${keyId}`, { asAdmin: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'apikeys', tenantId] }),
  })
}
