import { test, expect } from '../fixtures'

test.describe('Notifications', () => {
  test('create notification rule', async ({ adminPage: page }) => {
    await page.goto('/notifications')
    await page.getByRole('button', { name: 'New Rule' }).click()
    await page.getByLabel('Event').selectOption('document_processed')
    await page.getByLabel('Email').fill('notify@example.com')
    await page.getByRole('button', { name: 'Save' }).click()
    await expect(page.getByText('notify@example.com')).toBeVisible()
  })

  test('receive in-app notification after triggering event', async ({ adminPage: page }) => {
    // Upload a document to trigger 'document_processed' notification
    await page.goto('/knowledge-bases/test-kb/documents')
    await page.locator('input[type="file"]').setInputFiles({
      name: 'notify-test.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('notification trigger content'),
    })
    await page.getByRole('button', { name: 'Start Upload' }).click()
    // Wait for in-app notification badge or toast
    await expect(page.getByTestId('notification-badge')).toBeVisible({ timeout: 30000 })
  })
})
