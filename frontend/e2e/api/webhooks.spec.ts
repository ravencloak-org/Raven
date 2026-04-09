import { test, expect } from '@playwright/test'
import crypto from 'crypto'

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080'

test.describe('Webhook HMAC Validation', () => {
  test.beforeEach(async (_ctx, testInfo) => {
    testInfo.skip(!process.env.API_BASE_URL, 'Set API_BASE_URL to run API integration tests')
  })

  test('Meta webhook with valid HMAC returns 200', async ({ request }) => {
    const secret = process.env.META_WEBHOOK_SECRET!
    const body = JSON.stringify({ object: 'whatsapp_business_account', entry: [] })
    const sig = 'sha256=' + crypto.createHmac('sha256', secret).update(body).digest('hex')
    const resp = await request.post(`${API_BASE}/webhooks/meta`, {
      headers: { 'X-Hub-Signature-256': sig, 'Content-Type': 'application/json' },
      data: body,
    })
    expect(resp.status()).toBe(200)
  })

  test('Meta webhook with invalid HMAC returns 403', async ({ request }) => {
    const resp = await request.post(`${API_BASE}/webhooks/meta`, {
      headers: {
        'X-Hub-Signature-256': 'sha256=invalidsignature',
        'Content-Type': 'application/json',
      },
      data: JSON.stringify({ object: 'whatsapp_business_account' }),
    })
    expect(resp.status()).toBe(403)
  })
})
