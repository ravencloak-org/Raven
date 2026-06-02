import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useLlmProvidersStore } from './llm-providers'
import * as llmApi from '../api/llm-providers'
import type { LlmProvider } from '../api/llm-providers'

vi.mock('../api/llm-providers')

const openai: LlmProvider = {
  id: 'p-openai',
  org_id: 'org-1',
  provider: 'openai',
  display_name: 'OpenAI Prod',
  base_url: null,
  api_key_hint: 'sk-...abc',
  is_default: true,
  status: 'active',
  config: {},
  created_at: '2026-05-01T00:00:00Z',
  updated_at: '2026-05-01T00:00:00Z',
}

const anthropic: LlmProvider = {
  ...openai,
  id: 'p-anthropic',
  provider: 'anthropic',
  display_name: 'Anthropic',
  is_default: false,
}

describe('useLlmProvidersStore.setDefault', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('optimistically flips is_default and persists server response', async () => {
    vi.mocked(llmApi.listLlmProviders).mockResolvedValue([
      { ...openai },
      { ...anthropic },
    ])
    // Resolve only after we have a chance to inspect the optimistic state.
    let resolveSet: (v: LlmProvider) => void = () => {}
    vi.mocked(llmApi.setDefaultProvider).mockImplementation(
      () => new Promise<LlmProvider>((r) => { resolveSet = r }),
    )

    const store = useLlmProvidersStore()
    await store.fetchProviders('org-1')

    const promise = store.setDefault('org-1', 'p-anthropic')

    // Optimistic flip applied immediately — before the API resolves.
    expect(store.providers.find((p) => p.id === 'p-anthropic')!.is_default).toBe(true)
    expect(store.providers.find((p) => p.id === 'p-openai')!.is_default).toBe(false)

    resolveSet({ ...anthropic, is_default: true })
    await promise

    expect(store.providers.find((p) => p.id === 'p-anthropic')!.is_default).toBe(true)
    expect(store.providers.find((p) => p.id === 'p-openai')!.is_default).toBe(false)
    expect(store.error).toBeNull()
  })

  it('rolls back both rows when the server call fails', async () => {
    vi.mocked(llmApi.listLlmProviders).mockResolvedValue([
      { ...openai },
      { ...anthropic },
    ])
    vi.mocked(llmApi.setDefaultProvider).mockRejectedValue(new Error('boom'))

    const store = useLlmProvidersStore()
    await store.fetchProviders('org-1')

    await expect(store.setDefault('org-1', 'p-anthropic')).rejects.toThrow('boom')

    expect(store.providers.find((p) => p.id === 'p-openai')!.is_default).toBe(true)
    expect(store.providers.find((p) => p.id === 'p-anthropic')!.is_default).toBe(false)
    expect(store.error).toBe('boom')
  })

  it('is a no-op when the target is already the default', async () => {
    vi.mocked(llmApi.listLlmProviders).mockResolvedValue([{ ...openai }, { ...anthropic }])
    const store = useLlmProvidersStore()
    await store.fetchProviders('org-1')

    const result = await store.setDefault('org-1', 'p-openai')

    expect(result?.id).toBe('p-openai')
    expect(llmApi.setDefaultProvider).not.toHaveBeenCalled()
  })

  it('throws if the provider does not exist', async () => {
    vi.mocked(llmApi.listLlmProviders).mockResolvedValue([{ ...openai }])
    const store = useLlmProvidersStore()
    await store.fetchProviders('org-1')

    await expect(store.setDefault('org-1', 'p-missing')).rejects.toThrow(
      /not found/,
    )
    expect(llmApi.setDefaultProvider).not.toHaveBeenCalled()
  })
})
