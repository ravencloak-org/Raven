import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  getConversationVolume,
  getTopQueries,
  getSourceHits,
  getUnansweredQuestions,
  type ConversationVolumePoint,
  type TopQuery,
  type SourceHit,
  type UnansweredQuestion,
  type DateRange,
} from '../api/analytics'

export const useAnalyticsStore = defineStore('analytics', () => {
  const conversationVolume = ref<ConversationVolumePoint[]>([])
  const topQueries = ref<TopQuery[]>([])
  const sourceHits = ref<SourceHit[]>([])
  const unansweredQuestions = ref<UnansweredQuestion[]>([])
  const selectedRange = ref<DateRange>('30d')
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchAll(range?: DateRange) {
    if (range) {
      selectedRange.value = range
    }
    loading.value = true
    error.value = null
    try {
      const r = selectedRange.value
      const [volume, queries, sources, unanswered] = await Promise.all([
        getConversationVolume(r),
        getTopQueries(r),
        getSourceHits(r),
        getUnansweredQuestions(r),
      ])
      conversationVolume.value = volume
      topQueries.value = queries
      sourceHits.value = sources
      unansweredQuestions.value = unanswered
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function changeRange(range: DateRange) {
    await fetchAll(range)
  }

  return {
    conversationVolume,
    topQueries,
    sourceHits,
    unansweredQuestions,
    selectedRange,
    loading,
    error,
    fetchAll,
    changeRange,
  }
})
