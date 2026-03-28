import { test, expect } from '@playwright/test'

test('org detail page shows org name when API returns data', async ({ page }) => {
  // Mock the API response
  await page.route('**/api/v1/orgs/**', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ id: 'org-1', name: 'Acme Corp', slug: 'acme-corp', status: 'active', settings: {}, created_at: '', updated_at: '' }),
    })
  )
  await page.goto('/orgs/org-1')
  await expect(page.getByRole('heading', { name: 'Acme Corp' })).toBeVisible({ timeout: 5000 })
})
