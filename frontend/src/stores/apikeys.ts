import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  listApiKeys,
  createApiKey,
  revokeApiKey,
  updateApiKeySettings,
  type ApiKey,
  type CreateApiKeyRequest,
  type CreateApiKeyResponse,
  type UpdateApiKeySettingsRequest,
} from '../api/apikeys'

export const useApiKeysStore = defineStore('apikeys', () => {
  const keys = ref<ApiKey[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  /** The full raw key value, shown only once after creation */
  const lastCreatedRawKey = ref<string | null>(null)

  const activeKeys = computed(() => keys.value.filter((k) => k.status === 'active'))
  const revokedKeys = computed(() => keys.value.filter((k) => k.status === 'revoked'))

  async function fetchKeys(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      keys.value = await listApiKeys()
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function create(req: CreateApiKeyRequest): Promise<CreateApiKeyResponse> {
    error.value = null
    try {
      const result = await createApiKey(req)
      keys.value.push(result.api_key)
      lastCreatedRawKey.value = result.raw_key
      return result
    } catch (e) {
      error.value = (e as Error).message
      throw e
    }
  }

  async function revoke(keyId: string): Promise<void> {
    error.value = null
    try {
      const updated = await revokeApiKey(keyId)
      const idx = keys.value.findIndex((k) => k.id === keyId)
      if (idx !== -1) keys.value[idx] = updated
    } catch (e) {
      error.value = (e as Error).message
      throw e
    }
  }

  async function updateSettings(
    keyId: string,
    settings: UpdateApiKeySettingsRequest,
  ): Promise<void> {
    error.value = null
    try {
      const updated = await updateApiKeySettings(keyId, settings)
      const idx = keys.value.findIndex((k) => k.id === keyId)
      if (idx !== -1) keys.value[idx] = updated
    } catch (e) {
      error.value = (e as Error).message
      throw e
    }
  }

  function clearLastCreatedKey(): void {
    lastCreatedRawKey.value = null
  }

  return {
    keys,
    loading,
    error,
    activeKeys,
    revokedKeys,
    lastCreatedRawKey,
    fetchKeys,
    create,
    revoke,
    updateSettings,
    clearLastCreatedKey,
  }
})
