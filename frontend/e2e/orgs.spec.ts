import { test, expect } from '@playwright/test'

// Abort Keycloak requests so keycloak.init() resolves fast as unauthenticated
test.beforeEach(async ({ page }) => {
  await page.route('**/realms/**', (route) => route.abort())
})

test('privacy policy page renders', async ({ page }) => {
  await page.goto('/legal/privacy')
  await expect(page.getByRole('heading', { name: 'Privacy Policy' })).toBeVisible({ timeout: 15000 })
})

test('terms of service page renders', async ({ page }) => {
  await page.goto('/legal/terms')
  await expect(page.getByRole('heading', { name: 'Terms of Service' })).toBeVisible({ timeout: 15000 })
})
