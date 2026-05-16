import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export type RamTier = 'unknown' | 'floor' | 'low' | 'mid' | 'high'

export interface PrecheckResult {
  ram_gb: number
  free_disk_gb: number
  cpu_cores: number
  ok: boolean
  warnings: unknown[]
}

/**
 * Returns the RAM tier band for the host. Bands match the model-picker spec
 * and the per-model "fits X GB hosts" descriptions in ModelPickerStep.vue:
 *   ≤ 8 GB   → 'floor' (only llama3.2:3b — "fits 8 GB hosts")
 *   9-11 GB  → 'low'   (default 8b model is OK but tight)
 *   12-15 GB → 'mid'   (default 8b model is comfortable)
 *   16+ GB   → 'high'  (13b also offered)
 */
export function ramTier(ramGb: number): RamTier {
  if (ramGb < 9) return 'floor'
  if (ramGb < 12) return 'low'
  if (ramGb < 16) return 'mid'
  return 'high'
}

/**
 * Holds the structured result of the Tauri-side system-requirements precheck
 * (#420). Updated either by an explicit `applyResult` call (tests, frontend
 * code that has the data already) or by `subscribeToTauri()` which listens
 * to the `precheck:result` event emitted by the Rust shell on boot.
 *
 * On the cloud frontend the Tauri import resolves to nothing; the store
 * simply stays at its `unknown` default, which is fine since the model-
 * picker step is only rendered in single-user mode.
 */
export const usePrecheckStore = defineStore('precheck', () => {
  const result = ref<PrecheckResult | null>(null)

  const tier = computed<RamTier>(() => {
    if (!result.value) return 'unknown'
    return ramTier(result.value.ram_gb)
  })

  function applyResult(r: PrecheckResult) {
    result.value = r
  }

  async function subscribeToTauri() {
    try {
      const { listen } = await import('@tauri-apps/api/event')
      await listen<PrecheckResult>('precheck:result', (e) => {
        applyResult(e.payload)
      })
    } catch {
      // Not in a Tauri context — no-op.
    }
  }

  return { result, tier, applyResult, subscribeToTauri }
})
