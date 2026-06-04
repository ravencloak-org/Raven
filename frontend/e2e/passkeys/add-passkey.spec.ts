/**
 * M14 — Add passkey from Settings → Authentication (Issue #776, scenario 1).
 *
 * Flow:
 *  1. Single-user session is pre-seeded; passkey list starts empty.
 *  2. Virtual WebAuthn authenticator is enabled on the BrowserContext before
 *     navigating, so the registration ceremony completes silently.
 *  3. Navigate to /settings → Authentication tab → click `add-passkey-btn`.
 *  4. The ceremony runs; the new row appears in the list with an
 *     auto-generated label (UA-parsed default, e.g. "Chrome on macOS").
 *
 * Backend, Pinia store, settings tab and login button (Issues A/B/C/D) are
 * landed in their own PRs — this spec assumes the data-test attributes from
 * the spec doc: `tab-authentication`, `panel-authentication`,
 * `add-passkey-btn`, `passkey-row-{id}`, `passkey-label-{id}`.
 */
import { test, expect } from '../fixtures'
import { enableVirtualAuthenticator, removeVirtualAuthenticator } from './virtualAuthenticator'
import {
  PasskeyBackend,
  installPasskeyRoutes,
  seedSession,
  skipIfUIMissing,
} from './passkeyMocks'

test.describe('M14 Passkeys — Add', () => {
  test('add a passkey from Settings → Authentication', async ({
    page,
    context,
  }, testInfo) => {
    const backend = new PasskeyBackend([])
    await seedSession(page, context)
    await installPasskeyRoutes(context, backend)

    // Authenticator MUST be enabled before the registration page renders.
    const auth = await enableVirtualAuthenticator(context)

    await page.goto('/settings')
    await skipIfUIMissing(
      page,
      testInfo,
      'tab-authentication',
      'M14 Issue C #773 (Settings → Authentication tab) not yet merged',
    )
    await page.getByTestId('tab-authentication').click()
    await expect(page.getByTestId('panel-authentication')).toBeVisible()

    // Empty state visible before adding.
    await expect(page.getByText(/No passkeys yet/i)).toBeVisible()

    await page.getByTestId('add-passkey-btn').click()

    // Wait for the new row to appear. The label is auto-generated from the
    // UA (default: "Chrome on macOS" inside Playwright's Chromium).
    const newRow = page.locator('[data-test^="passkey-row-"]').first()
    await expect(newRow).toBeVisible({ timeout: 10_000 })
    await expect(newRow).toContainText(/Chrome|Chromium|macOS|Linux|Windows/i)

    // Backend received the persistence call.
    expect(backend.passkeys.length).toBeGreaterThanOrEqual(1)

    await removeVirtualAuthenticator(auth)
  })
})
