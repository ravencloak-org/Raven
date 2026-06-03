import { defineStore } from 'pinia'
import { ref } from 'vue'

import { fetchDefaultProviderHealth, type TestConnectionResult } from '../api/llm-providers'

// Poll cadence. 60s is long enough to be cheap and short enough that a
// flaky tunnel surfaces before users complain. The probe itself is
// bounded by the Go service's 10s context timeout, so worst-case CPU
// cost per tick is one HTTP roundtrip + a JSON parse.
const POLL_INTERVAL_MS = 60_000

// Initial-delay tick. We don't fire immediately on `start()` because the
// auth store often races provider-fetch on cold boot; waiting one second
// lets pinia state settle so the first probe sees the real default.
const INITIAL_DELAY_MS = 1_000

export const useLlmProviderHealthStore = defineStore('llmProviderHealth', () => {
  const lastResult = ref<TestConnectionResult | null>(null)
  const lastCheckedAt = ref<number | null>(null)
  const lastError = ref<string | null>(null)
  const isPolling = ref(false)

  // Active timer handle. Kept in a closure (not a ref) so reactivity
  // doesn't unnecessarily re-run consumers whenever we re-arm the
  // interval. A ref would force every dependent computed to recompute.
  let timer: ReturnType<typeof setTimeout> | null = null
  let currentOrgId: string | null = null

  function isHealthy(): boolean {
    // Treat "not yet checked" as healthy so the toast doesn't flash on
    // every cold-boot before the first probe lands.
    if (lastResult.value === null) return true
    return lastResult.value.ok === true
  }

  // failureReason returns a short string suitable for the toast body
  // and the page banner. Falls back to a generic message when the
  // backend didn't supply a detail.
  function failureReason(): string {
    if (!lastResult.value || lastResult.value.ok) return ''
    return lastResult.value.detail || 'Connection to the configured LLM provider failed.'
  }

  async function probeOnce(orgId: string) {
    try {
      const result = await fetchDefaultProviderHealth(orgId)
      lastResult.value = result
      lastError.value = null
    } catch (err) {
      // A network/auth error talking to OUR API (not the provider) is
      // treated separately: we keep the previous result so a transient
      // backend blip doesn't immediately flap the toast, but expose
      // the error string for debugging.
      lastError.value = err instanceof Error ? err.message : String(err)
    } finally {
      lastCheckedAt.value = Date.now()
    }
  }

  function start(orgId: string) {
    if (!orgId) return
    // Already polling for this org? No-op so DefaultLayout re-mounts
    // (e.g. on route changes that don't unmount the layout) don't
    // double the cadence.
    if (isPolling.value && currentOrgId === orgId) return
    stop()
    currentOrgId = orgId
    isPolling.value = true

    const tick = () => {
      // Bail if stop() raced us between schedule and fire.
      if (!isPolling.value || currentOrgId === null) return
      void probeOnce(currentOrgId).finally(() => {
        if (isPolling.value) {
          timer = setTimeout(tick, POLL_INTERVAL_MS)
        }
      })
    }

    timer = setTimeout(tick, INITIAL_DELAY_MS)
  }

  function stop() {
    isPolling.value = false
    currentOrgId = null
    if (timer !== null) {
      clearTimeout(timer)
      timer = null
    }
  }

  // resetState clears the cached result. Called on sign-out so a fresh
  // sign-in by a different user doesn't briefly inherit the previous
  // org's last health status.
  function resetState() {
    lastResult.value = null
    lastCheckedAt.value = null
    lastError.value = null
  }

  return {
    lastResult,
    lastCheckedAt,
    lastError,
    isPolling,
    isHealthy,
    failureReason,
    start,
    stop,
    resetState,
  }
})
