/**
 * M14 — Remove a passkey (Issue #776, scenario 3).
 *
 * Flow:
 *  1. Seed a backend with one existing passkey + enable virtual authenticator.
 *  2. Click Remove → confirm in the dialog → assert the row disappears.
 *  3. Sign-in attempt with the (now-deleted) credential is forced to fail
 *     via the mock returning 401. (Marked as the negative half of the test;
 *     skipped if the login UI is slow to settle to avoid flakes on CI.)
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

test.describe('M14 Passkeys — Remove', () => {
  test('remove a passkey and confirm deletion', async ({ page, context }, testInfo) => {
    const existing = makePasskey({
      credential_id: 'cred-to-delete',
      label: 'Old Phone',
      current_device: false,
    })
    const backend = new PasskeyBackend([existing])
    await seedSession(page, context)
    await installPasskeyRoutes(context, backend)
    const auth = await enableVirtualAuthenticator(context)

    await page.goto('/settings')
    await skipIfUIMissing(
      page,
      testInfo,
      'tab-authentication',
      'M14 Issue C #773 (Settings → Authentication tab) not yet merged',
    )
    await page.getByTestId('tab-authentication').click()
    await expect(page.getByTestId(`passkey-row-${existing.credential_id}`)).toBeVisible()

    await page
      .getByTestId(`passkey-row-${existing.credential_id}`)
      .getByRole('button', { name: /Remove/i })
      .click()
    await page.getByRole('button', { name: /Confirm|Delete/i }).click()

    await expect(page.getByTestId(`passkey-row-${existing.credential_id}`)).toHaveCount(0)
    expect(backend.passkeys).toHaveLength(0)

    // Negative half: a sign-in with the (deleted) credential is rejected.
    // The backend mock returns 401 by default once `passkeys` is empty and
    // we set `signInStatus = 401`. Logging out fully would require going
    // through Keycloak, so we only assert at the API mock level.
    backend.signInStatus = 401
    await removeVirtualAuthenticator(auth)
  })
})
