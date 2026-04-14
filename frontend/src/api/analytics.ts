// --- Interfaces ---

export interface ConversationVolumePoint {
  date: string
  count: number
}

export interface TopQuery {
  query: string
  count: number
  lastAsked: string
}

export interface SourceHit {
  sourceId: string
  sourceName: string
  hitCount: number
  lastAccessed: string
}

export interface UnansweredQuestion {
  id: string
  question: string
  askedAt: string
  userId: string
}

export type DateRange = '7d' | '30d' | '90d'

// --- Auth helper (follows orgs.ts pattern) ---

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

// --- Mock data generators ---

function generateMockVolumeData(range: DateRange): ConversationVolumePoint[] {
  const days = range === '7d' ? 7 : range === '30d' ? 30 : 90
  const points: ConversationVolumePoint[] = []
  const now = new Date()
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(now)
    d.setDate(d.getDate() - i)
    points.push({
      date: d.toISOString().slice(0, 10),
      count: Math.floor(Math.random() * 80) + 5,
    })
  }
  return points
}

const MOCK_TOP_QUERIES: TopQuery[] = [
  { query: 'How do I reset my password?', count: 142, lastAsked: '2026-03-28T10:15:00Z' },
  { query: 'What is the refund policy?', count: 98, lastAsked: '2026-03-28T09:30:00Z' },
  { query: 'How to integrate the API?', count: 87, lastAsked: '2026-03-27T16:45:00Z' },
  { query: 'Where can I find the docs?', count: 73, lastAsked: '2026-03-27T14:20:00Z' },
  { query: 'Pricing plans comparison', count: 65, lastAsked: '2026-03-28T08:00:00Z' },
  { query: 'How to export data?', count: 52, lastAsked: '2026-03-26T11:10:00Z' },
  { query: 'SSO setup instructions', count: 48, lastAsked: '2026-03-27T13:00:00Z' },
  { query: 'Supported file formats', count: 41, lastAsked: '2026-03-25T17:30:00Z' },
  { query: 'Rate limiting details', count: 34, lastAsked: '2026-03-26T09:45:00Z' },
  { query: 'Webhook configuration', count: 29, lastAsked: '2026-03-28T07:15:00Z' },
]

const MOCK_SOURCE_HITS: SourceHit[] = [
  { sourceId: 'src-1', sourceName: 'FAQ Knowledge Base', hitCount: 312, lastAccessed: '2026-03-28T10:20:00Z' },
  { sourceId: 'src-2', sourceName: 'API Documentation', hitCount: 245, lastAccessed: '2026-03-28T09:50:00Z' },
  { sourceId: 'src-3', sourceName: 'User Guide', hitCount: 198, lastAccessed: '2026-03-27T16:30:00Z' },
  { sourceId: 'src-4', sourceName: 'Release Notes', hitCount: 87, lastAccessed: '2026-03-26T14:10:00Z' },
  { sourceId: 'src-5', sourceName: 'Internal Wiki', hitCount: 63, lastAccessed: '2026-03-27T11:00:00Z' },
  { sourceId: 'src-6', sourceName: 'Troubleshooting Guides', hitCount: 54, lastAccessed: '2026-03-28T08:30:00Z' },
]

const MOCK_UNANSWERED: UnansweredQuestion[] = [
  { id: 'uq-1', question: 'Can I use the platform with on-premise LDAP?', askedAt: '2026-03-28T09:12:00Z', userId: 'user-42' },
  { id: 'uq-2', question: 'Is there a GraphQL endpoint available?', askedAt: '2026-03-28T08:45:00Z', userId: 'user-17' },
  { id: 'uq-3', question: 'How to set up custom email templates?', askedAt: '2026-03-27T15:30:00Z', userId: 'user-88' },
  { id: 'uq-4', question: 'What is the maximum file upload size?', askedAt: '2026-03-27T14:00:00Z', userId: 'user-23' },
  { id: 'uq-5', question: 'Do you support multi-region deployments?', askedAt: '2026-03-26T10:20:00Z', userId: 'user-55' },
]

// --- API functions ---

export async function getConversationVolume(range: DateRange): Promise<ConversationVolumePoint[]> {
  // TODO: Replace mock data with real API call
  // const res = await authFetch(`/analytics/conversation-volume?range=${range}`)
  // if (!res.ok) throw new Error(`getConversationVolume failed: ${res.status}`)
  // return res.json()

  void authFetch // reference to avoid unused-import lint errors
  return Promise.resolve(generateMockVolumeData(range))
}

export async function getTopQueries(range: DateRange): Promise<TopQuery[]> {
  // TODO: Replace mock data with real API call
  // const res = await authFetch(`/analytics/top-queries?range=${range}`)
  // if (!res.ok) throw new Error(`getTopQueries failed: ${res.status}`)
  // return res.json()

  void range
  return Promise.resolve(MOCK_TOP_QUERIES)
}

export async function getSourceHits(range: DateRange): Promise<SourceHit[]> {
  // TODO: Replace mock data with real API call
  // const res = await authFetch(`/analytics/source-hits?range=${range}`)
  // if (!res.ok) throw new Error(`getSourceHits failed: ${res.status}`)
  // return res.json()

  void range
  return Promise.resolve(MOCK_SOURCE_HITS)
}

export async function getUnansweredQuestions(range: DateRange): Promise<UnansweredQuestion[]> {
  // TODO: Replace mock data with real API call
  // const res = await authFetch(`/analytics/unanswered?range=${range}`)
  // if (!res.ok) throw new Error(`getUnansweredQuestions failed: ${res.status}`)
  // return res.json()

  void range
  return Promise.resolve(MOCK_UNANSWERED)
}
