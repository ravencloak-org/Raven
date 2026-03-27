import { computed } from 'vue'
import { useAuthStore } from '../stores/auth'
import type { User } from '../types'

export function useAuth() {
  const authStore = useAuthStore()

  const currentUser = computed<User | null>(() => authStore.user)
  const isAuthenticated = computed(() => authStore.isAuthenticated)

  async function login(email: string, password: string): Promise<void> {
    await authStore.login(email, password)
  }

  function logout(): void {
    authStore.logout()
  }

  return {
    currentUser,
    isAuthenticated,
    login,
    logout,
  }
}
