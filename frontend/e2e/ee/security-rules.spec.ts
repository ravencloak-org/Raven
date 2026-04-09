import { test, expect } from '../fixtures'

test.describe('Security Rules (EE)', () => {
  test('create a block rule', async ({ adminPage: page }) => {
    await page.goto('/settings/security-rules')
    await page.getByRole('button', { name: 'Add Rule' }).click()
    await page.getByLabel('Pattern').fill('DROP TABLE')
    await page.getByLabel('Action').selectOption('block')
    await page.getByRole('button', { name: 'Save Rule' }).click()
    await expect(page.getByText('DROP TABLE')).toBeVisible()
  })
})
