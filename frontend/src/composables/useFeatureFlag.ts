import { ref, onMounted, onUnmounted, type Ref, readonly } from 'vue'
import { usePostHog } from '../plugins/posthog'

/**
 * Reactive composable that tracks a PostHog feature flag.
 *
 * The returned `isEnabled` ref updates automatically whenever PostHog
 * re-evaluates its feature flags (e.g. after identify, or a periodic
 * refresh).
 *
 * When PostHog is not initialised (no API key configured) the flag
 * defaults to `false`.
 *
 * @example
 * ```vue
 * <script setup lang="ts">
 * import { useFeatureFlag } from '@/composables/useFeatureFlag'
 * const { isEnabled: canUseNewDashboard } = useFeatureFlag('new-dashboard')
 * </script>
 * <template>
 *   <NewDashboard v-if="canUseNewDashboard" />
 *   <LegacyDashboard v-else />
 * </template>
 * ```
 */
export function useFeatureFlag(flagName: string): {
  isEnabled: Readonly<Ref<boolean>>
  value: Readonly<Ref<string | boolean | undefined>>
} {
  const { isInitialised, isFeatureEnabled, getFeatureFlag, onFeatureFlags } =
    usePostHog()

  const isEnabled = ref(false)
  const value = ref<string | boolean | undefined>(undefined)

  /** Re-read the flag from PostHog and update reactive refs. */
  function refresh(): void {
    if (!isInitialised.value) return
    isEnabled.value = isFeatureEnabled(flagName) ?? false
    value.value = getFeatureFlag(flagName)
  }

  // PostHog fires the onFeatureFlags callback every time flags are
  // (re-)loaded.  We use a custom event to bridge that into Vue's
  // reactivity system so the flag value stays up-to-date.

  const eventName = `posthog:flags-loaded`

  function handleFlagsLoaded(): void {
    refresh()
  }

  onMounted(() => {
    // Initial evaluation.
    refresh()

    // Listen for flag reloads.  PostHog's `onFeatureFlags` dispatches a
    // CustomEvent that we listen for here.
    window.addEventListener(eventName, handleFlagsLoaded)

    // Also register directly with PostHog so the event fires.
    onFeatureFlags(() => {
      window.dispatchEvent(new CustomEvent(eventName))
    })
  })

  onUnmounted(() => {
    window.removeEventListener(eventName, handleFlagsLoaded)
  })

  return {
    isEnabled: readonly(isEnabled),
    value: readonly(value),
  }
}
