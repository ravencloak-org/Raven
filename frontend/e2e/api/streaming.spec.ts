import { test, expect } from '@playwright/test'

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080'

test.describe('SSE Streaming', () => {
  test('chat SSE endpoint delivers chunked events', async ({ page }) => {
    if (!process.env.E2E_KB_ID) {
      test.skip(true, 'E2E_KB_ID not configured')
      return
    }
    // Use page.evaluate to test streaming in browser context
    const chunks = await page.evaluate(
      async ({ apiBase, kbId }) => {
        const resp = await fetch(`${apiBase}/api/v1/chat/${kbId}/completions`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ query: 'hello' }),
        })
        if (!resp.ok || !resp.body) return []
        const reader = resp.body.getReader()
        const decoder = new TextDecoder()
        const received: string[] = []
        for (let i = 0; i < 20; i++) {
          const { done, value } = await reader.read()
          if (done) break
          received.push(decoder.decode(value))
        }
        return received
      },
      { apiBase: API_BASE, kbId: process.env.E2E_KB_ID },
    )

    expect(chunks.length).toBeGreaterThan(0)
    const assembled = chunks.join('')
    expect(assembled.length).toBeGreaterThan(0)
  })
})
