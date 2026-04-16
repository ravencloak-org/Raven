export interface KnowledgeBase {
  id: string
  org_id: string
  workspace_id: string
  name: string
  slug: string
  settings: Record<string, unknown>
  status: 'active' | 'archived'
  doc_count: number
  created_at: string
  updated_at: string
}

export interface KBListResponse {
  items: KnowledgeBase[]
  total: number
  offset: number
  limit: number
}

export interface KBDocument {
  id: string
  kb_id: string
  name: string
  type: 'file' | 'url'
  status: 'pending' | 'processing' | 'completed' | 'failed'
  created_at: string
}

export interface KBSource {
  id: string
  kb_id: string
  url: string
  status: 'pending' | 'processing' | 'completed' | 'failed'
  created_at: string
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

function kbBasePath(orgId: string, wsId: string): string {
  return `/orgs/${orgId}/workspaces/${wsId}/knowledge-bases`
}

export async function listKnowledgeBases(
  orgId: string,
  wsId: string,
  offset = 0,
  limit = 20,
): Promise<KBListResponse> {
  const data = await authFetch<KnowledgeBase[] | KBListResponse>(
    `${kbBasePath(orgId, wsId)}?offset=${offset}&limit=${limit}`,
  )
  if (Array.isArray(data)) {
    return { items: data, total: data.length, offset, limit }
  }
  return data
}

export async function getKnowledgeBase(
  orgId: string,
  wsId: string,
  kbId: string,
): Promise<KnowledgeBase> {
  return authFetch<KnowledgeBase>(`${kbBasePath(orgId, wsId)}/${kbId}`)
}

export async function createKnowledgeBase(
  orgId: string,
  wsId: string,
  data: { name: string; settings?: Record<string, unknown> },
): Promise<KnowledgeBase> {
  return authFetch<KnowledgeBase>(kbBasePath(orgId, wsId), {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export async function archiveKnowledgeBase(
  orgId: string,
  wsId: string,
  kbId: string,
): Promise<void> {
  await authFetch<void>(`${kbBasePath(orgId, wsId)}/${kbId}`, { method: 'DELETE' })
}

export async function getDocuments(
  orgId: string,
  wsId: string,
  kbId: string,
): Promise<KBDocument[]> {
  const data = await authFetch<KBDocument[] | { items: KBDocument[] }>(
    `${kbBasePath(orgId, wsId)}/${kbId}/documents`,
  )
  return Array.isArray(data) ? data : (data.items ?? [])
}

export async function addDocument(
  orgId: string,
  wsId: string,
  kbId: string,
  file: File,
): Promise<KBDocument> {
  const formData = new FormData()
  formData.append('file', file)
  const res = await fetch(
    `${API_BASE()}${kbBasePath(orgId, wsId)}/${kbId}/documents/upload`,
    { method: 'POST', credentials: 'include', body: formData },
  )
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.message || data.error || `Upload failed (${res.status})`)
  }
  return res.json()
}

export async function getSources(
  orgId: string,
  wsId: string,
  kbId: string,
): Promise<KBSource[]> {
  const data = await authFetch<KBSource[] | { items: KBSource[] }>(
    `${kbBasePath(orgId, wsId)}/${kbId}/sources`,
  )
  return Array.isArray(data) ? data : (data.items ?? [])
}

export async function addSource(
  orgId: string,
  wsId: string,
  kbId: string,
  url: string,
): Promise<KBSource> {
  return authFetch<KBSource>(`${kbBasePath(orgId, wsId)}/${kbId}/sources`, {
    method: 'POST',
    body: JSON.stringify({ url }),
  })
}
