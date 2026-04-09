import { test, expect } from '@playwright/test'

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080'

test.describe('Health Check', () => {
  test.beforeEach(async (_ctx, testInfo) => {
    testInfo.skip(!process.env.API_BASE_URL, 'Set API_BASE_URL to run API integration tests')
  })

  test('GET /healthz returns 200 with status ok', async ({ request }) => {
    const resp = await request.get(`${API_BASE}/healthz`)
    expect(resp.status()).toBe(200)
    const body = (await resp.json()) as Record<string, unknown>
    expect(body.status).toBe('ok')
  })
})
