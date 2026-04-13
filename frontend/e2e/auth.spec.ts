import { test, expect } from '@playwright/test'

// Abort Zitadel OIDC requests so the login page renders without redirecting
test.beforeEach(async ({ page }) => {
  await page.route('**/oauth/**', (route) => route.abort())
  await page.route('**/.well-known/**', (route) => route.abort())
})

test('login page shows redirect message', async ({ page }) => {
  await page.goto('/login')
  await expect(page.getByText('Redirecting to Google...')).toBeVisible({ timeout: 15000 })
})

test('login page shows fallback click here button', async ({ page }) => {
  await page.goto('/login')
  await expect(page.getByRole('button', { name: 'click here' })).toBeVisible({ timeout: 15000 })
})
