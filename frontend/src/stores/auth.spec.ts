import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('supertokens-web-js/recipe/session', () => ({
  default: {
    doesSessionExist: vi.fn().mockResolvedValue(true),
    signOut: vi.fn().mockResolvedValue(undefined),
  },
}))

vi.mock('supertokens-web-js/recipe/thirdparty', () => ({
  getAuthorisationURLWithQueryParamsAndSetState: vi.fn().mockResolvedValue('https://accounts.google.com/o/oauth2/auth?mock=1'),
  signInAndUp: vi.fn().mockResolvedValue({ status: 'OK' }),
}))

vi.mock('../plugins/posthog', () => ({
  usePostHog: () => ({ reset: vi.fn(), identify: vi.fn() }),
}))

import { useAuthStore } from './auth'

describe('useAuthStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('initialises as unauthenticated', () => {
    const store = useAuthStore()
    expect(store.isAuthenticated).toBe(false)
  })

  it('sets sessionExists after init', async () => {
    const store = useAuthStore()
    await store.init()
    expect(store.isAuthenticated).toBe(true)
  })

  it('exposes orgId after setOrgId', async () => {
    const store = useAuthStore()
    await store.init()
    store.setOrgId('org-456')
    expect(store.orgId).toBe('org-456')
  })
})
