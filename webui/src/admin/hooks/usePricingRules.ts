import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@shared/api/client'
import type { PricingRuleListResponse, PricingRule } from '@shared/types'

export function usePricingRules() {
  return useQuery({
    queryKey: ['admin', 'pricing-rules'],
    queryFn: () => apiClient.get<PricingRuleListResponse>('/admin/v1/pricing-rules', { asAdmin: true }),
  })
}

export function useUpsertPricingRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (rule: Omit<PricingRule, 'updated_at'>) =>
      apiClient.put(`/admin/v1/pricing-rules/${encodeURIComponent(rule.model_key)}`, rule, { asAdmin: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'pricing-rules'] }),
  })
}

export function useDeletePricingRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (modelKey: string) =>
      apiClient.del(`/admin/v1/pricing-rules/${encodeURIComponent(modelKey)}`, { asAdmin: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'pricing-rules'] }),
  })
}
