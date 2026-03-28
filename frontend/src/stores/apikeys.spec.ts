import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useApiKeysStore } from './apikeys'
import * as apikeysApi from '../api/apikeys'
import type { ApiKey, CreateApiKeyResponse } from '../api/apikeys'

vi.mock('../api/apikeys')

const mockKey: ApiKey = {
  id: 'key-001',
  name: 'Test Key',
  key_prefix: 'rk_test_',
  org_id: 'org-456',
  workspace_id: 'ws-001',
  allowed_domains: ['example.com'],
  rate_limit: 1000,
  status: 'active',
  created_at: '2026-03-20T10:30:00Z',
}

const revokedKey: ApiKey = {
  ...mockKey,
  id: 'key-002',
  name: 'Revoked Key',
  status: 'revoked',
}

describe('useApiKeysStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('fetchKeys', () => {
    it('populates keys on success', async () => {
      vi.mocked(apikeysApi.listApiKeys).mockResolvedValue([mockKey, revokedKey])

      const store = useApiKeysStore()
      await store.fetchKeys()

      expect(store.keys).toHaveLength(2)
      expect(store.keys[0].name).toBe('Test Key')
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('sets loading state during fetch', async () => {
      let resolvePromise: (value: ApiKey[]) => void
      vi.mocked(apikeysApi.listApiKeys).mockImplementation(
        () => new Promise((resolve) => { resolvePromise = resolve }),
      )

      const store = useApiKeysStore()
      const promise = store.fetchKeys()

      expect(store.loading).toBe(true)

      resolvePromise!([mockKey])
      await promise

      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      vi.mocked(apikeysApi.listApiKeys).mockRejectedValue(new Error('Network error'))

      const store = useApiKeysStore()
      await store.fetchKeys()

      expect(store.error).toBe('Network error')
      expect(store.keys).toHaveLength(0)
    })

    it('computes activeKeys and revokedKeys', async () => {
      vi.mocked(apikeysApi.listApiKeys).mockResolvedValue([mockKey, revokedKey])

      const store = useApiKeysStore()
      await store.fetchKeys()

      expect(store.activeKeys).toHaveLength(1)
      expect(store.activeKeys[0].id).toBe('key-001')
      expect(store.revokedKeys).toHaveLength(1)
      expect(store.revokedKeys[0].id).toBe('key-002')
    })
  })

  describe('create', () => {
    it('adds key and stores raw key on success', async () => {
      const response: CreateApiKeyResponse = {
        api_key: mockKey,
        raw_key: 'rk_live_abc123def456',
      }
      vi.mocked(apikeysApi.createApiKey).mockResolvedValue(response)

      const store = useApiKeysStore()
      const result = await store.create({
        name: 'Test Key',
        allowed_domains: ['example.com'],
        rate_limit: 1000,
      })

      expect(store.keys).toHaveLength(1)
      expect(store.keys[0].name).toBe('Test Key')
      expect(store.lastCreatedRawKey).toBe('rk_live_abc123def456')
      expect(result.raw_key).toBe('rk_live_abc123def456')
    })

    it('sets error and rethrows on failure', async () => {
      vi.mocked(apikeysApi.createApiKey).mockRejectedValue(new Error('Create failed'))

      const store = useApiKeysStore()

      await expect(
        store.create({ name: 'Fail', allowed_domains: [], rate_limit: 100 }),
      ).rejects.toThrow('Create failed')

      expect(store.error).toBe('Create failed')
      expect(store.keys).toHaveLength(0)
    })
  })

  describe('revoke', () => {
    it('updates key status in the list', async () => {
      vi.mocked(apikeysApi.listApiKeys).mockResolvedValue([{ ...mockKey }])
      vi.mocked(apikeysApi.revokeApiKey).mockResolvedValue({ ...mockKey, status: 'revoked' })

      const store = useApiKeysStore()
      await store.fetchKeys()
      await store.revoke('key-001')

      expect(store.keys[0].status).toBe('revoked')
    })

    it('sets error and rethrows on failure', async () => {
      vi.mocked(apikeysApi.revokeApiKey).mockRejectedValue(new Error('Revoke failed'))

      const store = useApiKeysStore()

      await expect(store.revoke('key-999')).rejects.toThrow('Revoke failed')
      expect(store.error).toBe('Revoke failed')
    })
  })

  describe('updateSettings', () => {
    it('updates key settings in the list', async () => {
      vi.mocked(apikeysApi.listApiKeys).mockResolvedValue([{ ...mockKey }])
      vi.mocked(apikeysApi.updateApiKeySettings).mockResolvedValue({
        ...mockKey,
        allowed_domains: ['new.example.com'],
        rate_limit: 2000,
      })

      const store = useApiKeysStore()
      await store.fetchKeys()
      await store.updateSettings('key-001', {
        allowed_domains: ['new.example.com'],
        rate_limit: 2000,
      })

      expect(store.keys[0].allowed_domains).toEqual(['new.example.com'])
      expect(store.keys[0].rate_limit).toBe(2000)
    })

    it('sets error and rethrows on failure', async () => {
      vi.mocked(apikeysApi.updateApiKeySettings).mockRejectedValue(
        new Error('Update failed'),
      )

      const store = useApiKeysStore()

      await expect(
        store.updateSettings('key-999', { rate_limit: 500 }),
      ).rejects.toThrow('Update failed')
      expect(store.error).toBe('Update failed')
    })
  })

  describe('clearLastCreatedKey', () => {
    it('resets lastCreatedRawKey to null', async () => {
      const response: CreateApiKeyResponse = {
        api_key: mockKey,
        raw_key: 'rk_live_abc123',
      }
      vi.mocked(apikeysApi.createApiKey).mockResolvedValue(response)

      const store = useApiKeysStore()
      await store.create({ name: 'Key', allowed_domains: [], rate_limit: 100 })

      expect(store.lastCreatedRawKey).toBe('rk_live_abc123')

      store.clearLastCreatedKey()

      expect(store.lastCreatedRawKey).toBeNull()
    })
  })
})
