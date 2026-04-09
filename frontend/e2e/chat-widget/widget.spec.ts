import { test, expect } from '@playwright/test'

test.describe('Chat Widget', () => {
  test('valid API key: widget loads and accepts messages', async ({ page }) => {
    await page.goto('/e2e/chat-widget/widget-sandbox.html')
    // Wait for web component to register
    await page.waitForSelector('raven-chat', { timeout: 10000 })
    // Widget should show chat input
    const shadowInput = page.locator('raven-chat').locator('css=input[type="text"]')
    await shadowInput.fill('Hello from widget test')
    await shadowInput.press('Enter')
    // Wait for response in shadow DOM
    const assistantMessage = page.locator('raven-chat').locator('css=[data-role="assistant"]')
    await expect(assistantMessage.first()).toBeVisible({ timeout: 10000 })
  })

  test('invalid API key: widget shows error state, not blank or crash', async ({ page }) => {
    // Serve sandbox with invalid key
    await page.goto('/e2e/chat-widget/widget-sandbox-invalid-key.html')
    await page.waitForSelector('raven-chat', { timeout: 10000 })
    const errorEl = page.locator('raven-chat').locator('css=[data-testid="error-state"]')
    await expect(errorEl).toBeVisible({ timeout: 8000 })
    const errorText = await errorEl.innerText()
    expect(errorText).toMatch(/invalid|unauthorized|error/i)
  })
})
