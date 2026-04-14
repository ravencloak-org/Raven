import { test, expect } from '@playwright/test'

test.beforeEach(async ({ page }) => {
  // Block SuperTokens API calls so login page renders without redirecting
  await page.route('**/auth/**', (route) => route.abort())
})

test('login page shows sign in button', async ({ page }) => {
  await page.goto('/login')
  await expect(page.getByRole('button', { name: 'Sign in with Google' })).toBeVisible({ timeout: 15000 })
})

test('login page shows Raven branding', async ({ page }) => {
  await page.goto('/login')
  await expect(page.getByRole('heading', { name: 'Raven' })).toBeVisible({ timeout: 15000 })
})
