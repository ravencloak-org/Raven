import { useAuthStore } from '../stores/auth'

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

async function authFetch(path: string, init?: RequestInit): Promise<Response> {
  const auth = useAuthStore()
  const base = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'
  return fetch(base + path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${auth.accessToken ?? ''}`,
      ...init?.headers,
    },
  })
}

function kbBasePath(orgId: string, wsId: string): string {
  return `/orgs/${orgId}/workspaces/${wsId}/knowledge-bases`
}

// --- Mock data ---

const MOCK_KBS: KnowledgeBase[] = [
  {
    id: 'kb-1', org_id: 'org-456', workspace_id: 'ws-1', name: 'Product Docs',
    slug: 'product-docs', settings: {}, status: 'active', doc_count: 12,
    created_at: '2026-03-01T10:00:00Z', updated_at: '2026-03-20T10:00:00Z',
  },
  {
    id: 'kb-2', org_id: 'org-456', workspace_id: 'ws-1', name: 'FAQ',
    slug: 'faq', settings: {}, status: 'active', doc_count: 5,
    created_at: '2026-03-05T10:00:00Z', updated_at: '2026-03-18T10:00:00Z',
  },
]

const MOCK_DOCS: KBDocument[] = [
  { id: 'doc-1', kb_id: 'kb-1', name: 'getting-started.pdf', type: 'file', status: 'completed', created_at: '2026-03-10T10:00:00Z' },
  { id: 'doc-2', kb_id: 'kb-1', name: 'api-reference.md', type: 'file', status: 'processing', created_at: '2026-03-20T10:00:00Z' },
]

const MOCK_SOURCES: KBSource[] = [
  { id: 'src-1', kb_id: 'kb-1', url: 'https://docs.example.com', status: 'completed', created_at: '2026-03-12T10:00:00Z' },
]

let _nextKbId = 3
let _nextDocId = 3
let _nextSrcId = 2

const mockDelay = (ms = 200) => new Promise((r) => setTimeout(r, ms))

// --- API functions ---

export async function listKnowledgeBases(
  _orgId: string,
  _wsId: string,
  offset = 0,
  limit = 20,
): Promise<KBListResponse> {
  // TODO: const res = await authFetch(`${kbBasePath(orgId, wsId)}?offset=${offset}&limit=${limit}`)
  void authFetch; void kbBasePath
  await mockDelay()
  const items = MOCK_KBS.slice(offset, offset + limit)
  return { items, total: MOCK_KBS.length, offset, limit }
}

export async function getKnowledgeBase(
  _orgId: string,
  _wsId: string,
  kbId: string,
): Promise<KnowledgeBase> {
  // TODO: const res = await authFetch(`${kbBasePath(orgId, wsId)}/${kbId}`)
  await mockDelay()
  const kb = MOCK_KBS.find((k) => k.id === kbId)
  if (!kb) throw new Error('Knowledge base not found')
  return { ...kb }
}

export async function createKnowledgeBase(
  _orgId: string,
  _wsId: string,
  data: { name: string; settings?: Record<string, unknown> },
): Promise<KnowledgeBase> {
  // TODO: const res = await authFetch(kbBasePath(orgId, wsId), { method: 'POST', body: JSON.stringify(data) })
  await mockDelay()
  const kb: KnowledgeBase = {
    id: `kb-${_nextKbId++}`,
    org_id: _orgId,
    workspace_id: _wsId,
    name: data.name,
    slug: data.name.toLowerCase().replace(/\s+/g, '-'),
    settings: data.settings ?? {},
    status: 'active',
    doc_count: 0,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  }
  MOCK_KBS.push(kb)
  return kb
}

export async function archiveKnowledgeBase(
  _orgId: string,
  _wsId: string,
  kbId: string,
): Promise<void> {
  // TODO: await authFetch(`${kbBasePath(orgId, wsId)}/${kbId}`, { method: 'DELETE' })
  await mockDelay()
  const idx = MOCK_KBS.findIndex((k) => k.id === kbId)
  if (idx !== -1) MOCK_KBS[idx].status = 'archived'
}

export async function getDocuments(
  _orgId: string,
  _wsId: string,
  kbId: string,
): Promise<KBDocument[]> {
  // TODO: await authFetch(`${kbBasePath(orgId, wsId)}/${kbId}/documents`)
  await mockDelay()
  return MOCK_DOCS.filter((d) => d.kb_id === kbId)
}

export async function addDocument(
  _orgId: string,
  _wsId: string,
  kbId: string,
  file: File,
): Promise<KBDocument> {
  // TODO: FormData upload to real endpoint
  await mockDelay(500)
  const doc: KBDocument = {
    id: `doc-${_nextDocId++}`,
    kb_id: kbId,
    name: file.name,
    type: 'file',
    status: 'pending',
    created_at: new Date().toISOString(),
  }
  MOCK_DOCS.push(doc)
  return doc
}

export async function getSources(
  _orgId: string,
  _wsId: string,
  kbId: string,
): Promise<KBSource[]> {
  // TODO: await authFetch(`${kbBasePath(orgId, wsId)}/${kbId}/sources`)
  await mockDelay()
  return MOCK_SOURCES.filter((s) => s.kb_id === kbId)
}

export async function addSource(
  _orgId: string,
  _wsId: string,
  kbId: string,
  url: string,
): Promise<KBSource> {
  // TODO: await authFetch(`${kbBasePath(orgId, wsId)}/${kbId}/sources`, { method: 'POST', body: JSON.stringify({ url }) })
  await mockDelay()
  const src: KBSource = {
    id: `src-${_nextSrcId++}`,
    kb_id: kbId,
    url,
    status: 'pending',
    created_at: new Date().toISOString(),
  }
  MOCK_SOURCES.push(src)
  return src
}
