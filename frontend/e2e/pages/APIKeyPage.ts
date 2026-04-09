import { type Page, expect } from '@playwright/test'

export class APIKeyPage {
  constructor(private page: Page) {}

  async navigate() {
    await this.page.goto('/api-keys')
    await this.page.waitForSelector('[data-testid="api-keys-list"]', { timeout: 10000 })
  }

  async createKey(scope: 'workspace' | 'knowledge_base', kbIndex?: number) {
    await this.page.getByRole('button', { name: 'Create Key' }).click()
    await this.page.getByLabel('Scope').selectOption(scope)
    if (scope === 'knowledge_base' && kbIndex !== undefined) {
      await this.page.getByLabel('Knowledge Base').selectOption({ index: kbIndex })
    }
    await this.page.getByRole('button', { name: 'Generate' }).click()
    await expect(this.page.getByTestId('api-key-value')).toBeVisible({ timeout: 5000 })
    return this.page.getByTestId('api-key-value').innerText()
  }

  async revokeFirst() {
    await this.page
      .getByTestId('api-key-row')
      .first()
      .getByRole('button', { name: 'Revoke' })
      .click()
    await this.page.getByRole('button', { name: 'Confirm' }).click()
  }
}
