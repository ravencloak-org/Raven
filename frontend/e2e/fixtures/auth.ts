import { test as base, type Page } from '@playwright/test'

export type AuthFixtures = {
  authenticatedPage: Page
  adminPage: Page
}

// Keycloak test credentials — set in .env.test or CI secrets
const TEST_USER = process.env.E2E_USER ?? 'testuser@example.com'
const TEST_PASS = process.env.E2E_PASS ?? 'testpassword'
const TEST_ADMIN = process.env.E2E_ADMIN ?? 'admin@example.com'
const TEST_ADMIN_PASS = process.env.E2E_ADMIN_PASS ?? 'adminpassword'

export async function loginAs(page: Page, email: string, password: string) {
  await page.goto('/')
  // Wait for Keycloak redirect
  await page.waitForURL(/\/realms\/raven\/protocol\/openid-connect\/auth/)
  await page.getByLabel('Email').fill(email)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Sign In' }).click()
  // Wait for redirect back to app
  await page.waitForURL('/')
  await page.waitForSelector('[data-testid="dashboard"]', { timeout: 10000 })
}

export const test = base.extend<AuthFixtures>({
  authenticatedPage: async ({ page }, use) => {
    await loginAs(page, TEST_USER, TEST_PASS)
    await use(page)
  },
  adminPage: async ({ page }, use) => {
    await loginAs(page, TEST_ADMIN, TEST_ADMIN_PASS)
    await use(page)
  },
})

export { expect } from '@playwright/test'
