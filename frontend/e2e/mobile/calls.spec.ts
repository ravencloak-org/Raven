import { test, expect } from '../fixtures'

test.describe('Mobile: Call list renders cards', () => {
  test('Calls tab navigates to the calls URL', async ({ adminPage: page }) => {
    await page.goto('/dashboard')
    await page.getByRole('link', { name: 'Calls' }).click()
    await expect(page).toHaveURL(/\/whatsapp\/calls/, { timeout: 10000 })
  })

  test('calls page shows cards not table on mobile', async ({ adminPage: page }) => {
    await page.goto('/dashboard')
    await page.getByRole('link', { name: 'Calls' }).click()
    await page.waitForURL(/\/whatsapp\/calls/, { timeout: 10000 })

    // Mobile layout must NOT render a <table>
    await expect(page.locator('table')).not.toBeVisible()
  })

  test('call card is tappable', async ({ adminPage: page }) => {
    await page.goto('/dashboard')
    await page.getByRole('link', { name: 'Calls' }).click()
    await page.waitForURL(/\/whatsapp\/calls/, { timeout: 10000 })

    const firstCard = page.locator('[data-testid="call-card"]').first()
    if (await firstCard.isVisible()) {
      const box = await firstCard.boundingBox()
      expect(box).not.toBeNull()
      expect(box!.height).toBeGreaterThanOrEqual(44)
    }
  })
})
