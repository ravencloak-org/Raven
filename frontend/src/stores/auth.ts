import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import Keycloak from 'keycloak-js'
import { usePostHog } from '../plugins/posthog'

export interface AuthUser {
  id: string
  email: string
  username: string
  orgId: string
  orgRole: string
}

const keycloak = new Keycloak({
  url: import.meta.env.VITE_KEYCLOAK_URL ?? 'http://localhost:8080',
  realm: import.meta.env.VITE_KEYCLOAK_REALM ?? 'raven',
  clientId: import.meta.env.VITE_KEYCLOAK_CLIENT_ID ?? 'raven-admin',
})

export const useAuthStore = defineStore('auth', () => {
  const user = ref<AuthUser | null>(null)
  const accessToken = ref<string | null>(null)
  const initialized = ref(false)

  const isAuthenticated = computed(() => initialized.value && !!accessToken.value)

  async function init(): Promise<void> {
    try {
      const authenticated = await keycloak.init({
        onLoad: 'check-sso',
        pkceMethod: 'S256',
        silentCheckSsoRedirectUri: window.location.origin + '/silent-check-sso.html',
      })
      if (authenticated) {
        _syncFromKeycloak()
        // Auto-refresh token 30s before expiry
        setInterval(() => keycloak.updateToken(30), 60_000)
      }
    } catch {
      // Keycloak unavailable — proceed as unauthenticated
    } finally {
      initialized.value = true
    }
  }

  function login(): void {
    keycloak.login({ redirectUri: window.location.origin + '/dashboard' })
  }

  function logout(): void {
    // Reset PostHog identity before logging out.
    const { reset: resetPostHog } = usePostHog()
    resetPostHog()
    keycloak.logout({ redirectUri: window.location.origin + '/' })
  }

  function _syncFromKeycloak(): void {
    const p = keycloak.tokenParsed as Record<string, unknown>
    accessToken.value = keycloak.token ?? null
    user.value = {
      id: p['sub'] as string,
      email: p['email'] as string,
      username: p['preferred_username'] as string,
      orgId: p['org_id'] as string,
      orgRole: p['org_role'] as string,
    }

    // Identify the authenticated user in PostHog for analytics.
    const { identify } = usePostHog()
    identify(user.value.id, {
      email: user.value.email,
      username: user.value.username,
      org_id: user.value.orgId,
      org_role: user.value.orgRole,
    })
  }

  return { user, accessToken, isAuthenticated, initialized, init, login, logout }
})
