import { test, expect } from '../fixtures'

test.describe('WhatsApp Integration', () => {
  test('view incoming webhook events', async ({ adminPage: page }) => {
    await page.goto('/whatsapp/events')
    await expect(page.getByTestId('events-list')).toBeVisible()
  })

  test('trigger test callback endpoint', async ({ adminPage: page }) => {
    await page.goto('/whatsapp/settings')
    await page.getByRole('button', { name: 'Test Callback' }).click()
    await expect(page.getByTestId('callback-result')).toBeVisible({ timeout: 10000 })
    await expect(page.getByTestId('callback-result')).toHaveText(/success|200|ok/i)
  })

  test('view webhook delivery status', async ({ adminPage: page }) => {
    await page.goto('/whatsapp/events')
    const events = await page.getByTestId('event-row').all()
    if (events.length === 0) {
      test.skip(true, 'No events available to verify delivery status')
      return
    }
    await expect(page.getByTestId('delivery-status').first()).toBeVisible()
  })
})
