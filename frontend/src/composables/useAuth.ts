import { computed } from 'vue'
import { useAuthStore } from '../stores/auth'

export function useAuth() {
  const store = useAuthStore()
  return {
    user: computed(() => store.user),
    isAuthenticated: computed(() => store.isAuthenticated),
    hasOrg: computed(() => store.hasOrg),
    accessToken: computed(() => store.accessToken),
    login: (idpHint?: string) => store.login(idpHint),
    logout: () => store.logout(),
  }
}
