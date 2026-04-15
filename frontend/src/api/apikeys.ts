import { isDefined, find } from 'remeda'

export interface ApiKey {
  id: string
  name: string
  key_prefix: string
  org_id: string
  workspace_id: string
  allowed_domains: string[]
  rate_limit: number
  status: 'active' | 'revoked'
  created_at: string
}

export interface CreateApiKeyRequest {
  name: string
  allowed_domains: string[]
  rate_limit: number
}

export interface CreateApiKeyResponse {
  api_key: ApiKey
  /** The full key value, only returned once at creation time */
  raw_key: string
}

export interface UpdateApiKeySettingsRequest {
  allowed_domains?: string[]
  rate_limit?: number
}

// TODO: Replace with real API base URL when backend endpoints exist
const API_BASE = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

async function authFetch(path: string, init?: RequestInit): Promise<Response> {
  return fetch(API_BASE + path, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })
}

// TODO: Remove mock data once backend API key endpoints are implemented
const mockKeys: ApiKey[] = [
  {
    id: 'key-001',
    name: 'Production Widget',
    key_prefix: 'rk_prod_',
    org_id: 'org-456',
    workspace_id: 'ws-001',
    allowed_domains: ['example.com', 'app.example.com'],
    rate_limit: 1000,
    status: 'active',
    created_at: '2026-03-20T10:30:00Z',
  },
  {
    id: 'key-002',
    name: 'Staging Widget',
    key_prefix: 'rk_stag_',
    org_id: 'org-456',
    workspace_id: 'ws-001',
    allowed_domains: ['staging.example.com'],
    rate_limit: 500,
    status: 'active',
    created_at: '2026-03-22T14:15:00Z',
  },
  {
    id: 'key-003',
    name: 'Old Key',
    key_prefix: 'rk_old__',
    org_id: 'org-456',
    workspace_id: 'ws-001',
    allowed_domains: [],
    rate_limit: 100,
    status: 'revoked',
    created_at: '2026-01-10T08:00:00Z',
  },
]

let mockIdCounter = 4

export async function listApiKeys(): Promise<ApiKey[]> {
  // TODO: Replace with real API call:
  // const res = await authFetch('/api-keys')
  // if (!res.ok) throw new Error(`listApiKeys failed: ${res.status}`)
  // return res.json()
  void authFetch // silence unused lint warning until real calls are wired
  return Promise.resolve([...mockKeys])
}

export async function createApiKey(req: CreateApiKeyRequest): Promise<CreateApiKeyResponse> {
  // TODO: Replace with real API call:
  // const res = await authFetch('/api-keys', {
  //   method: 'POST',
  //   body: JSON.stringify(req),
  // })
  // if (!res.ok) throw new Error(`createApiKey failed: ${res.status}`)
  // return res.json()
  const id = `key-${String(mockIdCounter++).padStart(3, '0')}`
  const rawKey = `rk_live_${crypto.randomUUID().replace(/-/g, '')}`
  const newKey: ApiKey = {
    id,
    name: req.name,
    key_prefix: rawKey.slice(0, 8),
    org_id: 'org-456',
    workspace_id: 'ws-001',
    allowed_domains: req.allowed_domains,
    rate_limit: req.rate_limit,
    status: 'active',
    created_at: new Date().toISOString(),
  }
  mockKeys.push(newKey)
  return Promise.resolve({ api_key: newKey, raw_key: rawKey })
}

export async function revokeApiKey(keyId: string): Promise<ApiKey> {
  // TODO: Replace with real API call:
  // const res = await authFetch(`/api-keys/${keyId}/revoke`, { method: 'POST' })
  // if (!res.ok) throw new Error(`revokeApiKey failed: ${res.status}`)
  // return res.json()
  const key = find(mockKeys, (k) => k.id === keyId)
  if (!key) throw new Error(`API key not found: ${keyId}`)
  key.status = 'revoked'
  return Promise.resolve({ ...key })
}

export async function updateApiKeySettings(
  keyId: string,
  settings: UpdateApiKeySettingsRequest,
): Promise<ApiKey> {
  // TODO: Replace with real API call:
  // const res = await authFetch(`/api-keys/${keyId}/settings`, {
  //   method: 'PATCH',
  //   body: JSON.stringify(settings),
  // })
  // if (!res.ok) throw new Error(`updateApiKeySettings failed: ${res.status}`)
  // return res.json()
  const key = find(mockKeys, (k) => k.id === keyId)
  if (!key) throw new Error(`API key not found: ${keyId}`)
  if (isDefined(settings.allowed_domains)) key.allowed_domains = settings.allowed_domains
  if (isDefined(settings.rate_limit)) key.rate_limit = settings.rate_limit
  return Promise.resolve({ ...key })
}
