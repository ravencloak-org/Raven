import { test, expect } from '@playwright/test'

// Abort Keycloak requests so keycloak.init() resolves fast as unauthenticated
test.beforeEach(async ({ page }) => {
  await page.route('**/realms/**', (route) => route.abort())
})

test('login page shows sign in button', async ({ page }) => {
  await page.goto('/login')
  await expect(page.getByRole('button', { name: 'Login' })).toBeVisible({ timeout: 15000 })
})

test('login page shows Raven branding', async ({ page }) => {
  await page.goto('/login')
  await expect(page.getByRole('heading', { name: 'Sign in to Raven' })).toBeVisible({ timeout: 15000 })
})
