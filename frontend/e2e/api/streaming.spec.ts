import { test, expect } from '@playwright/test'

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080'
const KB_ID = process.env.E2E_KB_ID ?? 'test-kb'

test.describe('SSE Streaming', () => {
  test.beforeEach(async ({}, testInfo) => {
    testInfo.skip(!process.env.API_BASE_URL, 'Set API_BASE_URL to run API integration tests')
  })

  test('chat SSE endpoint delivers chunked events', async ({ request }) => {
    // POST to the actual chat completions endpoint which returns SSE
    const response = await request.post(
      `${API_BASE}/api/v1/chat/${KB_ID}/completions`,
      {
        headers: { 'Content-Type': 'application/json' },
        data: { query: 'hello', stream: true },
      },
    )

    expect(response.status()).toBe(200)
    expect(response.headers()['content-type']).toContain('text/event-stream')

    const body = await response.text()
    // SSE events are separated by double newlines; collect data: lines
    const chunks = body
      .split('\n')
      .filter((line) => line.startsWith('data:'))
      .map((line) => line.replace(/^data:\s*/, ''))

    expect(chunks.length).toBeGreaterThan(0)
    const assembled = chunks.join('')
    expect(assembled.length).toBeGreaterThan(0)
  })
})
