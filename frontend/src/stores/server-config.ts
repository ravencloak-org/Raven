import { defineStore } from 'pinia'
import { ref } from 'vue'

interface ServerConfig {
  single_user: boolean
}

/**
 * Holds server-side feature flags fetched from GET /api/v1/config on boot.
 * Consumed by the router guard to skip the login flow in single-user mode.
 */
export const useServerConfigStore = defineStore('serverConfig', () => {
  const singleUser = ref(false)
  const loaded = ref(false)

  async function load() {
    if (loaded.value) return
    try {
      const res = await fetch(
        `${import.meta.env.VITE_API_BASE_URL}/api/v1/config`,
      )
      if (res.ok) {
        const data: ServerConfig = await res.json()
        singleUser.value = data.single_user ?? false
      }
    } catch {
      // Network error — assume multi-user mode (safe default).
      singleUser.value = false
    }
    loaded.value = true
  }

  return { singleUser, loaded, load }
})
