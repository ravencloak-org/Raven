import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  listKnowledgeBases,
  getKnowledgeBase,
  createKnowledgeBase,
  archiveKnowledgeBase,
  getDocuments,
  addDocument,
  getSources,
  addSource,
  type KnowledgeBase,
  type KBDocument,
  type KBSource,
} from '../api/knowledge-bases'

export const useKnowledgeBasesStore = defineStore('knowledgeBases', () => {
  const knowledgeBases = ref<KnowledgeBase[]>([])
  const currentKB = ref<KnowledgeBase | null>(null)
  const documents = ref<KBDocument[]>([])
  const sources = ref<KBSource[]>([])
  const total = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchKnowledgeBases(orgId: string, wsId: string, offset = 0, limit = 20) {
    loading.value = true
    error.value = null
    try {
      const res = await listKnowledgeBases(orgId, wsId, offset, limit)
      knowledgeBases.value = res.items
      total.value = res.total
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function fetchKnowledgeBase(orgId: string, wsId: string, kbId: string) {
    loading.value = true
    error.value = null
    try {
      currentKB.value = await getKnowledgeBase(orgId, wsId, kbId)
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function create(
    orgId: string,
    wsId: string,
    input: { name: string; settings?: Record<string, unknown> },
  ): Promise<KnowledgeBase> {
    const kb = await createKnowledgeBase(orgId, wsId, input)
    knowledgeBases.value.push(kb)
    total.value += 1
    return kb
  }

  async function archive(orgId: string, wsId: string, kbId: string): Promise<void> {
    await archiveKnowledgeBase(orgId, wsId, kbId)
    const idx = knowledgeBases.value.findIndex((k) => k.id === kbId)
    if (idx !== -1) knowledgeBases.value[idx] = { ...knowledgeBases.value[idx], status: 'archived' }
    if (currentKB.value?.id === kbId) {
      currentKB.value = { ...currentKB.value, status: 'archived' }
    }
  }

  async function fetchDocuments(orgId: string, wsId: string, kbId: string) {
    try {
      documents.value = await getDocuments(orgId, wsId, kbId)
    } catch (e) {
      error.value = (e as Error).message
    }
  }

  async function uploadDocument(
    orgId: string,
    wsId: string,
    kbId: string,
    file: File,
  ): Promise<KBDocument> {
    const doc = await addDocument(orgId, wsId, kbId, file)
    documents.value.push(doc)
    if (currentKB.value?.id === kbId) {
      currentKB.value = { ...currentKB.value, doc_count: currentKB.value.doc_count + 1 }
    }
    return doc
  }

  async function fetchSources(orgId: string, wsId: string, kbId: string) {
    try {
      sources.value = await getSources(orgId, wsId, kbId)
    } catch (e) {
      error.value = (e as Error).message
    }
  }

  async function createSource(
    orgId: string,
    wsId: string,
    kbId: string,
    url: string,
  ): Promise<KBSource> {
    const source = await addSource(orgId, wsId, kbId, url)
    sources.value.push(source)
    return source
  }

  return {
    knowledgeBases,
    currentKB,
    documents,
    sources,
    total,
    loading,
    error,
    fetchKnowledgeBases,
    fetchKnowledgeBase,
    create,
    archive,
    fetchDocuments,
    uploadDocument,
    fetchSources,
    createSource,
  }
})
