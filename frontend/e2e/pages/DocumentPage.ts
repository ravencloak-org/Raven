import { type Page, expect } from '@playwright/test'

export class DocumentPage {
  constructor(private page: Page) {}

  async uploadFile(filePath: string) {
    await this.page.getByRole('button', { name: 'Upload' }).click()
    await this.page.locator('input[type="file"]').setInputFiles(filePath)
    await this.page.getByRole('button', { name: 'Start Upload' }).click()
  }

  async addURL(url: string) {
    await this.page.getByRole('button', { name: 'Add URL' }).click()
    await this.page.getByLabel('URL').fill(url)
    await this.page.getByRole('button', { name: 'Add' }).click()
  }

  async waitForProcessingComplete(docName: string) {
    // Poll for status badge to change from "processing" to "ready".
    // Scope via data-testid on the row rather than brittle parent traversal.
    await expect(
      this.page.getByTestId('doc-item').filter({ hasText: docName }).getByTestId('status-badge'),
    ).toHaveText('Ready', { timeout: 60000 })
  }
}
