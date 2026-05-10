import { useQuery } from '@tanstack/react-query'
import { apiClient } from '@shared/api/client'
import type { AuditLogListResponse } from '@shared/types'

interface AuditLogParams {
  limit?: number
  offset?: number
  action?: string
  tenant_id?: string
  from?: string
  to?: string
}

export function useAuditLogs(params: AuditLogParams = {}) {
  const searchParams = new URLSearchParams()
  if (params.limit) searchParams.set('limit', String(params.limit))
  if (params.offset) searchParams.set('offset', String(params.offset))
  if (params.action) searchParams.set('action', params.action)
  if (params.tenant_id) searchParams.set('tenant_id', params.tenant_id)
  if (params.from) searchParams.set('from', params.from)
  if (params.to) searchParams.set('to', params.to)

  const qs = searchParams.toString()
  return useQuery({
    queryKey: ['admin', 'audit-logs', params],
    queryFn: () => apiClient.get<AuditLogListResponse>(`/admin/v1/audit-logs${qs ? '?' + qs : ''}`, { asAdmin: true }),
  })
}
