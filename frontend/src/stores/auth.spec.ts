import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const mockUser = vi.hoisted(() => ({
  access_token: 'mock-access-token',
  expired: false,
  profile: {
    sub: 'user-123',
    email: 'test@example.com',
    preferred_username: 'testuser',
    name: 'Test User',
  },
}))

vi.mock('oidc-client-ts', () => {
  class MockWebStorageStateStore {}
  class MockUserManager {
    getUser = vi.fn().mockResolvedValue(mockUser)
    signinRedirect = vi.fn()
    signinRedirectCallback = vi.fn().mockResolvedValue(mockUser)
    signoutRedirect = vi.fn()
  }
  return {
    UserManager: MockUserManager,
    WebStorageStateStore: MockWebStorageStateStore,
  }
})

import { useAuthStore } from './auth'

describe('useAuthStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('initialises as unauthenticated', () => {
    const store = useAuthStore()
    expect(store.isAuthenticated).toBe(false)
  })

  it('exposes token after init', async () => {
    const store = useAuthStore()
    await store.init()
    expect(store.accessToken).toBe('mock-access-token')
    expect(store.isAuthenticated).toBe(true)
  })

  it('exposes user claims after init', async () => {
    const store = useAuthStore()
    await store.init()
    expect(store.user?.profile.email).toBe('test@example.com')
    expect(store.user?.profile.sub).toBe('user-123')
  })

  it('exposes orgId after setOrgId', async () => {
    const store = useAuthStore()
    await store.init()
    store.setOrgId('org-456')
    expect(store.orgId).toBe('org-456')
  })
})
