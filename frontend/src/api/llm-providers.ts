import { isDefined, find, findIndex } from 'remeda'

// --- Types ---

export type ProviderType = 'openai' | 'anthropic' | 'ollama' | 'custom'

export interface LlmProvider {
  id: string
  org_id: string
  workspace_id: string | null // null = org-wide
  provider_type: ProviderType
  display_name: string
  model: string
  base_url: string | null
  api_key_set: boolean // never expose the actual key
  status: 'active' | 'inactive'
  created_at: string
}

export interface CreateLlmProviderRequest {
  workspace_id?: string | null
  provider_type: ProviderType
  display_name: string
  model: string
  base_url?: string | null
  api_key: string // sent on create/update, never returned
}

export interface UpdateLlmProviderRequest {
  display_name?: string
  model?: string
  base_url?: string | null
  api_key?: string // optional on update; omit to keep existing key
  status?: 'active' | 'inactive'
  workspace_id?: string | null
}

export interface TestConnectionResult {
  success: boolean
  message: string
  latency_ms?: number
}

/** Models available per provider type (used for model selection dropdowns). */
export interface ProviderModelOption {
  value: string
  label: string
}

export const PROVIDER_MODELS: Record<ProviderType, ProviderModelOption[]> = {
  openai: [
    { value: 'gpt-4o', label: 'GPT-4o' },
    { value: 'gpt-4o-mini', label: 'GPT-4o Mini' },
    { value: 'gpt-4-turbo', label: 'GPT-4 Turbo' },
    { value: 'gpt-3.5-turbo', label: 'GPT-3.5 Turbo' },
  ],
  anthropic: [
    { value: 'claude-sonnet-4-20250514', label: 'Claude Sonnet 4' },
    { value: 'claude-opus-4-20250514', label: 'Claude Opus 4' },
    { value: 'claude-3-5-haiku-20241022', label: 'Claude 3.5 Haiku' },
  ],
  ollama: [
    { value: 'llama3', label: 'Llama 3' },
    { value: 'mistral', label: 'Mistral' },
    { value: 'codellama', label: 'Code Llama' },
    { value: 'phi3', label: 'Phi-3' },
  ],
  custom: [
    { value: 'custom', label: 'Custom Model' },
  ],
}

// --- Helpers ---

async function authFetch(path: string, init?: RequestInit): Promise<Response> {
  const base = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'
  return fetch(base + path, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })
}

// --- Mock helpers (remove when backend is ready) ---

let _nextId = 4

function mockDelay(ms = 300): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

// TODO: Remove mock data once the backend LLM provider config API exists
const MOCK_PROVIDERS: LlmProvider[] = [
  {
    id: 'llm-1',
    org_id: 'org-456',
    workspace_id: null,
    provider_type: 'openai',
    display_name: 'OpenAI Production',
    model: 'gpt-4o',
    base_url: null,
    api_key_set: true,
    status: 'active',
    created_at: '2026-03-01T10:00:00Z',
  },
  {
    id: 'llm-2',
    org_id: 'org-456',
    workspace_id: 'ws-1',
    provider_type: 'anthropic',
    display_name: 'Anthropic - Engineering',
    model: 'claude-sonnet-4-20250514',
    base_url: null,
    api_key_set: true,
    status: 'active',
    created_at: '2026-03-10T14:30:00Z',
  },
  {
    id: 'llm-3',
    org_id: 'org-456',
    workspace_id: null,
    provider_type: 'ollama',
    display_name: 'Local Ollama',
    model: 'llama3',
    base_url: 'http://localhost:11434',
    api_key_set: false,
    status: 'inactive',
    created_at: '2026-03-15T09:00:00Z',
  },
]

// Keep a mutable copy for mock CRUD
let mockData = [...MOCK_PROVIDERS]

/** Reset mock data (useful for tests). */
export function _resetMockData(): void {
  mockData = [...MOCK_PROVIDERS]
  _nextId = 4
}

// --- API functions ---

/**
 * List all LLM providers for the given org.
 * TODO: Replace mock with `authFetch(\`/orgs/\${orgId}/llm-providers\`)`
 */
export async function listLlmProviders(orgId: string): Promise<LlmProvider[]> {
  // TODO: Uncomment when backend API is ready
  // const res = await authFetch(`/orgs/${orgId}/llm-providers`)
  // if (!res.ok) throw new Error(`listLlmProviders failed: ${res.status}`)
  // return res.json()

  void orgId
  await mockDelay()
  return [...mockData]
}

/**
 * Create a new LLM provider configuration.
 * TODO: Replace mock with `authFetch(\`/orgs/\${orgId}/llm-providers\`, { method: 'POST', ... })`
 */
export async function createLlmProvider(
  orgId: string,
  data: CreateLlmProviderRequest,
): Promise<LlmProvider> {
  // TODO: Uncomment when backend API is ready
  // const res = await authFetch(`/orgs/${orgId}/llm-providers`, {
  //   method: 'POST',
  //   body: JSON.stringify(data),
  // })
  // if (!res.ok) throw new Error(`createLlmProvider failed: ${res.status}`)
  // return res.json()

  await mockDelay()
  const provider: LlmProvider = {
    id: `llm-${_nextId++}`,
    org_id: orgId,
    workspace_id: data.workspace_id ?? null,
    provider_type: data.provider_type,
    display_name: data.display_name,
    model: data.model,
    base_url: data.base_url ?? null,
    api_key_set: !!data.api_key,
    status: 'active',
    created_at: new Date().toISOString(),
  }
  mockData.push(provider)
  return provider
}

/**
 * Update an existing LLM provider configuration.
 * TODO: Replace mock with `authFetch(\`/orgs/\${orgId}/llm-providers/\${providerId}\`, { method: 'PATCH', ... })`
 */
export async function updateLlmProvider(
  orgId: string,
  providerId: string,
  data: UpdateLlmProviderRequest,
): Promise<LlmProvider> {
  // TODO: Uncomment when backend API is ready
  // const res = await authFetch(`/orgs/${orgId}/llm-providers/${providerId}`, {
  //   method: 'PATCH',
  //   body: JSON.stringify(data),
  // })
  // if (!res.ok) throw new Error(`updateLlmProvider failed: ${res.status}`)
  // return res.json()

  void orgId
  await mockDelay()
  const idx = findIndex(mockData, (p) => p.id === providerId)
  if (idx === -1) throw new Error('Provider not found')
  const existing = mockData[idx]
  const updated: LlmProvider = {
    ...existing,
    display_name: data.display_name ?? existing.display_name,
    model: data.model ?? existing.model,
    base_url: isDefined(data.base_url) ? data.base_url : existing.base_url,
    api_key_set: data.api_key ? true : existing.api_key_set,
    status: data.status ?? existing.status,
    workspace_id: isDefined(data.workspace_id) ? (data.workspace_id ?? null) : existing.workspace_id,
  }
  mockData[idx] = updated
  return updated
}

/**
 * Delete an LLM provider configuration.
 * TODO: Replace mock with `authFetch(\`/orgs/\${orgId}/llm-providers/\${providerId}\`, { method: 'DELETE' })`
 */
export async function deleteLlmProvider(
  orgId: string,
  providerId: string,
): Promise<void> {
  // TODO: Uncomment when backend API is ready
  // const res = await authFetch(`/orgs/${orgId}/llm-providers/${providerId}`, {
  //   method: 'DELETE',
  // })
  // if (!res.ok) throw new Error(`deleteLlmProvider failed: ${res.status}`)

  void orgId
  await mockDelay()
  const idx = findIndex(mockData, (p) => p.id === providerId)
  if (idx === -1) throw new Error('Provider not found')
  mockData.splice(idx, 1)
}

/**
 * Test the connection / API key validity for a provider.
 * TODO: Replace mock with `authFetch(\`/orgs/\${orgId}/llm-providers/\${providerId}/test\`, { method: 'POST' })`
 */
export async function testConnection(
  orgId: string,
  providerId: string,
): Promise<TestConnectionResult> {
  // TODO: Uncomment when backend API is ready
  // const res = await authFetch(`/orgs/${orgId}/llm-providers/${providerId}/test`, {
  //   method: 'POST',
  // })
  // if (!res.ok) throw new Error(`testConnection failed: ${res.status}`)
  // return res.json()

  void orgId
  await mockDelay(600)
  const provider = find(mockData, (p) => p.id === providerId)
  if (!provider) throw new Error('Provider not found')

  // Simulate: active providers with api_key_set pass, others fail
  if (provider.api_key_set && provider.status === 'active') {
    return { success: true, message: 'Connection successful', latency_ms: 142 }
  }
  return { success: false, message: 'Connection failed: invalid or missing API key' }
}

// Export authFetch for potential reuse (kept private by convention via underscore)
export { authFetch as _authFetch }
