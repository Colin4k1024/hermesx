import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@shared/api/client'
import type {
  EffectiveTenantSharingPolicy,
  SharingMode,
  SharingPolicyHistoryResponse,
  SharingPolicySnapshot,
} from '@shared/types'

const governanceKey = ['admin', 'governance']

export interface GlobalSharingPolicyUpdate {
  mode: SharingMode
  reason?: string
}

export interface TenantSharingPolicyUpdate {
  consume_shared: boolean
  contribution_mode: SharingMode
  labels?: string[]
  reason?: string
}

export interface SharedKnowledgeRevokeRequest {
  task_class?: string
  source_tenant?: string
  source?: string
  from?: string
  to?: string
  confirm_all?: boolean
  reason?: string
}

export function useGlobalSharingPolicy() {
  return useQuery({
    queryKey: [...governanceKey, 'global'],
    queryFn: () => apiClient.get<SharingPolicySnapshot>('/admin/v1/evolution/sharing-policy', { asAdmin: true }),
    retry: false,
  })
}

export function useGlobalSharingPolicyHistory() {
  return useQuery({
    queryKey: [...governanceKey, 'global-history'],
    queryFn: () => apiClient.get<SharingPolicyHistoryResponse>('/admin/v1/evolution/sharing-policy/history?limit=50', { asAdmin: true }),
    retry: false,
  })
}

export function useUpdateGlobalSharingPolicy() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: GlobalSharingPolicyUpdate) => apiClient.put<SharingPolicySnapshot>('/admin/v1/evolution/sharing-policy', body, { asAdmin: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: governanceKey }),
  })
}

export function useRollbackGlobalSharingPolicy() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { version: number; reason?: string }) => apiClient.post<SharingPolicySnapshot>('/admin/v1/evolution/sharing-policy/rollback', body, { asAdmin: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: governanceKey }),
  })
}

export function useTenantSharingPolicy(tenantId: string) {
  return useQuery({
    queryKey: [...governanceKey, 'tenant', tenantId],
    queryFn: () => apiClient.get<EffectiveTenantSharingPolicy>(`/admin/v1/evolution/tenants/${encodeURIComponent(tenantId)}/sharing-policy`, { asAdmin: true }),
    enabled: !!tenantId,
    retry: false,
  })
}

export function useTenantSharingPolicyHistory(tenantId: string) {
  return useQuery({
    queryKey: [...governanceKey, 'tenant-history', tenantId],
    queryFn: () => apiClient.get<SharingPolicyHistoryResponse>(`/admin/v1/evolution/tenants/${encodeURIComponent(tenantId)}/sharing-policy/history?limit=50`, { asAdmin: true }),
    enabled: !!tenantId,
    retry: false,
  })
}

export function useUpdateTenantSharingPolicy(tenantId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: TenantSharingPolicyUpdate) =>
      apiClient.put<EffectiveTenantSharingPolicy>(`/admin/v1/evolution/tenants/${encodeURIComponent(tenantId)}/sharing-policy`, body, { asAdmin: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: governanceKey }),
  })
}

export function useRollbackTenantSharingPolicy(tenantId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { version: number; reason?: string }) =>
      apiClient.post<EffectiveTenantSharingPolicy>(`/admin/v1/evolution/tenants/${encodeURIComponent(tenantId)}/sharing-policy/rollback`, body, { asAdmin: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: governanceKey }),
  })
}

export function useRevokeSharedKnowledge() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: SharedKnowledgeRevokeRequest) => apiClient.post<{ deleted: number }>('/admin/v1/evolution/shared-knowledge/revoke', body, { asAdmin: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: governanceKey }),
  })
}
