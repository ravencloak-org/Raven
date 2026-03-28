import { computed } from 'vue'
import { useAuthStore } from '../stores/auth'

export function useAuth() {
  const store = useAuthStore()
  return {
    user: computed(() => store.user),
    isAuthenticated: computed(() => store.isAuthenticated),
    login: () => store.login(),
    logout: () => store.logout(),
  }
}
