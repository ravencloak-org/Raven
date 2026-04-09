import { test, expect } from '../fixtures'
import { KBPage } from '../pages/KBPage'

test.describe('Knowledge Base', () => {
  test('create, view, and delete a KB', async ({ authenticatedPage: page }, testInfo) => {
    const kb = new KBPage(page)
    const kbName = `E2E Test KB ${testInfo.workerIndex}-${Date.now()}`
    await kb.navigate()
    await kb.create(kbName)
    await kb.open(kbName)
    await expect(page).toHaveURL(/\/knowledge-bases\//)
    await kb.navigate()
    await kb.delete(kbName)
  })

  test('edit KB settings', async ({ authenticatedPage: page }, testInfo) => {
    const kb = new KBPage(page)
    const kbName = `Settings Test KB ${testInfo.workerIndex}-${Date.now()}`
    await kb.navigate()
    await kb.create(kbName)
    await kb.open(kbName)
    await page.getByRole('tab', { name: 'Settings' }).click()
    await page.getByLabel('Description').fill('Updated description')
    await page.getByRole('button', { name: 'Save' }).click()
    await expect(page.getByText('Saved')).toBeVisible()
  })
})
