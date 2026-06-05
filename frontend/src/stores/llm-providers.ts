// KEEP (issue #831 audit): reference implementation of the optimistic-update +
// snapshot-rollback pattern (setDefault). Issue #831 itself names this as the
// pattern other stores should match — not a shallow wrapper.
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  type LlmProvider,
  type CreateLlmProviderRequest,
  type UpdateLlmProviderRequest,
  listLlmProviders,
  createLlmProvider,
  updateLlmProvider,
  deleteLlmProvider,
  setDefaultProvider,
} from '../api/llm-providers'

export const useLlmProvidersStore = defineStore('llmProviders', () => {
  const providers = ref<LlmProvider[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  const activeProviders = computed(() => providers.value.filter((p) => p.status === 'active'))
  const defaultProvider = computed(() => providers.value.find((p) => p.is_default))

  function getById(id: string) {
    return providers.value.find((p) => p.id === id)
  }

  async function fetchProviders(orgId: string) {
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

  async function addProvider(orgId: string, data: CreateLlmProviderRequest) {
    const created = await createLlmProvider(orgId, data)
    providers.value.push(created)
    return created
  }

  async function editProvider(orgId: string, providerId: string, data: UpdateLlmProviderRequest) {
    const updated = await updateLlmProvider(orgId, providerId, data)
    const idx = providers.value.findIndex((p) => p.id === providerId)
    if (idx !== -1) providers.value[idx] = updated
    return updated
  }

  async function removeProvider(orgId: string, providerId: string) {
    await deleteLlmProvider(orgId, providerId)
    providers.value = providers.value.filter((p) => p.id !== providerId)
  }

  /**
   * Optimistically promotes `providerId` to be the org's default provider.
   * Locally flips `is_default` on the target to `true` and on every other
   * row to `false` BEFORE the network call so the UI updates instantly.
   * On API failure the previous default snapshot is restored and the
   * error is rethrown so callers can show a toast.
   */
  async function setDefault(orgId: string, providerId: string) {
    const target = providers.value.find((p) => p.id === providerId)
    if (!target) {
      throw new Error(`Provider ${providerId} not found`)
    }
    if (target.is_default) {
      return target
    }
    // Snapshot previous is_default state so we can roll back on failure.
    const previous = providers.value.map((p) => ({ id: p.id, is_default: p.is_default }))
    // Optimistic flip: exactly one row is the default at any time.
    providers.value = providers.value.map((p) => ({
      ...p,
      is_default: p.id === providerId,
    }))
    try {
      const updated = await setDefaultProvider(orgId, providerId)
      const idx = providers.value.findIndex((p) => p.id === providerId)
      if (idx !== -1) providers.value[idx] = updated
      return updated
    } catch (e) {
      // Roll back optimistic state.
      providers.value = providers.value.map((p) => {
        const prev = previous.find((q) => q.id === p.id)
        return prev ? { ...p, is_default: prev.is_default } : p
      })
      error.value = (e as Error).message
      throw e
    }
  }

  return {
    providers,
    loading,
    error,
    activeProviders,
    defaultProvider,
    getById,
    fetchProviders,
    addProvider,
    editProvider,
    removeProvider,
    setDefault,
  }
})
