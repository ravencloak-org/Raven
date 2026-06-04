/**
 * M14 — Browser without WebAuthn support (Issue #776, scenario 5).
 *
 * Flow:
 *  1. Inject a stub into the page BEFORE any module loads so that the
 *     supertokens-web-js Webauthn recipe sees `doesBrowserSupportWebAuthn`
 *     returning false (we delete the WebAuthn entry on `navigator.credentials`
 *     and `window.PublicKeyCredential`).
 *  2. On the Login page, the passkey button must be disabled with a tooltip.
 *  3. In Settings → Authentication, the Add button must be hidden OR show
 *     an explanatory "not supported" note.
 */
import { BrowserContext } from '@playwright/test'
import { test, expect } from '../fixtures'
import {
  PasskeyBackend,
  installPasskeyRoutes,
  seedSession,
  skipIfUIMissing,
} from './passkeyMocks'

/**
 * Stub the browser-level WebAuthn surface that
 * `doesBrowserSupportWebAuthn()` probes. Runs as an init script so it lands
 * before any page module reads these globals.
 */
async function disableWebAuthn(context: BrowserContext) {
  await context.addInitScript(() => {
    // Remove PublicKeyCredential entirely.
    try {
      // @ts-expect-error — runtime override
      delete window.PublicKeyCredential
    } catch {
      Object.defineProperty(window, 'PublicKeyCredential', { value: undefined, configurable: true })
    }
    // Strip the WebAuthn `create` / `get` from navigator.credentials.
    try {
      if (navigator.credentials) {
        Object.defineProperty(navigator, 'credentials', {
          value: undefined,
          configurable: true,
        })
      }
    } catch {
      /* ignore */
    }
  })
}

test.describe('M14 Passkeys — Unsupported browser', () => {
  test('login button disabled and settings hides Add', async ({
    page,
    context,
  }, testInfo) => {
    const backend = new PasskeyBackend([])
    await seedSession(page, context)
    await installPasskeyRoutes(context, backend)
    await disableWebAuthn(context)

    // --- Login page ---
    await page.goto('/login')
    await skipIfUIMissing(
      page,
      testInfo,
      'signin-passkey-btn',
      'M14 Issues C/D (#773, #774) not yet merged',
    )
    const loginBtn = page.getByTestId('signin-passkey-btn')
    await expect(loginBtn).toBeVisible()
    await expect(loginBtn).toBeDisabled()
    // Tooltip text from the spec acceptance criteria.
    const wrapper = page.getByTestId('signin-passkey-btn-wrapper').or(loginBtn)
    await expect(wrapper).toHaveAttribute(
      'title',
      /not support|unsupported|passkey/i,
    )

    // --- Settings → Authentication tab ---
    await page.goto('/settings')
    await page.getByTestId('tab-authentication').click()
    // Either: Add button is absent…
    const addBtn = page.getByTestId('add-passkey-btn')
    const notSupported = page.getByTestId('passkeys-not-supported')
    // …or it's rendered with a not-supported note. Accept either shape.
    const addHidden = (await addBtn.count()) === 0
    const noteVisible = (await notSupported.count()) > 0
    expect(addHidden || noteVisible).toBeTruthy()
  })
})
