import { defineStore } from 'pinia'
import { ref } from 'vue'

const STORAGE_KEY = 'onboarding_completed'

export const useOnboardingStore = defineStore('onboarding', () => {
  const completed = ref<boolean>(localStorage.getItem(STORAGE_KEY) === 'true')
  const currentStep = ref<number>(1)

  function markComplete(): void {
    completed.value = true
    localStorage.setItem(STORAGE_KEY, 'true')
  }

  function reset(): void {
    completed.value = false
    currentStep.value = 1
    localStorage.removeItem(STORAGE_KEY)
  }

  return { completed, currentStep, markComplete, reset }
})
