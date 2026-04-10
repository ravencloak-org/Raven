import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { find } from 'remeda'
import {
  getPlans,
  getUsage,
  createPaymentIntent,
  cancelSubscription as apiCancelSubscription,
  type Plan,
  type Subscription,
  type UsageResponse,
  type CreatePaymentIntentResponse,
} from '../api/billing'

export const useBillingStore = defineStore('billing', () => {
  const plans = ref<Plan[]>([])
  const subscription = ref<Subscription | null>(null)
  const usage = ref<UsageResponse | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const quotaExceeded = ref(false)
  const quotaMessage = ref<string | null>(null)

  const currentPlan = computed<Plan | undefined>(() => {
    if (!subscription.value) return undefined
    return find(plans.value, (p) => p.id === subscription.value!.plan_id)
  })

  async function fetchPlans(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      plans.value = await getPlans()
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function fetchUsage(): Promise<void> {
    error.value = null
    try {
      usage.value = await getUsage()
    } catch (e) {
      error.value = (e as Error).message
    }
  }

  /**
   * Starts polling usage every 30 seconds.
   * Returns a cleanup function to stop polling.
   */
  function startUsagePolling(): () => void {
    void fetchUsage()
    const timer = setInterval(() => {
      void fetchUsage()
    }, 30_000)
    return () => clearInterval(timer)
  }

  async function initiatePayment(planId: string): Promise<CreatePaymentIntentResponse> {
    error.value = null
    try {
      return await createPaymentIntent(planId)
    } catch (e) {
      error.value = (e as Error).message
      throw e
    }
  }

  async function cancel(subscriptionId: string): Promise<void> {
    error.value = null
    try {
      await apiCancelSubscription(subscriptionId)
      if (subscription.value?.id === subscriptionId) {
        subscription.value = null
      }
    } catch (e) {
      error.value = (e as Error).message
      throw e
    }
  }

  function flagQuotaExceeded(msg: string): void {
    quotaExceeded.value = true
    quotaMessage.value = msg
  }

  function clearQuotaExceeded(): void {
    quotaExceeded.value = false
    quotaMessage.value = null
  }

  return {
    plans,
    subscription,
    usage,
    loading,
    error,
    quotaExceeded,
    quotaMessage,
    currentPlan,
    fetchPlans,
    fetchUsage,
    startUsagePolling,
    initiatePayment,
    cancel,
    flagQuotaExceeded,
    clearQuotaExceeded,
  }
})
