import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getOrg, createOrg, type Org } from '../api/orgs'

export const useOrgsStore = defineStore('orgs', () => {
  const currentOrg = ref<Org | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchOrg(orgId: string) {
    loading.value = true
    error.value = null
    try {
      currentOrg.value = await getOrg(orgId)
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function create(name: string): Promise<Org> {
    const org = await createOrg(name)
    currentOrg.value = org
    return org
  }

  return { currentOrg, loading, error, fetchOrg, create }
})
