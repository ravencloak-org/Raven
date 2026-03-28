import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useKnowledgeBasesStore } from './knowledge-bases'
import type {
  KnowledgeBase,
  KBDocument,
  KBSource,
} from '../api/knowledge-bases'

vi.mock('../api/knowledge-bases', () => ({
  listKnowledgeBases: vi.fn(),
  getKnowledgeBase: vi.fn(),
  createKnowledgeBase: vi.fn(),

  archiveKnowledgeBase: vi.fn(),
  getDocuments: vi.fn(),
  addDocument: vi.fn(),
  getSources: vi.fn(),
  addSource: vi.fn(),
}))

import {
  listKnowledgeBases,
  getKnowledgeBase,
  createKnowledgeBase,
  archiveKnowledgeBase,
  getDocuments,
  addDocument,
  getSources,
  addSource,
} from '../api/knowledge-bases'

const mockedListKnowledgeBases = vi.mocked(listKnowledgeBases)
const mockedGetKnowledgeBase = vi.mocked(getKnowledgeBase)
const mockedCreateKnowledgeBase = vi.mocked(createKnowledgeBase)
const mockedArchiveKnowledgeBase = vi.mocked(archiveKnowledgeBase)
const mockedGetDocuments = vi.mocked(getDocuments)
const mockedAddDocument = vi.mocked(addDocument)
const mockedGetSources = vi.mocked(getSources)
const mockedAddSource = vi.mocked(addSource)

const ORG_ID = 'org-1'
const WS_ID = 'ws-1'

function fakeKB(overrides: Partial<KnowledgeBase> = {}): KnowledgeBase {
  return {
    id: 'kb-1',
    org_id: ORG_ID,
    workspace_id: WS_ID,
    name: 'Test KB',
    slug: 'test-kb',
    settings: {},
    status: 'active',
    doc_count: 5,
    created_at: '2026-03-15T10:00:00Z',
    updated_at: '2026-03-20T14:30:00Z',
    ...overrides,
  }
}

function fakeDocument(overrides: Partial<KBDocument> = {}): KBDocument {
  return {
    id: 'doc-1',
    kb_id: 'kb-1',
    name: 'test.pdf',
    type: 'file',
    status: 'completed',
    created_at: '2026-03-16T10:00:00Z',
    ...overrides,
  }
}

function fakeSource(overrides: Partial<KBSource> = {}): KBSource {
  return {
    id: 'src-1',
    kb_id: 'kb-1',
    url: 'https://docs.example.com',
    status: 'completed',
    created_at: '2026-03-17T10:00:00Z',
    ...overrides,
  }
}

describe('useKnowledgeBasesStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('fetchKnowledgeBases', () => {
    it('populates knowledgeBases and total on success', async () => {
      const kb = fakeKB()
      const response: { items: KnowledgeBase[]; total: number; offset: number; limit: number } = {
        items: [kb],
        total: 1,
        offset: 0,
        limit: 20,
      }
      mockedListKnowledgeBases.mockResolvedValue(response)

      const store = useKnowledgeBasesStore()
      await store.fetchKnowledgeBases(ORG_ID, WS_ID)

      expect(mockedListKnowledgeBases).toHaveBeenCalledWith(ORG_ID, WS_ID, 0, 20)
      expect(store.knowledgeBases).toEqual([kb])
      expect(store.total).toBe(1)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('sets error on failure', async () => {
      mockedListKnowledgeBases.mockRejectedValue(
        new Error('listKnowledgeBases failed: 500'),
      )

      const store = useKnowledgeBasesStore()
      await store.fetchKnowledgeBases(ORG_ID, WS_ID)

      expect(store.knowledgeBases).toEqual([])
      expect(store.error).toBe('listKnowledgeBases failed: 500')
      expect(store.loading).toBe(false)
    })

    it('passes offset and limit through', async () => {
      mockedListKnowledgeBases.mockResolvedValue({
        items: [],
        total: 0,
        offset: 10,
        limit: 5,
      })

      const store = useKnowledgeBasesStore()
      await store.fetchKnowledgeBases(ORG_ID, WS_ID, 10, 5)

      expect(mockedListKnowledgeBases).toHaveBeenCalledWith(ORG_ID, WS_ID, 10, 5)
    })
  })

  describe('fetchKnowledgeBase', () => {
    it('sets currentKB on success', async () => {
      const kb = fakeKB()
      mockedGetKnowledgeBase.mockResolvedValue(kb)

      const store = useKnowledgeBasesStore()
      await store.fetchKnowledgeBase(ORG_ID, WS_ID, 'kb-1')

      expect(mockedGetKnowledgeBase).toHaveBeenCalledWith(ORG_ID, WS_ID, 'kb-1')
      expect(store.currentKB).toEqual(kb)
      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      mockedGetKnowledgeBase.mockRejectedValue(new Error('getKnowledgeBase failed: 404'))

      const store = useKnowledgeBasesStore()
      await store.fetchKnowledgeBase(ORG_ID, WS_ID, 'kb-1')

      expect(store.currentKB).toBeNull()
      expect(store.error).toBe('getKnowledgeBase failed: 404')
    })
  })

  describe('create', () => {
    it('appends new KB and increments total', async () => {
      const kb = fakeKB({ id: 'kb-new', name: 'New KB' })
      mockedCreateKnowledgeBase.mockResolvedValue(kb)

      const store = useKnowledgeBasesStore()
      store.total = 3

      const result = await store.create(ORG_ID, WS_ID, { name: 'New KB' })

      expect(mockedCreateKnowledgeBase).toHaveBeenCalledWith(ORG_ID, WS_ID, { name: 'New KB' })
      expect(result).toEqual(kb)
      expect(store.knowledgeBases).toContainEqual(kb)
      expect(store.total).toBe(4)
    })

    it('propagates API errors', async () => {
      mockedCreateKnowledgeBase.mockRejectedValue(
        new Error('createKnowledgeBase failed: 400'),
      )

      const store = useKnowledgeBasesStore()

      await expect(store.create(ORG_ID, WS_ID, { name: '' })).rejects.toThrow(
        'createKnowledgeBase failed: 400',
      )
    })
  })

  describe('archive', () => {
    it('sets KB status to archived in list and currentKB', async () => {
      mockedArchiveKnowledgeBase.mockResolvedValue(undefined)

      const store = useKnowledgeBasesStore()
      const kb = fakeKB()
      store.knowledgeBases = [kb]
      store.currentKB = kb

      await store.archive(ORG_ID, WS_ID, 'kb-1')

      expect(mockedArchiveKnowledgeBase).toHaveBeenCalledWith(ORG_ID, WS_ID, 'kb-1')
      expect(store.knowledgeBases[0].status).toBe('archived')
      expect(store.currentKB?.status).toBe('archived')
    })

    it('propagates API errors', async () => {
      mockedArchiveKnowledgeBase.mockRejectedValue(
        new Error('archiveKnowledgeBase failed: 403'),
      )

      const store = useKnowledgeBasesStore()

      await expect(store.archive(ORG_ID, WS_ID, 'kb-1')).rejects.toThrow(
        'archiveKnowledgeBase failed: 403',
      )
    })
  })

  describe('fetchDocuments', () => {
    it('populates documents on success', async () => {
      const doc = fakeDocument()
      mockedGetDocuments.mockResolvedValue([doc])

      const store = useKnowledgeBasesStore()
      await store.fetchDocuments(ORG_ID, WS_ID, 'kb-1')

      expect(mockedGetDocuments).toHaveBeenCalledWith(ORG_ID, WS_ID, 'kb-1')
      expect(store.documents).toEqual([doc])
    })

    it('sets error on failure', async () => {
      mockedGetDocuments.mockRejectedValue(new Error('getDocuments failed: 500'))

      const store = useKnowledgeBasesStore()
      await store.fetchDocuments(ORG_ID, WS_ID, 'kb-1')

      expect(store.error).toBe('getDocuments failed: 500')
    })
  })

  describe('uploadDocument', () => {
    it('adds document to list and increments doc_count', async () => {
      const doc = fakeDocument({ id: 'doc-new', status: 'pending' })
      mockedAddDocument.mockResolvedValue(doc)

      const store = useKnowledgeBasesStore()
      store.currentKB = fakeKB({ doc_count: 5 })

      const file = new File(['content'], 'test.pdf', { type: 'application/pdf' })
      const result = await store.uploadDocument(ORG_ID, WS_ID, 'kb-1', file)

      expect(mockedAddDocument).toHaveBeenCalledWith(ORG_ID, WS_ID, 'kb-1', file)
      expect(result).toEqual(doc)
      expect(store.documents).toContainEqual(doc)
      expect(store.currentKB?.doc_count).toBe(6)
    })

    it('propagates API errors', async () => {
      mockedAddDocument.mockRejectedValue(new Error('addDocument failed: 413'))

      const store = useKnowledgeBasesStore()
      const file = new File(['x'], 'big.pdf', { type: 'application/pdf' })

      await expect(store.uploadDocument(ORG_ID, WS_ID, 'kb-1', file)).rejects.toThrow(
        'addDocument failed: 413',
      )
    })
  })

  describe('fetchSources', () => {
    it('populates sources on success', async () => {
      const source = fakeSource()
      mockedGetSources.mockResolvedValue([source])

      const store = useKnowledgeBasesStore()
      await store.fetchSources(ORG_ID, WS_ID, 'kb-1')

      expect(mockedGetSources).toHaveBeenCalledWith(ORG_ID, WS_ID, 'kb-1')
      expect(store.sources).toEqual([source])
    })

    it('sets error on failure', async () => {
      mockedGetSources.mockRejectedValue(new Error('getSources failed: 500'))

      const store = useKnowledgeBasesStore()
      await store.fetchSources(ORG_ID, WS_ID, 'kb-1')

      expect(store.error).toBe('getSources failed: 500')
    })
  })

  describe('createSource', () => {
    it('adds source to list', async () => {
      const source = fakeSource({ id: 'src-new', status: 'pending' })
      mockedAddSource.mockResolvedValue(source)

      const store = useKnowledgeBasesStore()
      const result = await store.createSource(ORG_ID, WS_ID, 'kb-1', 'https://example.com')

      expect(mockedAddSource).toHaveBeenCalledWith(ORG_ID, WS_ID, 'kb-1', 'https://example.com')
      expect(result).toEqual(source)
      expect(store.sources).toContainEqual(source)
    })

    it('propagates API errors', async () => {
      mockedAddSource.mockRejectedValue(new Error('addSource failed: 400'))

      const store = useKnowledgeBasesStore()

      await expect(
        store.createSource(ORG_ID, WS_ID, 'kb-1', 'bad-url'),
      ).rejects.toThrow('addSource failed: 400')
    })
  })
})
