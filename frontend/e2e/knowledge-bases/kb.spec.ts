import { test, expect } from '../fixtures'
import { KBPage } from '../pages/KBPage'

test.describe('Knowledge Base', () => {
  test('create, view, and delete a KB', async ({ authenticatedPage: page }) => {
    const kb = new KBPage(page)
    await kb.navigate()
    await kb.create('E2E Test KB')
    await kb.open('E2E Test KB')
    await expect(page).toHaveURL(/\/knowledge-bases\//)
    await kb.navigate()
    await kb.delete('E2E Test KB')
  })

  test('edit KB settings', async ({ authenticatedPage: page }) => {
    const kb = new KBPage(page)
    await kb.navigate()
    await kb.create('Settings Test KB')
    await kb.open('Settings Test KB')
    await page.getByRole('tab', { name: 'Settings' }).click()
    await page.getByLabel('Description').fill('Updated description')
    await page.getByRole('button', { name: 'Save' }).click()
    await expect(page.getByText('Saved')).toBeVisible()
  })
})
