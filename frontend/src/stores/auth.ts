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
        // Must be false behind Cloudflare — the iframe-based session check uses
        // third-party cookies that CF strips, causing an infinite redirect loop.
        checkLoginIframe: false,
      })
      if (authenticated) {
        await _syncFromKeycloak()
        setInterval(async () => {
          const refreshed = await keycloak.updateToken(30)
          if (refreshed) await _syncFromKeycloak()
        }, 60_000)
      }
    } catch {
      // Keycloak unavailable — proceed as unauthenticated
    } finally {
      initialized.value = true
    }
  }

  function login(): void {
    keycloak.login({ redirectUri: window.location.href })
  }

  function logout(): void {
    // Reset PostHog identity before logging out.
    const { reset: resetPostHog } = usePostHog()
    resetPostHog()
    keycloak.logout({ redirectUri: window.location.origin + '/' })
  }

  async function _syncFromKeycloak(): Promise<void> {
    accessToken.value = keycloak.token ?? null

    // Prefer ID token for standard OIDC claims (sub, email, preferred_username).
    // Access token for custom claims (org_id, org_role).
    const idClaims = (keycloak.idTokenParsed ?? {}) as Record<string, unknown>
    const atClaims = (keycloak.tokenParsed ?? {}) as Record<string, unknown>

    let id = (idClaims['sub'] ?? atClaims['sub']) as string | undefined
    let email = (idClaims['email'] ?? atClaims['email']) as string | undefined
    let username = (idClaims['preferred_username'] ?? atClaims['preferred_username']) as string | undefined
    const orgId = (atClaims['org_id'] ?? idClaims['org_id']) as string | undefined
    const orgRole = (atClaims['org_role'] ?? idClaims['org_role']) as string | undefined

    // Fallback: fetch from Keycloak account API when tokens lack user claims.
    if (!id || !email) {
      try {
        const profile = await keycloak.loadUserProfile()
        id = id ?? profile.id
        email = email ?? profile.email
        username = username ?? profile.username
      } catch {
        // Profile endpoint unavailable — continue with what we have
      }
    }

    if (id) {
      user.value = {
        id,
        email: email ?? '',
        username: username ?? '',
        orgId: orgId ?? '',
        orgRole: orgRole ?? '',
      }

      const { identify } = usePostHog()
      identify(user.value.id, {
        email: user.value.email,
        username: user.value.username,
        org_id: user.value.orgId,
        org_role: user.value.orgRole,
      })
    }
  }

  return { user, accessToken, isAuthenticated, initialized, init, login, logout }
})
