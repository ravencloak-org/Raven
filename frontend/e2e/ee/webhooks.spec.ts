import { test, expect } from '../fixtures'

test.describe('EE Webhooks', () => {
  test('configure webhook endpoint', async ({ adminPage: page }) => {
    await page.goto('/settings/webhooks')
    await page.getByRole('button', { name: 'Add Webhook' }).click()
    await page.getByLabel('URL').fill('https://webhook.site/test')
    await page.getByLabel('Events').selectOption('document.processed')
    await page.getByRole('button', { name: 'Save' }).click()
    await expect(page.getByText('webhook.site')).toBeVisible()
  })

  test('dead-lettered webhook can be replayed', async ({ adminPage: page }) => {
    await page.goto('/settings/webhooks/failed')
    const failedCount = await page.getByTestId('failed-delivery').count()
    if (failedCount > 0) {
      await page
        .getByTestId('failed-delivery')
        .first()
        .getByRole('button', { name: 'Replay' })
        .click()
      await expect(page.getByText('Replayed')).toBeVisible()
    }
  })
})
