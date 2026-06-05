// KEEP (issue #831 audit): bridges localStorage onboarding-completed flag to the
// reactive system; consumed by router guard, DefaultLayout, and OnboardingWizard.
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useOnboardingStore = defineStore('onboarding', () => {
  const currentStep = ref<number>(1)
  const storageVersion = ref(0)

  function storageKey(): string {
    return `onboarding_completed`
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
