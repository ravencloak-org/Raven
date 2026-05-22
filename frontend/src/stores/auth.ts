import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import Session from 'supertokens-web-js/recipe/session'
import {
  getAuthorisationURLWithQueryParamsAndSetState,
  signInAndUp,
} from 'supertokens-web-js/recipe/thirdparty'
import { usePostHog } from '../plugins/posthog'

export const useAuthStore = defineStore('auth', () => {
  const sessionExists = ref(false)
  const orgId = ref<string | null>(sessionStorage.getItem('raven_org_id'))
  const isAuthenticated = computed(() => sessionExists.value)
  const hasOrg = computed(() => !!orgId.value)

  /**
   * Probe SuperTokens for an existing session before the app mounts.
   *
   * Defensive: SuperTokens' session check can hang (network stall) or
   * throw (5xx, aborted fetch) if the auth service is unreachable. Both
   * are bad UX — the app would never mount and the user would see a
   * blank page. A 5s timeout + try/catch falls back to "no session" so
   * the router guard sends the user to /login as expected.
   */
  async function init() {
    try {
      sessionExists.value = await Promise.race([
        Session.doesSessionExist(),
        new Promise<boolean>((_, reject) =>
          setTimeout(() => reject(new Error('SuperTokens session check timed out')), 5000),
        ),
      ])
    } catch {
      sessionExists.value = false
    }
  }

  async function loginWithGoogle() {
    // BASE_URL (from Vite's `base`) is "/" for root deployments and
    // "/raven/" for the path-prefixed demo. Without it, prefixed
    // deployments send Google a redirect_uri of `<origin>/callback`
    // which (a) doesn't match Google Console's authorised URIs and
    // (b) bypasses Vue Router's base-path mapping for /callback.
    const authUrl = await getAuthorisationURLWithQueryParamsAndSetState({
      thirdPartyId: 'google',
      frontendRedirectURI: `${window.location.origin}${import.meta.env.BASE_URL}callback`,
    })
    window.location.assign(authUrl)
  }

  async function handleCallback(): Promise<{ isNewUser: boolean; orgId?: string }> {
    const response = await signInAndUp()
    if (response.status !== 'OK') {
      throw new Error('Sign-in failed: ' + response.status)
    }
    sessionExists.value = true

    // Call backend to create/find internal user.
    // Uses GET so cookies are sent with SameSite=Lax (POST blocks cross-origin cookies).
    const res = await fetch(
      `${import.meta.env.VITE_API_BASE_URL}/auth/callback`,
      { method: 'GET', credentials: 'include' },
    )
    if (!res.ok) {
      throw new Error(`Auth callback failed (${res.status})`)
    }
    return res.json()
  }

  async function logout() {
    const { reset: resetPostHog } = usePostHog()
    resetPostHog()
    sessionExists.value = false
    orgId.value = null
    sessionStorage.removeItem('raven_org_id')
    await Session.signOut()
    window.location.href = '/login'
  }

  function setOrgId(id: string) {
    orgId.value = id
    sessionStorage.setItem('raven_org_id', id)
  }

  /**
   * Called in single-user (Raven AI) mode to mark the session as
   * authenticated without going through SuperTokens. The backend injects
   * the local user / org via SingleUserMiddleware so no real token is needed.
   */
  function setLocalMode() {
    sessionExists.value = true
    // Use the fixed local org UUID from the single-user migration seed.
    const localOrgId = '00000000-0000-0000-0000-000000000001'
    orgId.value = localOrgId
    sessionStorage.setItem('raven_org_id', localOrgId)
  }

  return {
    sessionExists,
    orgId,
    isAuthenticated,
    hasOrg,
    init,
    loginWithGoogle,
    handleCallback,
    logout,
    setOrgId,
    setLocalMode,
  }
})
