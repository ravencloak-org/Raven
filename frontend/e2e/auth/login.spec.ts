import { test, expect } from '../fixtures'

test.describe('Authentication', () => {
  test('login via Keycloak SSO succeeds', async ({ page }) => {
    if (!process.env.E2E_USER || !process.env.E2E_PASS) {
      test.skip(true, 'E2E_USER/E2E_PASS not configured')
      return
    }
    await page.goto('/')
    await page.waitForURL(/\/realms\/raven\/protocol\/openid-connect\/auth/)
    await page.getByLabel('Email').fill(process.env.E2E_USER)
    await page.getByLabel('Password').fill(process.env.E2E_PASS)
    await page.getByRole('button', { name: 'Sign In' }).click()
    await page.waitForURL('/')
    await expect(page.getByTestId('dashboard')).toBeVisible()
  })

  test('logout clears session and redirects to login', async ({ authenticatedPage: page }) => {
    await page.getByTestId('user-menu').click()
    await page.getByRole('button', { name: 'Logout' }).click()
    await page.waitForURL(/\/realms\/raven\/protocol\/openid-connect\/auth/)
    await expect(page.getByLabel('Email')).toBeVisible()
  })

  test('session expiry redirects to login', async ({ page }) => {
    // Navigate to protected route without login
    await page.goto('/knowledge-bases')
    await page.waitForURL(/\/realms\/raven\/protocol\/openid-connect\/auth/)
    await expect(page.getByLabel('Email')).toBeVisible()
  })

  test('invalid credentials shows error', async ({ page }) => {
    await page.goto('/')
    await page.waitForURL(/\/realms\/raven\/protocol\/openid-connect\/auth/)
    await page.getByLabel('Email').fill('wrong@example.com')
    await page.getByLabel('Password').fill('wrongpass')
    await page.getByRole('button', { name: 'Sign In' }).click()
    await expect(page.getByText(/Invalid credentials|Login failed/i)).toBeVisible()
  })
})
