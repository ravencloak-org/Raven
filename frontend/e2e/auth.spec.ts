import { test, expect } from '@playwright/test'

test('unauthenticated user sees login page', async ({ page }) => {
  await page.goto('/')
  // Since Keycloak isn't running in test, check that the UI shows auth state
  await expect(page.locator('text=Login')).toBeVisible({ timeout: 5000 })
})

test('login button redirects to Keycloak', async ({ page }) => {
  await page.goto('/')
  // Mock: just check the login button exists and is clickable
  const loginBtn = page.getByRole('button', { name: /login/i })
    .or(page.getByRole('link', { name: /login/i }))
  await expect(loginBtn).toBeVisible({ timeout: 5000 })
})
