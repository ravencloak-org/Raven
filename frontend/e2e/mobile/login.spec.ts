import { test, expect } from '@playwright/test'

// Abort Keycloak requests so keycloak.init() resolves as unauthenticated
test.beforeEach(async ({ page }) => {
  await page.route('**/realms/**', (route) => route.abort())
})

test.describe('Mobile: Login page', () => {
  test('renders without horizontal scroll at 390px', async ({ page }) => {
    await page.goto('/login')
    await expect(page.getByRole('button', { name: 'Login' })).toBeVisible({ timeout: 15000 })

    const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth)
    const clientWidth = await page.evaluate(() => document.documentElement.clientWidth)
    expect(scrollWidth).toBeLessThanOrEqual(clientWidth)
  })

  test('login button meets 44px touch target', async ({ page }) => {
    await page.goto('/login')
    const btn = page.getByRole('button', { name: 'Login' })
    await expect(btn).toBeVisible({ timeout: 15000 })

    const box = await btn.boundingBox()
    expect(box).not.toBeNull()
    expect(box!.height).toBeGreaterThanOrEqual(44)
    expect(box!.width).toBeGreaterThanOrEqual(44)
  })
})
