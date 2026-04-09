import { test as base, type Page } from '@playwright/test'

export type AuthFixtures = {
  authenticatedPage: Page
  adminPage: Page
}

export async function loginAs(page: Page, email: string, password: string) {
  await page.goto('/')
  // Wait for Keycloak redirect
  await page.waitForURL(/\/realms\/raven\/protocol\/openid-connect\/auth/)
  await page.getByLabel('Email').fill(email)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Sign In' }).click()
  // Wait for redirect back to app
  await page.waitForURL('/dashboard')
  await page.waitForSelector('[data-testid="dashboard"]', { timeout: 10000 })
}

export const test = base.extend<AuthFixtures>({
  authenticatedPage: async ({ page }, use, testInfo) => {
    if (!process.env.E2E_USER) {
      testInfo.skip(true, 'E2E_USER not configured — skipping auth-required test')
      await use(page)
      return
    }
    await loginAs(page, process.env.E2E_USER!, process.env.E2E_PASS!)
    await use(page)
  },
  adminPage: async ({ page }, use, testInfo) => {
    if (!process.env.E2E_ADMIN) {
      testInfo.skip(true, 'E2E_ADMIN not configured — skipping admin test')
      await use(page)
      return
    }
    await loginAs(page, process.env.E2E_ADMIN!, process.env.E2E_ADMIN_PASS!)
    await use(page)
  },
})

export { expect } from '@playwright/test'
