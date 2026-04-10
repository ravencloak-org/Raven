import { type Page, expect } from '@playwright/test'

export class KBPage {
  constructor(private page: Page) {}

  async navigate() {
    await this.page.goto('/knowledge-bases')
    await this.page.waitForSelector('[data-testid="kb-list"]')
  }

  async create(name: string) {
    await this.page.getByRole('button', { name: 'New Knowledge Base' }).click()
    await this.page.getByLabel('Name').fill(name)
    await this.page.getByRole('button', { name: 'Create' }).click()
    await expect(this.page.getByText(name)).toBeVisible({ timeout: 5000 })
    return name
  }

  async delete(name: string) {
    await this.page.getByText(name).hover()
    await this.page.getByRole('button', { name: 'Delete' }).click()
    await this.page.getByRole('button', { name: 'Confirm' }).click()
    await expect(this.page.getByText(name)).not.toBeVisible({ timeout: 5000 })
  }

  async open(name: string) {
    await this.page.getByText(name).click()
    await this.page.waitForURL(/\/knowledge-bases\//)
  }
}
