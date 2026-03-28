import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useLlmProvidersStore } from './llm-providers'
import * as llmApi from '../api/llm-providers'

vi.mock('../api/llm-providers')

const MOCK_PROVIDER: llmApi.LlmProvider = {
  id: 'llm-1',
  org_id: 'org-1',
  workspace_id: null,
  provider_type: 'openai',
  display_name: 'OpenAI Prod',
  model: 'gpt-4o',
  base_url: null,
  api_key_set: true,
  status: 'active',
  created_at: '2026-03-01T00:00:00Z',
}

const MOCK_PROVIDER_WS: llmApi.LlmProvider = {
  id: 'llm-2',
  org_id: 'org-1',
  workspace_id: 'ws-1',
  provider_type: 'anthropic',
  display_name: 'Anthropic Engineering',
  model: 'claude-sonnet-4-20250514',
  base_url: null,
  api_key_set: true,
  status: 'active',
  created_at: '2026-03-10T00:00:00Z',
}

describe('useLlmProvidersStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  // --- fetchProviders ---

  it('fetchProviders populates the providers list', async () => {
    vi.mocked(llmApi.listLlmProviders).mockResolvedValue([MOCK_PROVIDER, MOCK_PROVIDER_WS])

    const store = useLlmProvidersStore()
    await store.fetchProviders('org-1')

    expect(llmApi.listLlmProviders).toHaveBeenCalledWith('org-1')
    expect(store.providers).toHaveLength(2)
    expect(store.providers[0].display_name).toBe('OpenAI Prod')
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('fetchProviders sets error on failure', async () => {
    vi.mocked(llmApi.listLlmProviders).mockRejectedValue(new Error('Network error'))

    const store = useLlmProvidersStore()
    await store.fetchProviders('org-1')

    expect(store.providers).toHaveLength(0)
    expect(store.error).toBe('Network error')
    expect(store.loading).toBe(false)
  })

  it('fetchProviders sets loading during request', async () => {
    let resolvePromise: (value: llmApi.LlmProvider[]) => void
    vi.mocked(llmApi.listLlmProviders).mockReturnValue(
      new Promise((resolve) => {
        resolvePromise = resolve
      }),
    )

    const store = useLlmProvidersStore()
    const promise = store.fetchProviders('org-1')
    expect(store.loading).toBe(true)

    resolvePromise!([MOCK_PROVIDER])
    await promise
    expect(store.loading).toBe(false)
  })

  // --- addProvider ---

  it('addProvider appends the new provider to the list', async () => {
    const newProvider: llmApi.LlmProvider = {
      ...MOCK_PROVIDER,
      id: 'llm-99',
      display_name: 'New Provider',
    }
    vi.mocked(llmApi.createLlmProvider).mockResolvedValue(newProvider)

    const store = useLlmProvidersStore()
    const result = await store.addProvider('org-1', {
      provider_type: 'openai',
      display_name: 'New Provider',
      model: 'gpt-4o',
      api_key: 'sk-test',
    })

    expect(result.id).toBe('llm-99')
    expect(store.providers).toHaveLength(1)
    expect(store.providers[0].display_name).toBe('New Provider')
  })

  it('addProvider sets error and re-throws on failure', async () => {
    vi.mocked(llmApi.createLlmProvider).mockRejectedValue(new Error('Validation error'))

    const store = useLlmProvidersStore()
    await expect(
      store.addProvider('org-1', {
        provider_type: 'openai',
        display_name: 'Bad',
        model: 'gpt-4o',
        api_key: '',
      }),
    ).rejects.toThrow('Validation error')

    expect(store.error).toBe('Validation error')
    expect(store.providers).toHaveLength(0)
  })

  // --- editProvider ---

  it('editProvider updates the provider in-place', async () => {
    const updated: llmApi.LlmProvider = { ...MOCK_PROVIDER, display_name: 'Renamed' }
    vi.mocked(llmApi.listLlmProviders).mockResolvedValue([MOCK_PROVIDER])
    vi.mocked(llmApi.updateLlmProvider).mockResolvedValue(updated)

    const store = useLlmProvidersStore()
    await store.fetchProviders('org-1')

    const result = await store.editProvider('org-1', 'llm-1', { display_name: 'Renamed' })

    expect(result.display_name).toBe('Renamed')
    expect(store.providers[0].display_name).toBe('Renamed')
  })

  it('editProvider sets error and re-throws on failure', async () => {
    vi.mocked(llmApi.updateLlmProvider).mockRejectedValue(new Error('Not found'))

    const store = useLlmProvidersStore()
    await expect(
      store.editProvider('org-1', 'llm-missing', { display_name: 'X' }),
    ).rejects.toThrow('Not found')

    expect(store.error).toBe('Not found')
  })

  // --- removeProvider ---

  it('removeProvider removes the provider from the list', async () => {
    vi.mocked(llmApi.listLlmProviders).mockResolvedValue([MOCK_PROVIDER, MOCK_PROVIDER_WS])
    vi.mocked(llmApi.deleteLlmProvider).mockResolvedValue(undefined)

    const store = useLlmProvidersStore()
    await store.fetchProviders('org-1')
    expect(store.providers).toHaveLength(2)

    await store.removeProvider('org-1', 'llm-1')

    expect(store.providers).toHaveLength(1)
    expect(store.providers[0].id).toBe('llm-2')
  })

  it('removeProvider sets error and re-throws on failure', async () => {
    vi.mocked(llmApi.deleteLlmProvider).mockRejectedValue(new Error('Forbidden'))

    const store = useLlmProvidersStore()
    await expect(store.removeProvider('org-1', 'llm-1')).rejects.toThrow('Forbidden')
    expect(store.error).toBe('Forbidden')
  })

  // --- testProviderConnection ---

  it('testProviderConnection returns success result', async () => {
    vi.mocked(llmApi.testConnection).mockResolvedValue({
      success: true,
      message: 'Connection successful',
      latency_ms: 120,
    })

    const store = useLlmProvidersStore()
    const result = await store.testProviderConnection('org-1', 'llm-1')

    expect(result.success).toBe(true)
    expect(result.latency_ms).toBe(120)
    expect(store.lastTestResult?.success).toBe(true)
    expect(store.testingProviderId).toBeNull()
  })

  it('testProviderConnection sets testingProviderId during request', async () => {
    let resolvePromise: (value: llmApi.TestConnectionResult) => void
    vi.mocked(llmApi.testConnection).mockReturnValue(
      new Promise((resolve) => {
        resolvePromise = resolve
      }),
    )

    const store = useLlmProvidersStore()
    const promise = store.testProviderConnection('org-1', 'llm-1')
    expect(store.testingProviderId).toBe('llm-1')

    resolvePromise!({ success: true, message: 'OK' })
    await promise
    expect(store.testingProviderId).toBeNull()
  })

  it('testProviderConnection returns failure on error', async () => {
    vi.mocked(llmApi.testConnection).mockRejectedValue(new Error('Timeout'))

    const store = useLlmProvidersStore()
    const result = await store.testProviderConnection('org-1', 'llm-1')

    expect(result.success).toBe(false)
    expect(result.message).toBe('Timeout')
    expect(store.lastTestResult?.success).toBe(false)
    expect(store.testingProviderId).toBeNull()
  })

  // --- Computed / getters ---

  it('orgWideProviders filters by null workspace_id', async () => {
    vi.mocked(llmApi.listLlmProviders).mockResolvedValue([MOCK_PROVIDER, MOCK_PROVIDER_WS])

    const store = useLlmProvidersStore()
    await store.fetchProviders('org-1')

    expect(store.orgWideProviders).toHaveLength(1)
    expect(store.orgWideProviders[0].id).toBe('llm-1')
  })

  it('providersForWorkspace filters by workspace_id', async () => {
    vi.mocked(llmApi.listLlmProviders).mockResolvedValue([MOCK_PROVIDER, MOCK_PROVIDER_WS])

    const store = useLlmProvidersStore()
    await store.fetchProviders('org-1')

    const wsProviders = store.providersForWorkspace('ws-1')
    expect(wsProviders).toHaveLength(1)
    expect(wsProviders[0].id).toBe('llm-2')
  })

  it('getById returns the matching provider', async () => {
    vi.mocked(llmApi.listLlmProviders).mockResolvedValue([MOCK_PROVIDER, MOCK_PROVIDER_WS])

    const store = useLlmProvidersStore()
    await store.fetchProviders('org-1')

    expect(store.getById('llm-2')?.display_name).toBe('Anthropic Engineering')
    expect(store.getById('llm-missing')).toBeUndefined()
  })
})
