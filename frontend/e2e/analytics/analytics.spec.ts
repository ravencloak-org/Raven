import { test, expect } from '../fixtures'

test.describe('Analytics', () => {
  test('view usage dashboard with date filter', async ({ adminPage: page }) => {
    await page.goto('/analytics')
    await expect(page.getByTestId('usage-chart')).toBeVisible()
    await page.getByLabel('Date Range').selectOption('last_7_days')
    await expect(page.getByTestId('usage-chart')).toBeVisible()
  })

  test('export analytics data', async ({ adminPage: page }) => {
    await page.goto('/analytics')
    // Intercept the download
    const downloadPromise = page.waitForEvent('download')
    await page.getByRole('button', { name: 'Export' }).click()
    const download = await downloadPromise
    expect(download.suggestedFilename()).toMatch(/analytics.*\.(csv|json|xlsx)/)
  })
})
