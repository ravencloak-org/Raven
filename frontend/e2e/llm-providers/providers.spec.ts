import { test, expect } from '../fixtures'

test.describe('LLM Providers', () => {
  test('add OpenAI BYOK config', async ({ adminPage: page }) => {
    await page.goto('/llm-providers')
    await page.getByRole('button', { name: 'Add Provider' }).click()
    await page.getByLabel('Provider').selectOption('openai')
    await page.getByLabel('API Key').fill(process.env.E2E_OPENAI_API_KEY ?? 'sk-test-fake-key-for-e2e')
    await page.getByRole('button', { name: 'Save' }).click()
    await expect(page.getByText('openai')).toBeVisible()
  })

  test('list configured providers', async ({ adminPage: page }) => {
    await page.goto('/llm-providers')
    await expect(page.getByTestId('providers-list')).toBeVisible()
  })

  test('remove a provider', async ({ adminPage: page }) => {
    await page.goto('/llm-providers')
    const providerCount = await page.getByTestId('provider-row').count()
    if (providerCount > 0) {
      await page.getByTestId('provider-row').first().getByRole('button', { name: 'Remove' }).click()
      await page.getByRole('button', { name: 'Confirm' }).click()
      await expect(page.getByTestId('provider-row')).toHaveCount(providerCount - 1)
    }
  })
})
