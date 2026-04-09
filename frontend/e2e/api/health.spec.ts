import { test, expect } from '@playwright/test'

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080'

test.describe('Health Check', () => {
  test('GET /healthz returns 200 with DB and cache status', async ({ request }) => {
    const resp = await request.get(`${API_BASE}/healthz`)
    expect(resp.status()).toBe(200)
    const body = (await resp.json()) as Record<string, unknown>
    expect(body).toHaveProperty('database')
    expect(body).toHaveProperty('cache')
  })
})
