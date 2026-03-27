import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { User } from '../types'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const token = ref<string | null>(localStorage.getItem('auth_token'))

  const isAuthenticated = computed(() => !!user.value && !!token.value)

  function setUser(newUser: User | null) {
    user.value = newUser
  }

  function setToken(newToken: string | null) {
    token.value = newToken
    if (newToken) {
      localStorage.setItem('auth_token', newToken)
    } else {
      localStorage.removeItem('auth_token')
    }
  }

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  async function login(email: string, _password: string): Promise<void> {
    // Placeholder: real auth will use Keycloak OIDC redirect
    // This simulates a login for development purposes
    const mockUser: User = {
      id: '1',
      email,
      displayName: 'Dev User',
      orgId: 'org-1',
      orgRole: 'owner',
    }
    setUser(mockUser)
    setToken('mock-jwt-token')
  }

  function logout() {
    setUser(null)
    setToken(null)
  }

  return {
    user,
    token,
    isAuthenticated,
    setUser,
    setToken,
    login,
    logout,
  }
})
