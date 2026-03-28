import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAnalyticsStore } from './analytics'
import * as analyticsApi from '../api/analytics'

vi.mock('../api/analytics')

const mockVolume: analyticsApi.ConversationVolumePoint[] = [
  { date: '2026-03-27', count: 42 },
  { date: '2026-03-28', count: 58 },
]

const mockQueries: analyticsApi.TopQuery[] = [
  { query: 'How do I reset my password?', count: 142, lastAsked: '2026-03-28T10:15:00Z' },
  { query: 'What is the refund policy?', count: 98, lastAsked: '2026-03-28T09:30:00Z' },
]

const mockSourceHits: analyticsApi.SourceHit[] = [
  { sourceId: 'src-1', sourceName: 'FAQ Knowledge Base', hitCount: 312, lastAccessed: '2026-03-28T10:20:00Z' },
]

const mockUnanswered: analyticsApi.UnansweredQuestion[] = [
  { id: 'uq-1', question: 'Can I use LDAP?', askedAt: '2026-03-28T09:12:00Z', userId: 'user-42' },
]

describe('useAnalyticsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(analyticsApi.getConversationVolume).mockResolvedValue(mockVolume)
    vi.mocked(analyticsApi.getTopQueries).mockResolvedValue(mockQueries)
    vi.mocked(analyticsApi.getSourceHits).mockResolvedValue(mockSourceHits)
    vi.mocked(analyticsApi.getUnansweredQuestions).mockResolvedValue(mockUnanswered)
  })

  it('initialises with empty state', () => {
    const store = useAnalyticsStore()
    expect(store.conversationVolume).toEqual([])
    expect(store.topQueries).toEqual([])
    expect(store.sourceHits).toEqual([])
    expect(store.unansweredQuestions).toEqual([])
    expect(store.selectedRange).toBe('30d')
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('fetchAll populates all data', async () => {
    const store = useAnalyticsStore()
    await store.fetchAll()

    expect(store.conversationVolume).toEqual(mockVolume)
    expect(store.topQueries).toEqual(mockQueries)
    expect(store.sourceHits).toEqual(mockSourceHits)
    expect(store.unansweredQuestions).toEqual(mockUnanswered)
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('fetchAll calls API with current selected range', async () => {
    const store = useAnalyticsStore()
    await store.fetchAll()

    expect(analyticsApi.getConversationVolume).toHaveBeenCalledWith('30d')
    expect(analyticsApi.getTopQueries).toHaveBeenCalledWith('30d')
    expect(analyticsApi.getSourceHits).toHaveBeenCalledWith('30d')
    expect(analyticsApi.getUnansweredQuestions).toHaveBeenCalledWith('30d')
  })

  it('fetchAll with explicit range updates selectedRange', async () => {
    const store = useAnalyticsStore()
    await store.fetchAll('7d')

    expect(store.selectedRange).toBe('7d')
    expect(analyticsApi.getConversationVolume).toHaveBeenCalledWith('7d')
  })

  it('changeRange updates range and fetches data', async () => {
    const store = useAnalyticsStore()
    await store.changeRange('90d')

    expect(store.selectedRange).toBe('90d')
    expect(analyticsApi.getConversationVolume).toHaveBeenCalledWith('90d')
    expect(store.conversationVolume).toEqual(mockVolume)
  })

  it('sets error on API failure', async () => {
    vi.mocked(analyticsApi.getConversationVolume).mockRejectedValue(new Error('Network error'))
    const store = useAnalyticsStore()
    await store.fetchAll()

    expect(store.error).toBe('Network error')
    expect(store.loading).toBe(false)
  })

  it('sets loading during fetch', async () => {
    let resolvePromise: (v: analyticsApi.ConversationVolumePoint[]) => void
    vi.mocked(analyticsApi.getConversationVolume).mockReturnValue(
      new Promise((resolve) => {
        resolvePromise = resolve
      }),
    )

    const store = useAnalyticsStore()
    const fetchPromise = store.fetchAll()

    expect(store.loading).toBe(true)

    resolvePromise!(mockVolume)
    await fetchPromise

    expect(store.loading).toBe(false)
  })

  it('clears previous error on successful fetch', async () => {
    const store = useAnalyticsStore()

    vi.mocked(analyticsApi.getConversationVolume).mockRejectedValueOnce(new Error('fail'))
    await store.fetchAll()
    expect(store.error).toBe('fail')

    vi.mocked(analyticsApi.getConversationVolume).mockResolvedValueOnce(mockVolume)
    await store.fetchAll()
    expect(store.error).toBeNull()
  })
})
