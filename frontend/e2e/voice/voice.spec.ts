import { test, expect } from '../fixtures'

test.describe('Voice Sessions', () => {
  test('initiate LiveKit session via UI (mocked SFU)', async ({ adminPage: page }) => {
    await page.goto('/knowledge-bases/test-kb/voice')
    // Click "Start Voice Session" — the SFU is mocked in E2E env via env var
    await page.getByRole('button', { name: 'Start Voice Session' }).click()
    // Session should transition to "connecting" or "active" state
    await expect(page.getByTestId('voice-session-status')).toContainText(/connecting|active/i, {
      timeout: 10000,
    })
  })

  test('view active voice sessions list', async ({ adminPage: page }) => {
    await page.goto('/voice/sessions')
    await expect(page.getByTestId('sessions-list')).toBeVisible()
  })

  test('end voice session', async ({ adminPage: page }) => {
    await page.goto('/voice/sessions')
    const sessionCount = await page.getByTestId('session-row').count()
    if (sessionCount > 0) {
      await page
        .getByTestId('session-row')
        .first()
        .getByRole('button', { name: 'End' })
        .click()
      await page.getByRole('button', { name: 'Confirm' }).click()
      await expect(page.getByText(/ended|terminated/i)).toBeVisible()
    }
  })
})
