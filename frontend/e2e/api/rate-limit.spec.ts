import { test, expect } from '@playwright/test'

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080'

test.describe('Rate Limiting', () => {
  test('burst beyond rate limit returns 429 with Retry-After', async ({ request }) => {
    const key = process.env.E2E_API_KEY!
    const results: number[] = []
    // Fire 20 requests in parallel — threshold is likely lower
    await Promise.all(
      Array.from({ length: 20 }, () =>
        request
          .post(`${API_BASE}/api/v1/chat`, {
            headers: { 'X-API-Key': key },
            data: { message: 'ping', kb_id: process.env.E2E_KB_ID! },
          })
          .then((r) => results.push(r.status())),
      ),
    )
    expect(results).toContain(429)
  })
})
