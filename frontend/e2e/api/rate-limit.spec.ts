import { test, expect } from '@playwright/test'

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080'

test.describe('Rate Limiting', () => {
  test('burst beyond rate limit returns 429 with Retry-After', async ({ request }) => {
    const key = process.env.E2E_API_KEY!
    const responses: { status: number; retryAfter: string | null }[] = []
    // Fire 20 requests in parallel — threshold is likely lower
    await Promise.all(
      Array.from({ length: 20 }, () =>
        request
          .post(`${API_BASE}/api/v1/chat`, {
            headers: { 'X-API-Key': key },
            data: { message: 'ping', kb_id: process.env.E2E_KB_ID! },
          })
          .then((r) => responses.push({ status: r.status(), retryAfter: r.headers()['retry-after'] ?? null })),
      ),
    )
    const limited = responses.filter((r) => r.status === 429)
    expect(limited.length).toBeGreaterThan(0)
    for (const r of limited) {
      expect(r.retryAfter).not.toBeNull()
    }
  })
})
