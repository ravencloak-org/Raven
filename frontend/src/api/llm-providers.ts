export type ProviderType = 'openai' | 'anthropic' | 'ollama' | 'custom'

export interface LlmProvider {
  id: string
  org_id: string
  provider: ProviderType
  display_name: string
  base_url: string | null
  api_key_hint: string | null
  is_default: boolean
  status: 'active' | 'inactive'
  config: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateLlmProviderRequest {
  provider: ProviderType
  display_name: string
  api_key: string
  base_url?: string | null
  is_default?: boolean
  config?: Record<string, unknown>
}

export interface UpdateLlmProviderRequest {
  display_name?: string
  api_key?: string
  base_url?: string | null
  is_default?: boolean
  status?: 'active' | 'inactive'
  config?: Record<string, unknown>
}

export interface ProviderModelOption {
  value: string
  label: string
}

export const PROVIDER_MODELS: Record<ProviderType, ProviderModelOption[]> = {
  openai: [
    { value: 'gpt-4o', label: 'GPT-4o' },
    { value: 'gpt-4o-mini', label: 'GPT-4o Mini' },
  ],
  anthropic: [
    { value: 'claude-sonnet-4-20250514', label: 'Claude Sonnet 4' },
    { value: 'claude-opus-4-20250514', label: 'Claude Opus 4' },
    { value: 'claude-3-5-haiku-20241022', label: 'Claude 3.5 Haiku' },
  ],
  ollama: [
    { value: 'llama3', label: 'Llama 3' },
    { value: 'mistral', label: 'Mistral' },
  ],
  custom: [
    { value: 'custom', label: 'Custom Model' },
  ],
}

const API_BASE = () => import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

async function authFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(API_BASE() + path, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.message || data.error || `Request failed (${res.status})`)
  }
  return res.json()
}

export async function listLlmProviders(orgId: string): Promise<LlmProvider[]> {
  const data = await authFetch<LlmProvider[] | { items: LlmProvider[] }>(
    `/orgs/${orgId}/llm-providers`,
  )
  return Array.isArray(data) ? data : (data.items ?? [])
}

export async function createLlmProvider(
  orgId: string,
  data: CreateLlmProviderRequest,
): Promise<LlmProvider> {
  return authFetch<LlmProvider>(`/orgs/${orgId}/llm-providers`, {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export async function updateLlmProvider(
  orgId: string,
  providerId: string,
  data: UpdateLlmProviderRequest,
): Promise<LlmProvider> {
  return authFetch<LlmProvider>(`/orgs/${orgId}/llm-providers/${providerId}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}

export async function deleteLlmProvider(
  orgId: string,
  providerId: string,
): Promise<void> {
  await authFetch<void>(`/orgs/${orgId}/llm-providers/${providerId}`, {
    method: 'DELETE',
  })
}

export async function setDefaultProvider(
  orgId: string,
  providerId: string,
): Promise<LlmProvider> {
  return authFetch<LlmProvider>(`/orgs/${orgId}/llm-providers/${providerId}/default`, {
    method: 'PUT',
  })
}

export { authFetch as _authFetch }
