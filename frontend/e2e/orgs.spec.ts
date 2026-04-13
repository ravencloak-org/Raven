import { test, expect } from '@playwright/test'

// Abort Zitadel OIDC requests so pages render without auth redirect
test.beforeEach(async ({ page }) => {
  await page.route('**/oauth/**', (route) => route.abort())
  await page.route('**/.well-known/**', (route) => route.abort())
})

test('privacy policy page renders', async ({ page }) => {
  await page.goto('/legal/privacy')
  await expect(page.getByRole('heading', { name: 'Privacy Policy' })).toBeVisible({ timeout: 15000 })
})

test('terms of service page renders', async ({ page }) => {
  await page.goto('/legal/terms')
  await expect(page.getByRole('heading', { name: 'Terms of Service' })).toBeVisible({ timeout: 15000 })
})
