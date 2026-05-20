import { defineStore } from 'pinia'
import { ref } from 'vue'

interface ServerConfig {
  single_user: boolean
}

// Allow test harnesses to pre-seed the single-user flag via an init script
// that runs before any module code. This sidesteps the async fetch race
// where the router guard fires before serverConfig.load() resolves.
declare global {
  interface Window {
    __RAVEN_SINGLE_USER?: boolean
  }
}

/**
 * Holds server-side feature flags fetched from GET /api/v1/config on boot.
 * Consumed by the router guard to skip the login flow in single-user mode.
 *
 * If `window.__RAVEN_SINGLE_USER` is set (e.g. by a Playwright addInitScript),
 * that value is used immediately without waiting for the network fetch so the
 * router guard sees the correct flag on the very first navigation.
 */
export const useServerConfigStore = defineStore('serverConfig', () => {
  // Synchronously seed from the window flag if present (set by test harnesses
  // or Tauri shell before the Vue app boots).
  const singleUser = ref(window.__RAVEN_SINGLE_USER ?? false)
  const loaded = ref(window.__RAVEN_SINGLE_USER !== undefined)

  async function load() {
    if (loaded.value) return
    try {
      // VITE_API_BASE_URL already ends with /api/v1 (matches the
      // convention used everywhere else in src/api/*) — appending it
      // again here doubled the path on path-prefixed deployments.
      const res = await fetch(`${import.meta.env.VITE_API_BASE_URL}/config`)
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
