import { computed } from 'vue'
import { useAuthStore } from '../stores/auth'

export function useAuth() {
  const store = useAuthStore()
  return {
    isAuthenticated: computed(() => store.isAuthenticated),
    hasOrg: computed(() => store.hasOrg),
    loginWithGoogle: () => store.loginWithGoogle(),
    logout: () => store.logout(),
  }
}
