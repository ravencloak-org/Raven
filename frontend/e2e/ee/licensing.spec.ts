import { test, expect } from '../fixtures'

test.describe('Licensing (EE)', () => {
  test('EE feature is accessible with valid license', async ({ adminPage: page }) => {
    await page.goto('/settings/security-rules')
    await expect(page.getByTestId('security-rules-panel')).toBeVisible()
  })

  test('EE feature shows upgrade prompt without license', async ({ page }) => {
    // Use a non-EE tenant
    await page.goto('/settings/security-rules')
    await expect(page.getByText(/Upgrade|Enterprise/i)).toBeVisible()
  })
})
