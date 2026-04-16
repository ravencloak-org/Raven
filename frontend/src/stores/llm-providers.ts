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
  }
})
