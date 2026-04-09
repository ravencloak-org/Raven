import { test, expect } from '@playwright/test'

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080'

test.describe('API Auth', () => {
  test('valid JWT returns 200', async ({ request }) => {
    if (!process.env.E2E_USER || !process.env.E2E_PASS) {
      test.skip(true, 'E2E_USER/E2E_PASS not configured')
      return
    }
    // Obtain a valid JWT from Keycloak test realm
    const tokenResp = await request.post(
      `${process.env.KEYCLOAK_URL}/realms/raven/protocol/openid-connect/token`,
      {
        form: {
          grant_type: 'password',
          client_id: 'raven-api',
          username: process.env.E2E_USER,
          password: process.env.E2E_PASS,
        },
      },
    )
    expect(tokenResp.ok(), `Token request failed: ${tokenResp.status()}`).toBe(true)
    const { access_token } = (await tokenResp.json()) as { access_token: string }

    const resp = await request.get(`${API_BASE}/api/v1/knowledge-bases`, {
      headers: { Authorization: `Bearer ${access_token}` },
    })
    expect(resp.status()).toBe(200)
  })

  test('expired JWT returns 401', async ({ request }) => {
    const resp = await request.get(`${API_BASE}/api/v1/knowledge-bases`, {
      headers: {
        Authorization: 'Bearer eyJhbGciOiJSUzI1NiJ9.eyJleHAiOjF9.fake',
      },
    })
    expect(resp.status()).toBe(401)
  })

  test('valid API key returns 200', async ({ request }) => {
    if (!process.env.E2E_API_KEY || !process.env.E2E_KB_ID) {
      test.skip(true, 'E2E_API_KEY/E2E_KB_ID not configured')
      return
    }
    const resp = await request.post(`${API_BASE}/api/v1/chat`, {
      headers: { 'X-API-Key': process.env.E2E_API_KEY },
      data: { message: 'hello', kb_id: process.env.E2E_KB_ID },
    })
    expect(resp.status()).toBe(200)
  })

  test('revoked API key returns 401', async ({ request }) => {
    const resp = await request.post(`${API_BASE}/api/v1/chat`, {
      headers: { 'X-API-Key': 'revoked-key-00000000' },
      data: { message: 'hello', kb_id: 'kb-1' },
    })
    expect(resp.status()).toBe(401)
  })

  test('wrong-scope API key returns 403', async ({ request }) => {
    if (!process.env.E2E_KB_A_KEY || !process.env.E2E_KB_B_ID) {
      test.skip(true, 'E2E_KB_A_KEY/E2E_KB_B_ID not configured')
      return
    }
    // Key scoped to kb-A cannot access kb-B
    const resp = await request.post(`${API_BASE}/api/v1/chat`, {
      headers: { 'X-API-Key': process.env.E2E_KB_A_KEY },
      data: { message: 'hello', kb_id: process.env.E2E_KB_B_ID },
    })
    expect(resp.status()).toBe(403)
  })
})
