import { defineStore } from 'pinia'
import { UserManager, WebStorageStateStore } from 'oidc-client-ts'
import type { User } from 'oidc-client-ts'
import { ref, computed } from 'vue'
import { usePostHog } from '../plugins/posthog'

const userManager = new UserManager({
  authority: import.meta.env.VITE_ZITADEL_URL,
  client_id: import.meta.env.VITE_ZITADEL_CLIENT_ID,
  redirect_uri: `${window.location.origin}/callback`,
  post_logout_redirect_uri: window.location.origin,
  silent_redirect_uri: `${window.location.origin}/silent-renew.html`,
  scope: 'openid profile email',
  response_type: 'code',
  automaticSilentRenew: true,
  userStore: new WebStorageStateStore({ store: window.sessionStorage }),
})

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const orgId = ref<string | null>(sessionStorage.getItem('raven_org_id'))
  const isAuthenticated = computed(() => !!user.value && !user.value.expired)
  const hasOrg = computed(() => !!orgId.value)
  const accessToken = computed(() => user.value?.access_token ?? null)

  async function init() {
    const existingUser = await userManager.getUser()
    if (existingUser && !existingUser.expired) {
      user.value = existingUser
      _identify(existingUser)
    }
  }

  async function login(idpHint?: string) {
    const extraParams: Record<string, string> = {}
    if (idpHint) extraParams.idp_hint = idpHint
    await userManager.signinRedirect({ extraQueryParams: extraParams })
  }

  async function handleCallback(): Promise<User> {
    const callbackUser = await userManager.signinRedirectCallback()
    user.value = callbackUser
    _identify(callbackUser)
    return callbackUser
  }

  async function logout() {
    const { reset: resetPostHog } = usePostHog()
    resetPostHog()
    await userManager.signoutRedirect()
    user.value = null
    orgId.value = null
    sessionStorage.removeItem('raven_org_id')
  }

  function setOrgId(id: string) {
    orgId.value = id
    sessionStorage.setItem('raven_org_id', id)
  }

  function _identify(u: User) {
    const { identify } = usePostHog()
    identify(u.profile.sub, {
      email: u.profile.email,
      name: u.profile.name,
    })
  }

  return { user, orgId, isAuthenticated, hasOrg, accessToken, init, login, handleCallback, logout, setOrgId }
})
