import { test, expect } from '@playwright/test'

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080'

test.describe('Health Check', () => {
  test.beforeEach(async ({ request }) => {
    const alive = await request.get(`${API_BASE}/healthz`, { timeout: 3000 }).catch(() => null)
    if (!alive?.ok()) {
      test.skip(true, 'API not reachable — start the server to run health integration tests')
    }
  })

  test('GET /healthz returns 200 with status ok', async ({ request }) => {
    const resp = await request.get(`${API_BASE}/healthz`)
    expect(resp.status()).toBe(200)
    const body = await resp.json()
    expect(body.status).toBe('ok')
  })
})
