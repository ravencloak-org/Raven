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
  // Platform-admin flag — populated by /api/v1/me. Presentation-only;
  // the backend enforces admin routes via middleware against
  // RAVEN_ADMIN_EMAILS, so a tampered client cannot escalate.
  const isPlatformAdmin = ref(false)
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
   *
   * Cold-boot race: on a hard page load (vs in-app navigation),
   * Session.doesSessionExist() can return false EVEN when the
   * sFrontToken cookie is present and unexpired — the SDK's internal
   * state hasn't hydrated from cookies yet. Calling it again ~50ms later
   * returns true. To avoid bouncing the user to /login on every page
   * reload, fall back to parsing the sFrontToken cookie directly: if
   * its session expiry (`up.exp`) is in the future, treat as a valid
   * session. The SDK will catch up and subsequent calls behave
   * normally.
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
    if (!sessionExists.value && hasValidFrontTokenCookie()) {
      sessionExists.value = true
    }

    // sessionStorage is per-tab; closing/re-opening the tab wipes raven_org_id
    // so hasOrg falls to false on the next cold-boot. The router guard then
    // bounces every authenticated user to /onboarding even when their org has
    // existed for days. Refetch orgId from /api/v1/me whenever the session
    // exists but the in-memory orgId is missing — that's the canonical source
    // for the current user's org_id, set by the backend on every login.
    if (sessionExists.value && !orgId.value) {
      // Bound /me with a 5s abort so a stalled API can't block init().
      // init() runs before the first paint, so any wait here delays
      // the entire app boot and brings back the blank-page behaviour
      // this function is already trying to protect against.
      const controller = new AbortController()
      const timeoutId = window.setTimeout(() => controller.abort(), 5000)
      try {
        const res = await fetch(`${import.meta.env.VITE_API_BASE_URL}/me`, {
          credentials: 'include',
          signal: controller.signal,
        })
        if (res.ok) {
          const me = (await res.json()) as { org_id?: string; is_platform_admin?: boolean }
          if (me.org_id) setOrgId(me.org_id)
          isPlatformAdmin.value = !!me.is_platform_admin
        }
      } catch {
        // Swallow (including AbortError) — if /me is unreachable the
        // router will send the user to /onboarding which can re-derive
        // the org from the auth callback.
      } finally {
        window.clearTimeout(timeoutId)
      }
    }
  }

  function hasValidFrontTokenCookie(): boolean {
    const raw = readCookie('sFrontToken')
    if (!raw) return false
    try {
      const payload = JSON.parse(atob(decodeURIComponent(raw))) as {
        up?: { exp?: number }
      }
      const expSec = payload?.up?.exp
      if (typeof expSec !== 'number') return false
      // `up.exp` is seconds-since-epoch (matches the embedded JWT claim).
      return expSec * 1000 > Date.now()
    } catch {
      return false
    }
  }

  function readCookie(name: string): string | null {
    const prefix = `${name}=`
    for (const part of document.cookie.split(';')) {
      const trimmed = part.trim()
      if (trimmed.startsWith(prefix)) return trimmed.slice(prefix.length)
    }
    return null
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
    isPlatformAdmin,
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
