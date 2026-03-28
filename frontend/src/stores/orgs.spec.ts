import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useOrgsStore } from './orgs'
import * as orgsApi from '../api/orgs'

vi.mock('../api/orgs')

describe('useOrgsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('fetchOrg populates currentOrg', async () => {
    vi.mocked(orgsApi.getOrg).mockResolvedValue({
      id: 'org-1', name: 'Test Org', slug: 'test-org', status: 'active',
      settings: {}, created_at: '', updated_at: '',
    })
    const store = useOrgsStore()
    await store.fetchOrg('org-1')
    expect(store.currentOrg?.name).toBe('Test Org')
  })
})
