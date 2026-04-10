import { test, expect } from '../fixtures'

test.describe('Mobile: Bottom tab bar navigation', () => {
  test('tab bar renders on mobile dashboard', async ({ adminPage: page }) => {
    await page.goto('/dashboard')
    // Tab bar nav contains tab labels
    const tabBar = page.locator('nav').filter({ hasText: 'Home' })
    await expect(tabBar).toBeVisible({ timeout: 10000 })
  })

  test('each primary tab meets 44px touch target', async ({ adminPage: page }) => {
    await page.goto('/dashboard')
    const tabs = [
      page.getByRole('link', { name: 'Home' }),
      page.getByRole('link', { name: 'Voice' }),
      page.getByRole('link', { name: 'Calls' }),
      page.getByRole('link', { name: 'Numbers' }),
      page.getByRole('button', { name: 'More' }),
    ]
    for (const tab of tabs) {
      const box = await tab.first().boundingBox()
      expect(box, `tab "${await tab.first().textContent()}" bounding box`).not.toBeNull()
      expect(box!.height).toBeGreaterThanOrEqual(44)
    }
  })

  test('"More" button opens bottom sheet with secondary nav items', async ({ adminPage: page }) => {
    await page.goto('/dashboard')
    await page.getByRole('button', { name: 'More' }).click()
    await expect(page.getByText('Knowledge Bases')).toBeVisible({ timeout: 5000 })
    await expect(page.getByText('API Keys')).toBeVisible()
  })
})
