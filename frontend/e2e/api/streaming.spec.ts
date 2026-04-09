import { test, expect } from '@playwright/test'

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080'

test.describe('SSE Streaming', () => {
  test('chat SSE endpoint delivers chunked events', async ({ page }) => {
    // Use page.evaluate to test SSE in browser context
    const chunks = await page.evaluate(
      async ({ apiBase, kbId }) => {
        return new Promise<string[]>((resolve) => {
          const received: string[] = []
          const source = new EventSource(
            `${apiBase}/api/v1/chat/stream?kb_id=${kbId}&message=hello`,
          )
          source.onmessage = (e: MessageEvent) => received.push(e.data as string)
          setTimeout(() => {
            source.close()
            resolve(received)
          }, 5000)
        })
      },
      { apiBase: API_BASE, kbId: process.env.E2E_KB_ID },
    )

    expect(chunks.length).toBeGreaterThan(0)
    const assembled = chunks.join('')
    expect(assembled.length).toBeGreaterThan(0)
  })
})
