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

  async function init() {
    sessionExists.value = await Session.doesSessionExist()
  }

  async function loginWithGoogle() {
    const authUrl = await getAuthorisationURLWithQueryParamsAndSetState({
      thirdPartyId: 'google',
      frontendRedirectURI: `${window.location.origin}/callback`,
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
  }
})
