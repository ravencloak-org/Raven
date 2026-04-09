import { type Page } from '@playwright/test'

export class ChatPage {
  constructor(private page: Page) {}

  async sendMessage(text: string) {
    await this.page.getByRole('textbox', { name: 'Message' }).fill(text)
    await this.page.getByRole('button', { name: 'Send' }).click()
  }

  async waitForResponse() {
    // SSE streaming — wait for assistant bubble to appear and stop loading
    await this.page.waitForSelector('[data-testid="assistant-message"]', { timeout: 30000 })
    await this.page.waitForSelector('[data-testid="message-loading"]', {
      state: 'detached',
      timeout: 30000,
    })
  }

  async getLastResponse() {
    const messages = await this.page.getByTestId('assistant-message').all()
    if (messages.length === 0) {
      throw new Error('No assistant messages found')
    }
    return messages[messages.length - 1]!.innerText()
  }

  async getCitations() {
    return this.page.getByTestId('citation-link').all()
  }
}
