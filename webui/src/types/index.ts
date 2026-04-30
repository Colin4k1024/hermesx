export interface ChatMessage {
  role: 'user' | 'assistant' | 'system'
  content: string
  timestamp?: string
}

export interface Session {
  id: string
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
  stream?: false
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
