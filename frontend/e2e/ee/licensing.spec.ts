import { test, expect } from '../fixtures'

test.describe('Licensing (EE)', () => {
  test.beforeEach(async ({}, testInfo) => {
    testInfo.skip(!process.env.E2E_USER, 'Set E2E_USER to run EE licensing tests')
  })

  test('EE feature is accessible with valid license', async ({ adminPage: page }) => {
    await page.goto('/settings/security-rules')
    await expect(page.getByTestId('security-rules-panel')).toBeVisible()
  })

  // This test assumes an authenticated non-EE session (no enterprise license).
  // The page fixture provides an unauthenticated browser context, relying on
  // the default tenant being non-EE.
  test('EE feature shows upgrade prompt without license', async ({ page }) => {
    await page.goto('/settings/security-rules')
    await expect(page.getByText(/Upgrade|Enterprise/i)).toBeVisible()
  })
})
