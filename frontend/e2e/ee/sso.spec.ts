import { test, expect } from '@playwright/test'

test.describe('SSO (EE)', () => {
  test('OIDC login redirects to Keycloak and back', async ({ page }) => {
    await page.goto('/')
    await page.waitForURL(/keycloak/)
    expect(page.url()).toContain('protocol/openid-connect/auth')
  })

  test('SSO-only org blocks password login', async ({ page }) => {
    // Navigate to org that enforces SSO-only
    await page.goto('/org-sso-only/login')
    await expect(page.getByLabel('Password')).not.toBeVisible()
    await expect(
      page.getByRole('button', { name: /Login with SSO|Sign in with/i }),
    ).toBeVisible()
  })
})
