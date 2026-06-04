import { ref, watch, type Ref, type WatchSource } from 'vue'
import {
  testLlmProviderConnection,
  type TestProviderRequest,
} from '../api/llm-providers'

export type TestConnectionStatus = 'idle' | 'testing' | 'pass' | 'fail'

export interface UseTestConnectionGateOptions {
  /**
   * Closure that produces the payload to ship to
   * `POST /api/v1/orgs/:org_id/llm-providers/test`. Called lazily on each
   * `runTest()` invocation so the dialog can shape the body around its
   * current form state (e.g. `{provider_id}` vs `{provider, api_key}`).
   */
  buildPayload: () => TestProviderRequest
  /**
   * Reactive deps that invalidate a prior pass when ANY of them changes —
   * the gate falls back to `idle` so the user is forced to re-test. Pass
   * getter functions (the standard `WatchSource` shape) so the consumer
   * controls reactivity tracking.
   */
  invalidateOn: WatchSource[]
  /** Current org id — re-evaluated on each `runTest()` call. */
  orgId: () => string
}

export interface TestConnectionGate {
  status: Ref<TestConnectionStatus>
  detail: Ref<string>
  runTest: () => Promise<void>
  reset: () => void
}

/**
 * Composable wrapper around the credential-probe state machine used by
 * both the Create and Edit dialogs. Exposes a single `status` + `detail`
 * pair and a `runTest()` method; every entry in `invalidateOn` rolls the
 * status back to `idle` on change so a passing probe can't be smuggled
 * past a later edit.
 */
export function useTestConnectionGate(
  options: UseTestConnectionGateOptions,
): TestConnectionGate {
  const status = ref<TestConnectionStatus>('idle')
  const detail = ref<string>('')

  function reset() {
    if (status.value !== 'idle') {
      status.value = 'idle'
      detail.value = ''
    }
  }

  async function runTest() {
    status.value = 'testing'
    detail.value = ''
    try {
      const payload = options.buildPayload()
      const result = await testLlmProviderConnection(options.orgId(), payload)
      if (result.ok) {
        status.value = 'pass'
        detail.value = result.detail ?? ''
      } else {
        status.value = 'fail'
        detail.value = result.detail ?? 'Probe rejected the credentials.'
      }
    } catch (e: unknown) {
      status.value = 'fail'
      detail.value = e instanceof Error ? e.message : 'Test connection failed'
    }
  }

  watch(options.invalidateOn, () => {
    reset()
  })

  return { status, detail, runTest, reset }
}
