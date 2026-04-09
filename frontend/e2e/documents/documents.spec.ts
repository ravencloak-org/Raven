import { test, expect } from '../fixtures'
import { DocumentPage } from '../pages/DocumentPage'
import path from 'path'

test.describe('Documents', () => {
  test('upload TXT file and see it processing', async ({ authenticatedPage: page }) => {
    await page.goto('/knowledge-bases/test-kb/documents')
    const docs = new DocumentPage(page)
    await docs.uploadFile(path.join(__dirname, '../fixtures/sample.txt'))
    await expect(page.getByText('sample.txt')).toBeVisible()
    await expect(page.getByTestId('status-badge').first()).toBeVisible()
    await docs.waitForProcessingComplete('sample.txt')
  })

  test('add URL source', async ({ authenticatedPage: page }) => {
    await page.goto('/knowledge-bases/test-kb/documents')
    const docs = new DocumentPage(page)
    await docs.addURL('https://en.wikipedia.org/wiki/Retrieval-augmented_generation')
    await expect(page.getByText('wikipedia.org')).toBeVisible()
  })

  test('view chunk list after processing', async ({ authenticatedPage: page }) => {
    // Upload a document, wait for processing, then navigate to its chunk list.
    await page.goto('/knowledge-bases/test-kb/documents')
    const docs = new DocumentPage(page)
    await docs.uploadFile(path.join(__dirname, '../fixtures/sample.txt'))
    await expect(page.getByText('sample.txt')).toBeVisible()
    await docs.waitForProcessingComplete('sample.txt')
    // Retrieve the real document ID from the link rather than using a hardcoded stub.
    const docLink = page.getByTestId('doc-item').filter({ hasText: 'sample.txt' }).getByRole('link')
    const href = await docLink.getAttribute('href')
    await page.goto(`${href}/chunks`)
    await expect(page.getByTestId('chunk-item').first()).toBeVisible()
  })

  test('delete document', async ({ authenticatedPage: page }) => {
    await page.goto('/knowledge-bases/test-kb/documents')
    await page.getByTestId('doc-item').first().hover()
    await page.getByRole('button', { name: 'Delete' }).click()
    await page.getByRole('button', { name: 'Confirm' }).click()
    await expect(page.getByText('Document deleted')).toBeVisible()
  })
})
