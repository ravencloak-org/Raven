export interface KnowledgeBase {
  id: string
  org_id: string
  workspace_id: string
  name: string
  slug: string
  settings: Record<string, unknown>
  status: 'active' | 'archived'
  doc_count: number
  /** Whether the semantic response cache is enabled for this KB. See #256. */
  cache_enabled: boolean
  /** Cosine similarity threshold (0.80–0.99) for a cache HIT. */
  cache_similarity_threshold: number
  created_at: string
  updated_at: string
}

/** Cache statistics returned by GET /orgs/:orgId/kbs/:kbId/cache/stats (#256). */
export interface CacheStats {
  total_entries: number
  hit_count: number
  estimated_tokens_saved: number
  expires_soonest: string | null
  avg_hits: number
}

/** Payload for PUT .../knowledge-bases/:kbId — the server merges provided fields. */
export interface UpdateKBPayload {
  name?: string
  description?: string
  settings?: Record<string, unknown>
  cache_enabled?: boolean
  cache_similarity_threshold?: number
}

export interface KBListResponse {
  items: KnowledgeBase[]
  total: number
  offset: number
  limit: number
}

export type KBDocumentStatus = 'pending' | 'processing' | 'completed' | 'failed'

export interface KBDocument {
  id: string
  kb_id: string
  name: string
  file_name: string
  type: string
  file_type: string
  status: KBDocumentStatus
  processing_status: KBDocumentStatus
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

function normalizeDoc(d: Record<string, unknown>): KBDocument {
  return {
    id: (d.id ?? '') as string,
    kb_id: (d.kb_id ?? d.knowledge_base_id ?? '') as string,
    name: (d.name ?? d.file_name ?? '') as string,
    file_name: (d.file_name ?? d.name ?? '') as string,
    type: (d.type ?? d.file_type ?? '') as string,
    file_type: (d.file_type ?? d.type ?? '') as string,
    status: (d.status ?? d.processing_status ?? 'pending') as KBDocumentStatus,
    processing_status: (d.processing_status ?? d.status ?? 'pending') as KBDocumentStatus,
    created_at: (d.created_at ?? '') as string,
  }
}

export async function getDocuments(
  orgId: string,
  wsId: string,
  kbId: string,
): Promise<KBDocument[]> {
  const data = await authFetch<Record<string, unknown>[] | { items: Record<string, unknown>[] }>(
    `${kbBasePath(orgId, wsId)}/${kbId}/documents`,
  )
  const items = Array.isArray(data) ? data : (data.items ?? [])
  return items.map(normalizeDoc)
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
  const raw = await res.json()
  return normalizeDoc(raw as Record<string, unknown>)
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

/**
 * Partially update a knowledge base. Used by the KB detail page to toggle the
 * cache or change the similarity threshold (#256).
 */
export async function updateKnowledgeBase(
  orgId: string,
  wsId: string,
  kbId: string,
  payload: UpdateKBPayload,
): Promise<KnowledgeBase> {
  return authFetch<KnowledgeBase>(`${kbBasePath(orgId, wsId)}/${kbId}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  })
}

/** Fetch semantic-cache stats for a KB. */
export async function getCacheStats(
  orgId: string,
  kbId: string,
): Promise<CacheStats> {
  return authFetch<CacheStats>(`/orgs/${orgId}/kbs/${kbId}/cache/stats`)
}

/** Flush every cached answer for the given KB. */
export async function flushCache(
  orgId: string,
  kbId: string,
): Promise<{ deleted: number }> {
  return authFetch<{ deleted: number }>(`/orgs/${orgId}/kbs/${kbId}/cache`, {
    method: 'DELETE',
  })
}
