import { computed } from 'vue'
import { useAuthStore } from '../stores/auth'

const STORAGE_KEY_PREFIX = 'raven_onboarding_completed_'

/**
 * useOnboarding tracks whether the current user has completed the onboarding
 * wizard. State is persisted in localStorage keyed by user ID so that each
 * user's completion is independent.
 */
export function useOnboarding() {
  const authStore = useAuthStore()

  function storageKey(): string | null {
    const userId = authStore.user?.id
    return userId ? `${STORAGE_KEY_PREFIX}${userId}` : null
  }

  const isCompleted = computed<boolean>(() => {
    const key = storageKey()
    if (!key) return false
    return localStorage.getItem(key) === 'true'
  })

  function markCompleted(): void {
    const key = storageKey()
    if (key) {
      localStorage.setItem(key, 'true')
    }
  }

  function reset(): void {
    const key = storageKey()
    if (key) {
      localStorage.removeItem(key)
    }
  }

  return { isCompleted, markCompleted, reset }
}
