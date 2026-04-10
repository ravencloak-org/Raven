import { test, expect } from '../fixtures'

test.describe('API Keys', () => {
  test('create workspace-scoped key', async ({ authenticatedPage: page }) => {
    await page.goto('/api-keys')
    await page.getByRole('button', { name: 'Create Key' }).click()
    await page.getByLabel('Scope').selectOption('workspace')
    await page.getByRole('button', { name: 'Generate' }).click()
    await expect(page.getByTestId('api-key-value')).toBeVisible()
  })

  test('create KB-scoped key', async ({ authenticatedPage: page }) => {
    await page.goto('/api-keys')
    await page.getByRole('button', { name: 'Create Key' }).click()
    await page.getByLabel('Scope').selectOption('knowledge_base')
    await page.getByLabel('Knowledge Base').selectOption({ index: 0 })
    await page.getByRole('button', { name: 'Generate' }).click()
    await expect(page.getByTestId('api-key-value')).toBeVisible()
  })

  test('revoke key removes it from list', async ({ authenticatedPage: page }, testInfo) => {
    await page.goto('/api-keys')
    const keyCount = await page.getByTestId('api-key-row').count()
    testInfo.skip(keyCount === 0, 'No API keys exist to revoke — run the create key tests first')
    await page.getByTestId('api-key-row').first().getByRole('button', { name: 'Revoke' }).click()
    await page.getByRole('button', { name: 'Revoke Key' }).click()
    await expect(page.getByTestId('api-key-row')).toHaveCount(keyCount - 1)
  })
})
