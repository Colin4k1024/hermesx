export interface ChatMessage {
  role: 'user' | 'assistant' | 'system'
  content: string
  timestamp?: string
}

export interface Session {
  id: string
  title?: string
  started_at: string
  ended_at: string | null
  message_count: number
}

export interface SessionListResponse {
  tenant_id: string
  user_id: string
  sessions: Session[]
  count: number
}

export interface SessionDetailResponse {
  session_id: string
  messages: ChatMessage[]
}

export interface MemoryEntry {
  key: string
  content: string
}

export interface MemoryListResponse {
  tenant_id: string
  user_id: string
  memories: MemoryEntry[]
  count: number
}

export interface SkillItem {
  name: string
  description?: string
  version?: string
  source?: string
  user_modified?: boolean
}

export interface SkillListResponse {
  tenant_id: string
  skills: SkillItem[]
  count: number
}

export interface MeResponse {
  tenant_id: string
  identity: string
  roles: string[]
  auth_method: string
  plan?: string
  rate_limit_rpm?: number
  max_sessions?: number
}

export interface TenantItem {
  id: string
  name: string
  created_at: string
}

export interface TenantListResponse {
  tenants: TenantItem[]
  total: number
}

export interface ApiKeyItem {
  id: string
  name: string
  prefix: string
  tenant_id: string
  roles: string[]
  scopes?: string[]
  expires_at?: string | null
  revoked_at?: string | null
  created_at: string
}

export interface ApiKeyListResponse {
  api_keys: ApiKeyItem[]
  total: number
}

export interface ApiKeyCreateResponse {
  id: string
  key: string
  name: string
  tenant_id: string
  roles: string[]
  created_at: string
}

export interface ChatRequest {
  model: string
  messages: ChatMessage[]
  stream?: boolean
}

export interface ChatResponse {
  id: string
  object: string
  created: number
  model: string
  choices: Array<{
    index: number
    message: ChatMessage
    finish_reason: string
  }>
}

export interface UsageResponse {
  tenant_id: string
  period: string
  input_tokens: number
  output_tokens: number
  total_tokens: number
  estimated_cost_usd: number
}

export interface AuditLogItem {
  id: number
  tenant_id: string
  user_id: string | null
  action: string
  detail: string | null
  request_id: string | null
  status_code: number | null
  created_at: string
}

export interface AuditLogListResponse {
  logs: AuditLogItem[]
  total: number
}

export interface PricingRule {
  model_key: string
  input_per_1k: number
  output_per_1k: number
  cache_read_per_1k: number
  updated_at: string
}

export interface PricingRuleListResponse {
  rules: PricingRule[]
}

export interface SandboxPolicy {
  tenant_id: string
  policy: string
  updated_at?: string
}

export interface BootstrapStatusResponse {
  bootstrap_required: boolean
}

export interface Notification {
  id: string
  type: 'info' | 'warning' | 'success' | 'error'
  title: string
  message: string
  read: boolean
  created_at: string
}
