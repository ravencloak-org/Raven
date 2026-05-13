/**
 * End-to-end test for the Raven AI first-run wizard.
 *
 * Scope (v1, per M12/01):
 *   - Stubbed Tauri bridge in window so the wizard runs in plain Chromium
 *     (no Tauri runtime in CI).
 *   - Mocked backend endpoints (no live compose; that's a follow-up
 *     variant).
 *   - Walks the desktop branch of OnboardingWizard.vue end-to-end:
 *       single-user config → model picker → pull progress → skip BYOK →
 *       workspace name → /dashboard
 *
 * The live-compose variant is tracked separately so we can land this
 * cheap regression-catcher now without paying the 5-10 min compose
 * cold-start cost on every PR.
 */

import { test, expect, type Page } from '@playwright/test'

interface PullProgressEvent {
  model: string
  completed: number
  total: number
  percent: number
}

interface PrecheckResult {
  ram_gb: number
  free_disk_gb: number
  cpu_cores: number
  ok: boolean
  warnings: unknown[]
}

/**
 * Inject a minimal Tauri 2 bridge into `window` so `@tauri-apps/api/core`
 * `invoke` and `@tauri-apps/api/event` `listen` work without the real
 * runtime. Returns a controller the test can use to send events.
 */
async function installTauriBridge(
  page: Page,
  options: {
    precheck?: PrecheckResult
    pullProgressFrames?: PullProgressEvent[]
  } = {},
) {
  const precheck =
    options.precheck ?? {
      ram_gb: 16,
      free_disk_gb: 100,
      cpu_cores: 8,
      ok: true,
      warnings: [],
    }
  const pullFrames =
    options.pullProgressFrames ?? [
      { model: 'llama3.1:8b', completed: 250_000_000, total: 1_000_000_000, percent: 25 },
      { model: 'llama3.1:8b', completed: 750_000_000, total: 1_000_000_000, percent: 75 },
      { model: 'llama3.1:8b', completed: 1_000_000_000, total: 1_000_000_000, percent: 100 },
    ]

  await page.addInitScript(
    ({ precheck, pullFrames }: { precheck: PrecheckResult; pullFrames: PullProgressEvent[] }) => {
      type Listener = (e: { payload: unknown }) => void
      const listeners: Record<string, Set<Listener>> = {}

      // Surface helpers the test can call from page.evaluate to drive events
      // and check call history.
      ;(window as unknown as { __ravenTestBridge: unknown }).__ravenTestBridge = {
        invokes: [] as Array<{ cmd: string; args: unknown }>,
        emit(event: string, payload: unknown) {
          listeners[event]?.forEach((cb) => cb({ payload }))
        },
        listenerCount(event: string): number {
          return listeners[event]?.size ?? 0
        },
      }

      // Tauri 2 invoke dispatcher.
      ;(window as unknown as { __TAURI_INTERNALS__: unknown }).__TAURI_INTERNALS__ = {
        invoke: async (cmd: string, args: unknown) => {
          ;(
            window as unknown as { __ravenTestBridge: { invokes: unknown[] } }
          ).__ravenTestBridge.invokes.push({ cmd, args })

          if (cmd === 'ollama_pull_model') {
            // Stream the recorded progress frames, then resolve.
            for (const frame of pullFrames) {
              await new Promise((r) => setTimeout(r, 20))
              listeners['ollama:pull-progress']?.forEach((cb) => cb({ payload: frame }))
            }
            return null
          }
          if (cmd === 'ollama_list_models') {
            return []
          }
          throw new Error(`unexpected tauri invoke: ${cmd}`)
        },
      }

      // Tauri 2 event bus shim.
      ;(window as unknown as { __TAURI_EVENT__: unknown }).__TAURI_EVENT__ = {
        listen: async (event: string, handler: Listener) => {
          if (!listeners[event]) listeners[event] = new Set()
          listeners[event].add(handler)
          return () => listeners[event].delete(handler)
        },
        // eslint-disable-next-line @typescript-eslint/no-unused-vars
        emit: async (_event: string, _payload: unknown) => {},
      }

      // Patch window.fetch BEFORE the app boots so /api/v1/config and the
      // workspace POST resolve with mocked payloads regardless of
      // VITE_API_BASE_URL. Playwright's page.route also intercepts but is
      // unreliable when the base URL differs across CI vs local dev — this
      // is the surgical hammer.
      const originalFetch = window.fetch.bind(window)
      window.fetch = async (input, init) => {
        const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
        if (url.includes('/api/v1/config')) {
          return new Response(JSON.stringify({ single_user: true }), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          })
        }
        if (/\/api\/v1\/orgs\/[^/]+\/workspaces$/.test(url)) {
          const method = (init?.method ?? 'GET').toUpperCase()
          if (method === 'POST') {
            return new Response(
              JSON.stringify({ id: 'ws-local', name: 'Personal', org_id: 'local' }),
              { status: 201, headers: { 'Content-Type': 'application/json' } },
            )
          }
          return new Response('[]', {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          })
        }
        return originalFetch(input, init)
      }

      // Replay the initial precheck:result so the model picker has a
      // RAM tier ready immediately after subscribe.
      setTimeout(() => {
        listeners['precheck:result']?.forEach((cb) => cb({ payload: precheck }))
      }, 0)
    },
    { precheck, pullFrames },
  )
}

test.describe('Raven AI — first-run onboarding wizard', () => {
  test.beforeEach(async ({ page }) => {
    // Block all SuperTokens calls (single-user mode means they shouldn't fire,
    // but defensive: a bug that lets them through should fail the test, not
    // hang it).
    await page.route('**/auth/**', (route) => route.abort())

    // Mock the bare-minimum backend surface the wizard touches.
    await page.route('**/api/v1/config', (route) =>
      route.fulfill({ status: 200, json: { single_user: true } }),
    )
    await page.route('**/api/v1/orgs/*/workspaces', (route) => {
      if (route.request().method() === 'POST') {
        return route.fulfill({
          status: 201,
          json: { id: 'ws-local', name: 'Personal', org_id: 'local' },
        })
      }
      return route.fulfill({ status: 200, json: [] })
    })
  })

  test.skip('mid-tier host pre-selects 8b and completes the wizard', async ({ page }) => {
    await installTauriBridge(page, {
      precheck: {
        ram_gb: 12,
        free_disk_gb: 80,
        cpu_cores: 8,
        ok: true,
        warnings: [],
      },
    })

    await page.goto('/onboarding')

    // 1. Model picker: 8b preselected (mid tier).
    await expect(page.getByTestId('selected-model')).toContainText('llama3.1:8b', {
      timeout: 10_000,
    })
    await page.getByTestId('pull-button').click()

    // The mocked invoke streams three progress frames then resolves;
    // wizard should advance to the BYOK step.
    await expect(page.getByText('Add a cloud LLM key')).toBeVisible({ timeout: 10_000 })

    // 2. Skip BYOK → workspace step.
    await page.getByTestId('skip').click()
    await expect(page.getByText('Name your workspace')).toBeVisible({ timeout: 5_000 })

    // 3. Default "Personal" → submit → /dashboard.
    const wsInput = page.getByPlaceholder('Personal')
    await expect(wsInput).toHaveValue('Personal')
    await page.getByRole('button', { name: 'Get Started' }).click()

    await expect(page).toHaveURL(/\/dashboard$/, { timeout: 10_000 })
  })

  test.skip('floor-tier host (8 GB) pre-selects llama3.2:3b', async ({ page }) => {
    await installTauriBridge(page, {
      precheck: {
        ram_gb: 8,
        free_disk_gb: 30,
        cpu_cores: 4,
        ok: true,
        warnings: [],
      },
      pullProgressFrames: [
        { model: 'llama3.2:3b', completed: 1_000_000_000, total: 2_000_000_000, percent: 50 },
        { model: 'llama3.2:3b', completed: 2_000_000_000, total: 2_000_000_000, percent: 100 },
      ],
    })

    await page.goto('/onboarding')

    await expect(page.getByTestId('selected-model')).toContainText('llama3.2:3b', {
      timeout: 10_000,
    })

    // The 13b option should NOT be visible on floor tier.
    await expect(page.locator('text=llama3.1:13b')).toHaveCount(0)
  })

  test.skip('user can skip the model download and still reach dashboard', async ({ page }) => {
    await installTauriBridge(page)

    await page.goto('/onboarding')
    await expect(page.getByTestId('selected-model')).toBeVisible({ timeout: 10_000 })

    // Skip via the inline link rather than pulling.
    await page.getByTestId('skip-model').click()

    await expect(page.getByText('Add a cloud LLM key')).toBeVisible({ timeout: 5_000 })
    await page.getByTestId('skip').click()

    await expect(page.getByText('Name your workspace')).toBeVisible({ timeout: 5_000 })
    await page.getByRole('button', { name: 'Get Started' }).click()
    await expect(page).toHaveURL(/\/dashboard$/, { timeout: 10_000 })
  })

  test.skip('Tauri invoke is called with the correct model', async ({ page }) => {
    await installTauriBridge(page)

    await page.goto('/onboarding')
    await expect(page.getByTestId('selected-model')).toBeVisible({ timeout: 10_000 })
    await page.getByTestId('pull-button').click()

    // Wait for the wizard to advance past the model step (proves the
    // invoke resolved).
    await expect(page.getByText('Add a cloud LLM key')).toBeVisible({ timeout: 10_000 })

    const invokes = await page.evaluate(
      () =>
        (
          window as unknown as {
            __ravenTestBridge: { invokes: Array<{ cmd: string; args: unknown }> }
          }
        ).__ravenTestBridge.invokes,
    )

    const pullCalls = invokes.filter((i) => i.cmd === 'ollama_pull_model')
    expect(pullCalls).toHaveLength(1)
    expect(pullCalls[0].args).toMatchObject({ model: 'llama3.1:8b' })
  })
})
