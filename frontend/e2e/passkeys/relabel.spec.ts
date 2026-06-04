/**
 * M14 — Relabel a passkey inline (Issue #776, scenario 2).
 *
 * Flow:
 *  1. Seed a backend with one existing passkey.
 *  2. Navigate to Settings → Authentication.
 *  3. Click the label, edit to a new value, blur to commit.
 *  4. Reload the page; assert the new label is rendered (persisted via
 *     PATCH /api/v1/me/passkeys/:id which the mock applies in-memory).
 */
import { test, expect } from '../fixtures'
import {
  PasskeyBackend,
  installPasskeyRoutes,
  makePasskey,
  seedSession,
  skipIfUIMissing,
} from './passkeyMocks'

// TODO(#787): unskip once PasskeysSection.vue exposes per-credential
// `passkey-label-{id}` and `passkey-label-input-{id}` testids.
test.describe.skip('M14 Passkeys — Relabel', () => {
  test('inline rename persists across reload', async ({ page, context }, testInfo) => {
    const existing = makePasskey({
      credential_id: 'cred-existing-1',
      label: 'MacBook Pro',
      current_device: true,
    })
    const backend = new PasskeyBackend([existing])
    await seedSession(page, context)
    await installPasskeyRoutes(context, backend)

    await page.goto('/settings')
    await skipIfUIMissing(
      page,
      testInfo,
      'tab-authentication',
      'M14 Issue C #773 (Settings → Authentication tab) not yet merged',
    )
    await page.getByTestId('tab-authentication').click()

    const labelLocator = page.getByTestId(`passkey-label-${existing.credential_id}`)
    await expect(labelLocator).toHaveText('MacBook Pro')

    await labelLocator.click()
    // The inline editor renders an input bound to the same data-test.
    const editor = page.getByTestId(`passkey-label-input-${existing.credential_id}`)
    await editor.fill('Work Laptop')
    await editor.blur()

    // Wait for PATCH to settle.
    await expect.poll(() => backend.passkeys[0]?.label).toBe('Work Laptop')

    // Reload — server returns the updated label.
    await page.reload()
    await page.getByTestId('tab-authentication').click()
    await expect(page.getByTestId(`passkey-label-${existing.credential_id}`)).toHaveText(
      'Work Laptop',
    )
  })
})
