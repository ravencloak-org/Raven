import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { useAuthStore } from './auth'

export const useOnboardingStore = defineStore('onboarding', () => {
  const currentStep = ref<number>(1)
  const storageVersion = ref(0)

  function storageKey(): string {
    const auth = useAuthStore()
    const userId = auth.user?.profile.sub ?? 'anonymous'
    return `onboarding_completed_${userId}`
  }

  const completed = computed<boolean>(() => {
    void storageVersion.value // reactive dependency for localStorage writes
    return localStorage.getItem(storageKey()) === 'true'
  })

  function markComplete(): void {
    localStorage.setItem(storageKey(), 'true')
    storageVersion.value++
  }

  function reset(): void {
    currentStep.value = 1
    localStorage.removeItem(storageKey())
    storageVersion.value++
  }

  return { completed, currentStep, markComplete, reset }
})
