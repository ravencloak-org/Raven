import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAuthStore } from './auth'

// Mock keycloak-js with a real class so `new Keycloak(...)` works
vi.mock('keycloak-js', () => ({
  default: class MockKeycloak {
    init = vi.fn().mockResolvedValue(true)
    login = vi.fn()
    logout = vi.fn()
    updateToken = vi.fn().mockResolvedValue(true)
    token = 'mock-access-token'
    tokenParsed = {
      sub: 'user-123',
      email: 'test@example.com',
      preferred_username: 'testuser',
      org_id: 'org-456',
      org_role: 'org_admin',
    }
    authenticated = true
    isTokenExpired = vi.fn().mockReturnValue(false)
  },
}))

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
    expect(store.user?.email).toBe('test@example.com')
    expect(store.user?.orgId).toBe('org-456')
  })
})
