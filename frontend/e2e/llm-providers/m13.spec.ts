/**
 * M13 LLM-Providers Management UI — Playwright coverage (Issue #746).
 *
 * Scope: drive the LlmProviderListPage end-to-end with the backend mocked
 * via `page.route`. Auth is short-circuited by:
 *   - pre-seeding the server-config store in single-user mode via
 *     `window.__RAVEN_SINGLE_USER` (see `useServerConfigStore`), which
 *     causes the router guard to skip the SuperTokens session probe;
 *   - seeding `sessionStorage['raven_org_id']` so the page reads the
 *     fixture orgId without hitting `/api/v1/me`.
 *
 * Each scenario corresponds to an Issue F bullet:
 *   1. Switch default               — optimistic flip lands within 200 ms
 *   2. Rollback on failed switch    — 500 reverts pill + shows toast
 *   3. Inline rename                — click name, type, blur, reload → persisted
 *   4. Inline model change          — change <select>, reload → persisted
 *   5. Rotate API key               — Save gated by passing test probe
 *   6. base_url without rotate      — probe body uses {provider_id} shape
 *   7. PUT omits api_key            — Save without Rotate → PUT has no api_key
 *   8. Menu reachability            — header avatar + sidebar both navigate
 */
import { test, expect, type BrowserContext, type Page, type Route } from '@playwright/test'

// Matches the single-user mode local org UUID baked into `useAuthStore`
// (`setLocalMode()`). Single-user mode boots the dashboard with this org
// pre-set, so mocking the providers API requires the SAME id.
const ORG_ID = '00000000-0000-0000-0000-000000000001'

type Provider = {
  id: string
  org_id: string
  provider: 'openai' | 'anthropic' | 'ollama' | 'custom'
  display_name: string
  base_url: string | null
  api_key_hint: string | null
  is_default: boolean
  status: 'active' | 'inactive'
  config: Record<string, unknown>
  created_at: string
  updated_at: string
}

function makeProvider(overrides: Partial<Provider> = {}): Provider {
  const now = '2026-06-02T00:00:00Z'
  return {
    id: overrides.id ?? 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
    org_id: ORG_ID,
    provider: overrides.provider ?? 'openai',
    display_name: overrides.display_name ?? 'OpenAI Primary',
    base_url: overrides.base_url ?? null,
    api_key_hint: overrides.api_key_hint ?? 'sk-...abcd',
    is_default: overrides.is_default ?? false,
    status: overrides.status ?? 'active',
    config: overrides.config ?? { model: 'gpt-4o' },
    created_at: now,
    updated_at: now,
    ...overrides,
  }
}

/**
 * Tiny in-memory store that backs the mocked routes for one test. Each test
 * gets its own instance so they can mutate state without bleed-through.
 */
class MockBackend {
  providers: Provider[]
  /** Last request body received per (METHOD path-pattern) so assertions can
   * verify what the page actually shipped over the wire. */
  lastBody: Record<string, unknown> = {}
  /** Optional override: if non-null, the next `PUT .../default` returns this status. */
  setDefaultStatus: number | null = null
  /** Optional override: route handler for POST .../test. */
  testHandler: ((body: Record<string, unknown>) => { ok: boolean; provider: string; detail?: string }) | null = null

  constructor(initial: Provider[]) {
    this.providers = initial.map((p) => ({ ...p }))
  }

  find(id: string): Provider | undefined {
    return this.providers.find((p) => p.id === id)
  }
}

async function seedSession(page: Page, context: BrowserContext) {
  // Pre-seed single-user mode BEFORE the app boots so the router guard
  // skips SuperTokens. addInitScript runs before any module code,
  // including Pinia store hydration. (orgId is pinned by `setLocalMode`
  // in main.ts when single-user is true, so we don't need to seed it.)
  await page.addInitScript(() => {
    ;(window as unknown as { __RAVEN_SINGLE_USER: boolean }).__RAVEN_SINGLE_USER = true
  })
  // context.route is required: `page.route` skips XHR initiated by lazy
  // modules in this app, but context.route catches them reliably.
  await context.route('**/auth/**', (route) => route.abort())
  await context.route('**/api/v1/config', (route) =>
    route.fulfill({ status: 200, json: { single_user: true } }),
  )
}

async function installProviderRoutes(context: BrowserContext, backend: MockBackend) {
  // Playwright matches routes in REVERSE registration order (most-recently
  // added wins). Register the LEAST-specific pattern first so the more
  // specific `/test` and `/*/default` handlers below override it for
  // the exact paths they care about.
  // PUT / DELETE on a single provider — also matches `/test` lexically,
  // but the `/test` route registered later takes precedence.
  await context.route(`**/api/v1/orgs/${ORG_ID}/llm-providers/*`, async (route: Route) => {
    const method = route.request().method()
    const url = new URL(route.request().url())
    const id = url.pathname.split('/').pop() ?? ''
    if (method === 'PUT') {
      const body = JSON.parse(route.request().postData() ?? '{}')
      backend.lastBody['PUT /llm-providers/:id'] = body
      const target = backend.find(id)
      if (!target) return route.fulfill({ status: 404, json: { error: 'not found' } })
      // Mirror backend semantics: when api_key is OMITTED the stored
      // hint is preserved; when provided we update the hint.
      if (body.display_name !== undefined) target.display_name = body.display_name
      if (body.base_url !== undefined) target.base_url = body.base_url
      if (body.config !== undefined) target.config = body.config
      if (body.is_default !== undefined) target.is_default = body.is_default
      if (body.status !== undefined) target.status = body.status
      if (body.api_key !== undefined && body.api_key) {
        target.api_key_hint = `${String(body.api_key).slice(0, 5)}...`
      }
      target.updated_at = '2026-06-02T01:00:00Z'
      return route.fulfill({ status: 200, json: target })
    }
    if (method === 'DELETE') {
      backend.providers = backend.providers.filter((p) => p.id !== id)
      return route.fulfill({ status: 204, body: '' })
    }
    return route.continue()
  })

  // GET / POST collection
  await context.route(`**/api/v1/orgs/${ORG_ID}/llm-providers`, async (route: Route) => {
    if (route.request().method() === 'GET') {
      return route.fulfill({ status: 200, json: backend.providers })
    }
    if (route.request().method() === 'POST') {
      const body = JSON.parse(route.request().postData() ?? '{}')
      backend.lastBody['POST /llm-providers'] = body
      const created = makeProvider({
        id: `new-${Date.now()}`,
        display_name: body.display_name,
        provider: body.provider,
        base_url: body.base_url ?? null,
        api_key_hint: body.api_key ? `${String(body.api_key).slice(0, 5)}...` : null,
        is_default: !!body.is_default,
      })
      backend.providers.push(created)
      return route.fulfill({ status: 201, json: created })
    }
    return route.continue()
  })

  // POST test probe — MUST be registered BEFORE the wildcard `:id` route
  // below so Playwright matches the more specific pattern first.
  await context.route(`**/api/v1/orgs/${ORG_ID}/llm-providers/test`, async (route: Route) => {
    const body = JSON.parse(route.request().postData() ?? '{}')
    backend.lastBody['POST /llm-providers/test'] = body
    const result = backend.testHandler
      ? backend.testHandler(body)
      : { ok: true, provider: 'openai', detail: 'ok' }
    return route.fulfill({ status: 200, json: result })
  })

  // PUT .../:id/default
  await context.route(`**/api/v1/orgs/${ORG_ID}/llm-providers/*/default`, async (route: Route) => {
    const url = new URL(route.request().url())
    const id = url.pathname.split('/').slice(-2, -1)[0]
    if (backend.setDefaultStatus && backend.setDefaultStatus >= 400) {
      return route.fulfill({ status: backend.setDefaultStatus, json: { error: 'forced failure' } })
    }
    backend.providers = backend.providers.map((p) => ({ ...p, is_default: p.id === id }))
    const updated = backend.find(id)
    return route.fulfill({ status: 200, json: updated })
  })

}

async function gotoProviders(page: Page) {
  await page.goto('/llm-providers')
  // Single-user mode pins orgId via setLocalMode() in main.ts, so no extra
  // setup is needed here — wait for the heading and for at least one
  // provider card to mount before we drive the rest of the test.
  await expect(page.getByRole('heading', { name: 'LLM Providers' })).toBeVisible()
  await expect(page.getByTestId('provider-card').first()).toBeVisible({ timeout: 15000 })
}

// The repo-wide playwright.config.ts sets testIdAttribute to `data-test`,
// but LlmProviderListPage uses `data-testid` (the Playwright default).
// Override just for this spec so `getByTestId(...)` matches the components
// shipped in PRs #748 / #751.
test.use({ testIdAttribute: 'data-testid' })

test.describe('LLM Providers (M13)', () => {
  test('1) Switch default — optimistic flip lands within 200 ms', async ({ page, context }) => {
    const backend = new MockBackend([
      makeProvider({ id: 'p-openai', display_name: 'OpenAI Primary', is_default: true }),
      makeProvider({
        id: 'p-anthropic',
        provider: 'anthropic',
        display_name: 'Anthropic Backup',
        is_default: false,
        config: { model: 'claude-sonnet-4-20250514' },
      }),
    ])
    await seedSession(page, context)
    await installProviderRoutes(context, backend)

    // Add a 500 ms delay to the network call so the optimistic flip is
    // observable BEFORE the server responds. If the UI waits for the
    // server, the assertion below will time out. Re-registering a route
    // with `context.route` overrides the one installed above.
    await context.route(
      `**/api/v1/orgs/${ORG_ID}/llm-providers/*/default`,
      async (route) => {
        const id = new URL(route.request().url()).pathname.split('/').slice(-2, -1)[0]
        await new Promise((r) => setTimeout(r, 500))
        backend.providers = backend.providers.map((p) => ({ ...p, is_default: p.id === id }))
        return route.fulfill({ status: 200, json: backend.find(id) })
      },
    )

    await gotoProviders(page)
    await expect(page.getByTestId('provider-card')).toHaveCount(2, { timeout: 10000 })

    // The default pill starts under "OpenAI Primary".
    const openaiCard = page
      .getByTestId('provider-card')
      .filter({ hasText: 'OpenAI Primary' })
    const anthCard = page
      .getByTestId('provider-card')
      .filter({ hasText: 'Anthropic Backup' })
    await expect(openaiCard.getByTestId('default-pill')).toBeVisible()
    await expect(anthCard.getByTestId('default-pill')).toHaveCount(0)

    // Click Make default on the non-default card and assert the pill
    // moves WITHIN 200 ms — well under the artificial 500 ms server delay.
    const clickedAt = Date.now()
    await anthCard.getByTestId('make-default-btn').click()
    await expect(anthCard.getByTestId('default-pill')).toBeVisible({ timeout: 250 })
    expect(Date.now() - clickedAt).toBeLessThan(250)
    await expect(openaiCard.getByTestId('default-pill')).toHaveCount(0)

    // And once the server resolves, the body the page would send to a
    // subsequent chat call resolves to the new default.
    await page.waitForResponse(
      (r) =>
        r.url().includes(`/llm-providers/p-anthropic/default`) && r.status() === 200,
    )
    const updated = backend.find('p-anthropic')
    expect(updated?.is_default).toBe(true)
  })

  test('2) Rollback on failed switch — 500 reverts pill + shows toast', async ({ page, context }) => {
    const backend = new MockBackend([
      makeProvider({ id: 'p-openai', display_name: 'OpenAI Primary', is_default: true }),
      makeProvider({
        id: 'p-anthropic',
        provider: 'anthropic',
        display_name: 'Anthropic Backup',
        is_default: false,
      }),
    ])
    backend.setDefaultStatus = 500
    await seedSession(page, context)
    await installProviderRoutes(context, backend)

    await gotoProviders(page)
    const openaiCard = page
      .getByTestId('provider-card')
      .filter({ hasText: 'OpenAI Primary' })
    const anthCard = page
      .getByTestId('provider-card')
      .filter({ hasText: 'Anthropic Backup' })
    await expect(openaiCard.getByTestId('default-pill')).toBeVisible()

    await anthCard.getByTestId('make-default-btn').click()
    // After the 500, the pill must be back under the original card and
    // the toast must surface the failure.
    await expect(openaiCard.getByTestId('default-pill')).toBeVisible()
    await expect(anthCard.getByTestId('default-pill')).toHaveCount(0)
    await expect(page.getByTestId('default-error-toast')).toBeVisible()
  })

  test('3) Inline rename persists across reload', async ({ page, context }) => {
    const backend = new MockBackend([
      makeProvider({ id: 'p-openai', display_name: 'OpenAI Primary', is_default: true }),
    ])
    await seedSession(page, context)
    await installProviderRoutes(context, backend)

    await gotoProviders(page)
    const card = page.getByTestId('provider-card').first()
    await card.getByTestId('display-name-label').click()
    const input = card.getByTestId('display-name-input')
    await expect(input).toBeVisible()
    await input.fill('Renamed OpenAI')
    // Enter commits and exits the inline editor. PUT fires synchronously
    // afterwards — wait for it before reloading.
    await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().includes(`/orgs/${ORG_ID}/llm-providers/p-openai`) &&
          r.request().method() === 'PUT',
      ),
      input.press('Enter'),
    ])
    await expect(card.getByTestId('display-name-label')).toContainText('Renamed OpenAI')
    // The PUT body must NOT contain api_key (inline rename never ships secrets).
    const putBody = backend.lastBody['PUT /llm-providers/:id'] as Record<string, unknown>
    expect(putBody).toBeDefined()
    expect(putBody.display_name).toBe('Renamed OpenAI')
    expect('api_key' in putBody).toBe(false)

    await page.reload()
    await expect(
      page.getByTestId('provider-card').getByTestId('display-name-label'),
    ).toContainText('Renamed OpenAI')
  })

  test('4) Inline model change persists across reload', async ({ page, context }) => {
    const backend = new MockBackend([
      makeProvider({
        id: 'p-openai',
        display_name: 'OpenAI Primary',
        is_default: true,
        config: { model: 'gpt-4o' },
      }),
    ])
    await seedSession(page, context)
    await installProviderRoutes(context, backend)

    await gotoProviders(page)
    const card = page.getByTestId('provider-card').first()
    const modelSelect = card.getByTestId('model-select')
    await expect(modelSelect).toHaveValue('gpt-4o')
    await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().includes(`/orgs/${ORG_ID}/llm-providers/p-openai`) &&
          r.request().method() === 'PUT',
      ),
      modelSelect.selectOption('gpt-4o-mini'),
    ])
    const putBody = backend.lastBody['PUT /llm-providers/:id'] as Record<string, unknown>
    expect(putBody).toBeDefined()
    const config = putBody.config as Record<string, unknown>
    expect(config.model).toBe('gpt-4o-mini')
    expect('api_key' in putBody).toBe(false)

    // The optimistic flip + PUT response both leave the model in
    // store.providers[0].config.model. Reload and re-fetch via the GET
    // mock proves the value survived a round trip.
    await page.reload()
    await expect(page.getByTestId('provider-card').first()).toBeVisible({ timeout: 15000 })
    await expect(
      page.getByTestId('provider-card').getByTestId('model-select'),
    ).toHaveValue('gpt-4o-mini')
  })

  test('5) Rotate API key — Save gated by passing test probe', async ({ page, context }) => {
    const backend = new MockBackend([
      makeProvider({
        id: 'p-openai',
        display_name: 'OpenAI Primary',
        is_default: true,
        api_key_hint: 'sk-...abcd',
      }),
    ])
    await seedSession(page, context)
    await installProviderRoutes(context, backend)

    await gotoProviders(page)
    const card = page.getByTestId('provider-card').first()
    await card.getByTestId('edit-credentials-btn').click()
    const dialog = page.locator('div').filter({ hasText: /^Edit credentials/ }).last()
    await expect(dialog).toBeVisible()

    // Click Rotate API key disclosure.
    await page.getByRole('button', { name: 'Rotate API key' }).click()
    // The label isn't wired to the input via `for=`, so locate by the
    // placeholder hint instead (matches `sk-...` for OpenAI).
    const newKeyInput = page.locator('input[type="password"][placeholder^="sk-"]')
    await expect(newKeyInput).toBeVisible()
    await newKeyInput.fill('sk-rotated-zzzz')

    // Save should be disabled until the test probe passes.
    const save = page.getByRole('button', { name: /^Save/ })
    await expect(save).toBeDisabled()

    // Make the probe always succeed. Don't put expect() inside the handler —
    // an exception there is swallowed by Playwright and the fulfill returns
    // `undefined`. Assert on the captured body afterwards instead.
    backend.testHandler = () => ({ ok: true, provider: 'openai', detail: 'authorized' })
    await Promise.all([
      page.waitForResponse(
        (r) => r.url().includes('/llm-providers/test') && r.status() === 200,
      ),
      page.getByRole('button', { name: /Test now|Testing/ }).click(),
    ])
    const probeBody = backend.lastBody['POST /llm-providers/test'] as Record<string, unknown>
    expect(probeBody.api_key).toBe('sk-rotated-zzzz')
    await expect(page.getByText(/Connection OK/)).toBeVisible()
    await expect(save).toBeEnabled()

    await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().includes(`/orgs/${ORG_ID}/llm-providers/p-openai`) &&
          r.request().method() === 'PUT',
      ),
      save.click(),
    ])
    const putBody = backend.lastBody['PUT /llm-providers/:id'] as Record<string, unknown>
    expect(putBody.api_key).toBe('sk-rotated-zzzz')

    // Reload — api_key_hint should reflect the new prefix (sk-ro…).
    await page.reload()
    const updated = backend.find('p-openai')
    expect(updated?.api_key_hint?.startsWith('sk-ro')).toBe(true)
  })

  test('6) Edit base_url without rotate — probe body uses {provider_id} shape', async ({ page, context }) => {
    // Only `custom` / `ollama` show base_url in the Edit dialog, so seed
    // a custom provider with an editable base URL.
    const backend = new MockBackend([
      makeProvider({
        id: 'p-custom',
        provider: 'custom',
        display_name: 'Local vLLM',
        base_url: 'http://localhost:8000/v1',
        api_key_hint: 'sk-...local',
        is_default: true,
        config: { model: 'custom' },
      }),
    ])
    await seedSession(page, context)
    await installProviderRoutes(context, backend)

    await gotoProviders(page)
    await page.getByTestId('edit-credentials-btn').click()
    // Labels aren't `for=`-wired; locate the Base URL `<input type="url">`
    // that lives in the Edit dialog.
    const baseUrlInput = page.locator('input[type="url"]')
    await expect(baseUrlInput).toBeVisible()
    await baseUrlInput.fill('http://localhost:9000/v1')

    backend.testHandler = () => ({ ok: true, provider: 'custom', detail: 'ok' })
    await Promise.all([
      page.waitForResponse(
        (r) => r.url().includes('/llm-providers/test') && r.status() === 200,
      ),
      page.getByRole('button', { name: /Test now|Testing/ }).click(),
    ])
    // The exact assertion Issue F #6 requires: when base_url is the only
    // change, the probe must use {provider_id} (no inline api_key).
    const probeBody = backend.lastBody['POST /llm-providers/test'] as Record<string, unknown>
    expect(probeBody.provider_id).toBe('p-custom')
    expect('api_key' in probeBody).toBe(false)
    await expect(page.getByText(/Connection OK/)).toBeVisible()
    await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().includes(`/orgs/${ORG_ID}/llm-providers/p-custom`) &&
          r.request().method() === 'PUT',
      ),
      page.getByRole('button', { name: /^Save/ }).click(),
    ])
    // PUT should NOT carry api_key.
    const putBody = backend.lastBody['PUT /llm-providers/:id'] as Record<string, unknown>
    expect('api_key' in putBody).toBe(false)
    expect(putBody.base_url).toBe('http://localhost:9000/v1')
  })

  test('7) PUT omits api_key when user did not click Rotate', async ({ page, context }) => {
    const backend = new MockBackend([
      makeProvider({
        id: 'p-openai',
        display_name: 'OpenAI Primary',
        is_default: true,
        api_key_hint: 'sk-...abcd',
      }),
    ])
    await seedSession(page, context)
    await installProviderRoutes(context, backend)

    await gotoProviders(page)
    await page.getByTestId('edit-credentials-btn').click()
    // OpenAI doesn't show Base URL in the Edit dialog, so the cheap
    // change here is display_name via the dialog itself.
    // Labels aren't `for=`-wired; the Display Name field is the only
    // required text input in the Edit dialog.
    const displayName = page.locator('input[type="text"][required]')
    await displayName.fill('OpenAI Primary (renamed via dialog)')
    // No gate is required because neither base_url nor a new key changed.
    const save = page.getByRole('button', { name: /^Save/ })
    await expect(save).toBeEnabled()
    await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().includes(`/orgs/${ORG_ID}/llm-providers/p-openai`) &&
          r.request().method() === 'PUT',
      ),
      save.click(),
    ])
    const putBody = backend.lastBody['PUT /llm-providers/:id'] as Record<string, unknown>
    expect('api_key' in putBody).toBe(false)
    expect(putBody.display_name).toBe('OpenAI Primary (renamed via dialog)')
  })

  test('8) Menu reachability — header avatar + sidebar link both land on /llm-providers', async ({
    page,
    context,
  }) => {
    const backend = new MockBackend([
      makeProvider({ id: 'p-openai', display_name: 'OpenAI Primary', is_default: true }),
    ])
    await seedSession(page, context)
    await installProviderRoutes(context, backend)
    // Empty /me + workspaces routes so dashboard renders happily.
    await context.route('**/api/v1/me', (route) =>
      route.fulfill({ status: 200, json: { org_id: ORG_ID, id: 'u-1', email: 'e2e@raven.local' } }),
    )
    await context.route(`**/api/v1/orgs/${ORG_ID}/workspaces`, (route) =>
      route.fulfill({ status: 200, json: [] }),
    )

    await page.goto('/dashboard')

    // Header avatar → AI Providers
    await page.getByRole('button', { name: 'User menu' }).click()
    await page.getByRole('menuitem', { name: 'AI Providers' }).click()
    await expect(page).toHaveURL(/\/llm-providers$/)

    // Back to /dashboard, then take the sidebar route.
    await page.goto('/dashboard')
    await page.getByRole('link', { name: 'AI Providers' }).click()
    await expect(page).toHaveURL(/\/llm-providers$/)
  })
})
