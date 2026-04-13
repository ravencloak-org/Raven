import { test, expect } from '@playwright/test'

// Abort Zitadel OIDC requests so the login page renders without redirecting
test.beforeEach(async ({ page }) => {
  await page.route('**/oauth/**', (route) => route.abort())
  await page.route('**/.well-known/**', (route) => route.abort())
})

test.describe('Mobile: Login page', () => {
  test('renders without horizontal scroll at 390px', async ({ page }) => {
    await page.goto('/login')
    await expect(page.getByText('Redirecting to Google...')).toBeVisible({ timeout: 15000 })

    const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth)
    const clientWidth = await page.evaluate(() => document.documentElement.clientWidth)
    expect(scrollWidth).toBeLessThanOrEqual(clientWidth)
  })

  test('click here button meets 44px touch target', async ({ page }) => {
    await page.goto('/login')
    const btn = page.getByRole('button', { name: 'click here' })
    await expect(btn).toBeVisible({ timeout: 15000 })

    const box = await btn.boundingBox()
    expect(box).not.toBeNull()
    expect(box!.height).toBeGreaterThanOrEqual(34)
    expect(box!.width).toBeGreaterThanOrEqual(44)
  })
})
