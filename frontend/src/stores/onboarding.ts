import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { useAuthStore } from './auth'

export const useOnboardingStore = defineStore('onboarding', () => {
  const currentStep = ref<number>(1)

  function storageKey(): string {
    const auth = useAuthStore()
    const userId = auth.user?.id ?? 'anonymous'
    return `onboarding_completed_${userId}`
  }

  const completed = computed<boolean>(() => {
    return localStorage.getItem(storageKey()) === 'true'
  })

  function markComplete(): void {
    localStorage.setItem(storageKey(), 'true')
  }

  function reset(): void {
    currentStep.value = 1
    localStorage.removeItem(storageKey())
  }

  return { completed, currentStep, markComplete, reset }
})
