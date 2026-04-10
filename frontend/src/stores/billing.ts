import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getUsage, getSubscription, type BillingUsage, type Subscription } from '../api/billing'

const POLL_INTERVAL_MS = 30_000

export const useBillingStore = defineStore('billing', () => {
  const usage = ref<BillingUsage | null>(null)
  const subscription = ref<Subscription | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const upgradePromptOpen = ref(false)
  const upgradeFeature = ref<string | undefined>(undefined)

  let _pollTimer: ReturnType<typeof setInterval> | null = null

  async function fetchUsage(orgId: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      usage.value = await getUsage(orgId)
    } catch (e) {
      const err = e as Error & { status?: number }
      if (err.status === 402) {
        showUpgradePrompt()
      }
      error.value = err.message
    } finally {
      loading.value = false
    }
  }

  async function fetchSubscription(orgId: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      subscription.value = await getSubscription(orgId)
    } catch (e) {
      const err = e as Error & { status?: number }
      if (err.status === 402) {
        showUpgradePrompt()
      }
      error.value = err.message
    } finally {
      loading.value = false
    }
  }

  function startPolling(orgId: string): void {
    stopPolling()
    _pollTimer = setInterval(() => {
      fetchUsage(orgId)
    }, POLL_INTERVAL_MS)
  }

  function stopPolling(): void {
    if (_pollTimer !== null) {
      clearInterval(_pollTimer)
      _pollTimer = null
    }
  }

  function showUpgradePrompt(feature?: string): void {
    upgradeFeature.value = feature
    upgradePromptOpen.value = true
  }

  function hideUpgradePrompt(): void {
    upgradePromptOpen.value = false
    upgradeFeature.value = undefined
  }

  return {
    usage,
    subscription,
    loading,
    error,
    upgradePromptOpen,
    upgradeFeature,
    fetchUsage,
    fetchSubscription,
    startPolling,
    stopPolling,
    showUpgradePrompt,
    hideUpgradePrompt,
  }
})
