/**
 * M14 — Sign in with passkey from /login (Issue #776, scenario 4).
 *
 * Flow:
 *  1. Virtual authenticator is enabled BEFORE any navigation so the
 *     ceremony binds to the right CDP target.
 *  2. Seed a "registered" passkey in the backend so the sign-in endpoint
 *     returns OK.
 *  3. Navigate to /login (no session) → click `signin-passkey-btn`.
 *  4. WebAuthn assertion is auto-approved by the virtual authenticator →
 *     SuperTokens mock returns OK → frontend redirects to /dashboard.
 */
import { test, expect } from '../fixtures'
import { enableVirtualAuthenticator, removeVirtualAuthenticator } from './virtualAuthenticator'
import {
  PasskeyBackend,
  installPasskeyRoutes,
  makePasskey,
  seedSession,
  skipIfUIMissing,
} from './passkeyMocks'

// TODO(#787): unskip once LoginPage.vue uses `data-test` (not `data-testid`)
// for `signin-passkey-btn`, matching playwright.config.ts testIdAttribute.
test.describe.skip('M14 Passkeys — Sign in', () => {
  test('sign in with passkey lands on dashboard', async ({ page, context }, testInfo) => {
    const registered = makePasskey({ credential_id: 'cred-signin', label: 'My Laptop' })
    const backend = new PasskeyBackend([registered])
    await seedSession(page, context)
    await installPasskeyRoutes(context, backend)
    const auth = await enableVirtualAuthenticator(context)

    await page.goto('/login')
    await skipIfUIMissing(
      page,
      testInfo,
      'signin-passkey-btn',
      'M14 Issue D #774 (Login page passkey button) not yet merged',
    )
    const button = page.getByTestId('signin-passkey-btn')
    await expect(button).toBeVisible()
    await expect(button).toBeEnabled()
    await button.click()

    // Either Vue Router or SuperTokens drives the redirect. Accept any URL
    // that lands inside the authenticated app.
    await page.waitForURL(/\/(dashboard|workspaces|chat)/, { timeout: 15_000 })

    await removeVirtualAuthenticator(auth)
  })
})
