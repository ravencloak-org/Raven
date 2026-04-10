import { test, expect } from '../fixtures'
import { ChatPage } from '../pages/ChatPage'

test.describe('Chat', () => {
  test('send message and receive streaming response', async ({ authenticatedPage: page }) => {
    await page.goto('/knowledge-bases/test-kb/chat')
    const chat = new ChatPage(page)
    await chat.sendMessage('What is this knowledge base about?')
    await chat.waitForResponse()
    const response = await chat.getLastResponse()
    expect(response.length).toBeGreaterThan(0)
  })

  test('citation links point to source documents', async ({ authenticatedPage: page }) => {
    await page.goto('/knowledge-bases/test-kb/chat')
    const chat = new ChatPage(page)
    await chat.sendMessage('Tell me about the main topics')
    await chat.waitForResponse()
    const citations = await chat.getCitations()
    if (citations.length > 0) {
      // Click first citation and verify it opens source
      await citations[0]!.click()
      await expect(page.getByTestId('source-preview')).toBeVisible()
    }
  })

  test('view session history', async ({ authenticatedPage: page }) => {
    await page.goto('/knowledge-bases/test-kb/chat')
    const chat = new ChatPage(page)
    await chat.sendMessage('First message')
    await chat.waitForResponse()
    // Reload and check history persists
    await page.reload()
    await expect(page.getByText('First message')).toBeVisible()
  })

  test('start new session clears history', async ({ authenticatedPage: page }) => {
    await page.goto('/knowledge-bases/test-kb/chat')
    const chat = new ChatPage(page)
    await chat.sendMessage('Old message')
    await chat.waitForResponse()
    await page.getByRole('button', { name: 'New Chat' }).click()
    await expect(page.getByText('Old message')).not.toBeVisible()
  })
})
