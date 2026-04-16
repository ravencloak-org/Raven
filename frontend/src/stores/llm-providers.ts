import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { filter, find, findIndex } from 'remeda'
import {
  listLlmProviders,
  createLlmProvider,
  updateLlmProvider,
  deleteLlmProvider,
  type LlmProvider,
  type CreateLlmProviderRequest,
  type UpdateLlmProviderRequest,
} from '../api/llm-providers'

export const useLlmProvidersStore = defineStore('llmProviders', () => {
  // --- State ---
  const providers = ref<LlmProvider[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const testingProviderId = ref<string | null>(null)

  // --- Getters ---

  /** Providers assigned org-wide (workspace_id is null). */
  const orgWideProviders = computed(() =>
    filter(providers.value, (p) => p.workspace_id === null),
  )

  /** Providers assigned to a specific workspace. */
  function providersForWorkspace(workspaceId: string): LlmProvider[] {
    return filter(providers.value, (p) => p.workspace_id === workspaceId)
  }

  /** Get a single provider by id. */
  function getById(id: string): LlmProvider | undefined {
    return find(providers.value, (p) => p.id === id)
  }

  // --- Actions ---

  async function fetchProviders(orgId: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      providers.value = await listLlmProviders(orgId)
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function addProvider(
    orgId: string,
    data: CreateLlmProviderRequest,
  ): Promise<LlmProvider> {
    error.value = null
    try {
      const provider = await createLlmProvider(orgId, data)
      providers.value.push(provider)
      return provider
    } catch (e) {
      error.value = (e as Error).message
      throw e
    }
  }

  async function editProvider(
    orgId: string,
    providerId: string,
    data: UpdateLlmProviderRequest,
  ): Promise<LlmProvider> {
    error.value = null
    try {
      const updated = await updateLlmProvider(orgId, providerId, data)
      const idx = findIndex(providers.value, (p) => p.id === providerId)
      if (idx !== -1) {
        providers.value[idx] = updated
      }
      return updated
    } catch (e) {
      error.value = (e as Error).message
      throw e
    }
  }

  async function removeProvider(orgId: string, providerId: string): Promise<void> {
    error.value = null
    try {
      await deleteLlmProvider(orgId, providerId)
      providers.value = filter(providers.value, (p) => p.id !== providerId)
    } catch (e) {
      error.value = (e as Error).message
      throw e
    }
  }

  async function testProviderConnection(
    orgId: string,
    providerId: string,
    testingProviderId.value = providerId
    lastTestResult.value = null
    try {
      lastTestResult.value = result
      return result
    } catch (e) {
        success: false,
        message: (e as Error).message,
      }
      lastTestResult.value = failResult
      return failResult
    } finally {
      testingProviderId.value = null
    }
  }

  return {
    // state
    providers,
    loading,
    error,
    testingProviderId,
    lastTestResult,
    // getters
    orgWideProviders,
    providersForWorkspace,
    getById,
    // actions
    fetchProviders,
    addProvider,
    editProvider,
    removeProvider,
    testProviderConnection,
  }
})
