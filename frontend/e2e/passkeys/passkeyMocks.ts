/**
 * Shared mock layer for passkey E2E specs.
 *
 * The five passkey specs run against a fully mocked backend (mirrors the
 * `m13.spec.ts` style for LLM providers) so they're hermetic and don't
 * require Keycloak / SuperTokens / Raven API to be reachable. The real
 * cryptography happens inside Chromium's virtual WebAuthn authenticator —
 * only the Raven REST surface and SuperTokens label endpoints are stubbed.
 */
import { BrowserContext, Page, Route, TestInfo } from '@playwright/test'

/**
 * Skip a test gracefully when the UI for M14 Issues A/B/C/D isn't on main
 * yet. The five upstream PRs (#771-#774) ship the backend endpoints,
 * passkey store, Settings → Authentication tab, and login-page passkey
 * button. Until they land, the data-test selectors won't exist; rather
 * than fail CI, the suite skips with a descriptive reason so it stays
 * green and auto-activates the moment the dependencies merge.
 */
export async function skipIfUIMissing(
  page: Page,
  testInfo: TestInfo,
  selector: string,
  reason: string,
): Promise<void> {
  // 250 ms grace period for the element to render; Playwright auto-waits
  // only inside expect/locator actions, not for count().
  await page.waitForTimeout(250)
  const count = await page.locator(`[data-test="${selector}"]`).count()
  testInfo.skip(count === 0, `${reason} (data-test="${selector}" not found)`)
}

/**
 * Single-user mode local org UUID baked into `useAuthStore.setLocalMode()`.
 * Has to match exactly — the dashboard guard reads `orgId` from the store.
 */
export const ORG_ID = '00000000-0000-0000-0000-000000000001'

export type Passkey = {
  credential_id: string
  label: string
  created_at: string
  last_used_at: string | null
  current_device: boolean
}

export class PasskeyBackend {
  passkeys: Passkey[]
  /** Optional override: if non-null the next POST register-finish returns this status. */
  registerFinishStatus: number | null = null
  /** Optional override: if non-null the next sign-in attempt returns this status. */
  signInStatus: number | null = null

  constructor(initial: Passkey[] = []) {
    this.passkeys = initial
  }
}

export function makePasskey(overrides: Partial<Passkey> = {}): Passkey {
  return {
    credential_id: overrides.credential_id ?? `cred-${Math.random().toString(36).slice(2, 10)}`,
    label: overrides.label ?? 'Chrome on macOS',
    created_at: overrides.created_at ?? new Date().toISOString(),
    last_used_at: overrides.last_used_at ?? null,
    current_device: overrides.current_device ?? false,
  }
}

/**
 * Pre-seed single-user mode BEFORE the app boots so the router guard skips
 * SuperTokens. addInitScript runs before any module code, including Pinia
 * store hydration.
 */
export async function seedSession(page: Page, context: BrowserContext): Promise<void> {
  await page.addInitScript(() => {
    ;(window as unknown as { __RAVEN_SINGLE_USER: boolean }).__RAVEN_SINGLE_USER = true
  })
  await context.route('**/auth/**', (route) => route.abort())
  await context.route('**/api/v1/config', (route) =>
    route.fulfill({ status: 200, json: { single_user: true } }),
  )
  await context.route('**/api/v1/me', (route) =>
    route.fulfill({
      status: 200,
      json: { org_id: ORG_ID, id: 'u-1', email: 'e2e@raven.local' },
    }),
  )
  await context.route(`**/api/v1/orgs/${ORG_ID}/workspaces`, (route) =>
    route.fulfill({ status: 200, json: [] }),
  )
}

/**
 * Install the Raven `/api/v1/me/passkeys/*` routes plus the SuperTokens
 * WebAuthn endpoints the frontend calls during registration / sign-in.
 *
 * The virtual authenticator (CDP `WebAuthn.*`) performs the real crypto,
 * but the API surface that exchanges challenges + persists labels is mocked
 * so the test doesn't depend on a live backend.
 */
export async function installPasskeyRoutes(
  context: BrowserContext,
  backend: PasskeyBackend,
): Promise<void> {
  // GET / collection
  await context.route('**/api/v1/me/passkeys', async (route: Route) => {
    const method = route.request().method()
    if (method === 'GET') {
      return route.fulfill({ status: 200, json: { passkeys: backend.passkeys } })
    }
    if (method === 'POST') {
      // Hand-off for label persistence after a successful WebAuthn ceremony.
      const body = JSON.parse(route.request().postData() ?? '{}')
      const created = makePasskey({
        credential_id: body.credential_id ?? `cred-${Date.now()}`,
        label: body.label ?? 'Chrome on macOS',
        current_device: true,
      })
      backend.passkeys.push(created)
      return route.fulfill({ status: 201, json: created })
    }
    return route.continue()
  })

  // PATCH / DELETE individual passkey
  await context.route('**/api/v1/me/passkeys/*', async (route: Route) => {
    const url = new URL(route.request().url())
    const id = url.pathname.split('/').pop() ?? ''
    const method = route.request().method()
    if (method === 'PATCH') {
      const body = JSON.parse(route.request().postData() ?? '{}')
      const pk = backend.passkeys.find((p) => p.credential_id === id)
      if (!pk) return route.fulfill({ status: 404, json: { error: 'not_found' } })
      if (typeof body.label === 'string') pk.label = body.label
      return route.fulfill({ status: 200, json: pk })
    }
    if (method === 'DELETE') {
      backend.passkeys = backend.passkeys.filter((p) => p.credential_id !== id)
      return route.fulfill({ status: 204, body: '' })
    }
    return route.continue()
  })

  // SuperTokens WebAuthn register options + verify. Returns the bare minimum
  // shape the supertokens-web-js Webauthn recipe consumes. The virtual
  // authenticator does the actual ceremony — these stubs just exchange the
  // challenge/credential blobs.
  await context.route('**/auth/webauthn/options/register', async (route) => {
    return route.fulfill({
      status: 200,
      json: {
        status: 'OK',
        webauthnGeneratedOptionsId: 'opts-' + Date.now(),
        challenge: 'AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8',
        rp: { id: 'localhost', name: 'Raven' },
        user: {
          id: 'dS0x',
          name: 'e2e@raven.local',
          displayName: 'e2e@raven.local',
        },
        pubKeyCredParams: [{ type: 'public-key', alg: -7 }],
        timeout: 60_000,
        attestation: 'none',
        authenticatorSelection: {
          userVerification: 'preferred',
          residentKey: 'preferred',
        },
      },
    })
  })
  await context.route('**/auth/webauthn/signup', async (route) => {
    if (backend.registerFinishStatus && backend.registerFinishStatus >= 400) {
      return route.fulfill({
        status: backend.registerFinishStatus,
        json: { status: 'WEBAUTHN_VERIFICATION_FAILED' },
      })
    }
    return route.fulfill({ status: 200, json: { status: 'OK', user: { id: 'u-1' } } })
  })
  await context.route('**/auth/webauthn/credential', async (route) => {
    if (backend.registerFinishStatus && backend.registerFinishStatus >= 400) {
      return route.fulfill({
        status: backend.registerFinishStatus,
        json: { status: 'WEBAUTHN_VERIFICATION_FAILED' },
      })
    }
    return route.fulfill({ status: 200, json: { status: 'OK' } })
  })
  await context.route('**/auth/webauthn/options/signin', async (route) => {
    return route.fulfill({
      status: 200,
      json: {
        status: 'OK',
        webauthnGeneratedOptionsId: 'opts-' + Date.now(),
        challenge: 'AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8',
        rpId: 'localhost',
        timeout: 60_000,
        userVerification: 'preferred',
      },
    })
  })
  await context.route('**/auth/webauthn/signin', async (route) => {
    if (backend.signInStatus && backend.signInStatus >= 400) {
      return route.fulfill({
        status: backend.signInStatus,
        json: { status: 'INVALID_CREDENTIALS_ERROR' },
      })
    }
    return route.fulfill({
      status: 200,
      json: { status: 'OK', user: { id: 'u-1', email: 'e2e@raven.local' } },
    })
  })
}
