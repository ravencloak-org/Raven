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
    const keyValueLocator = this.page.getByTestId('api-key-value')
    await expect(keyValueLocator).toBeVisible({ timeout: 5000 })
    return keyValueLocator.innerText()
  }

  async revokeFirst() {
    await this.page
      .getByTestId('api-key-row')
      .first()
      .getByRole('button', { name: 'Revoke' })
      .click()
    await this.page.getByRole('button', { name: 'Revoke Key' }).click()
    await expect(this.page.getByTestId('api-key-row').first()).not.toBeVisible({ timeout: 5000 })
  }
}
