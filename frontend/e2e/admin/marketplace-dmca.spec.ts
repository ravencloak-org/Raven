import { test, expect } from '../fixtures'

// E2E for the admin DMCA inbox + counter-notice workflow (#736, launch
// blocker per ADR-0006).
//
// Skipped unless E2E_ADMIN is configured — the gate is server-side
// (RAVEN_ADMIN_EMAILS). The `adminPage` fixture signs in as the
// configured admin operator.
//
// Backend integration tests (TestDMCAService_*) pin the atomic
// commitments (Submit two-side-effect, sweep auto-resolve, race-skip).
// This spec exercises the SPA: navigate -> render -> modal open/close
// -> form fields wired -> footer link to /legal/dmca renders.

test.describe('Admin: DMCA inbox', () => {
  test('renders the inbox and the status filter pills', async ({ adminPage: page }) => {
    await page.goto('/admin/marketplace/dmca')
    await expect(page.getByRole('heading', { name: /DMCA inbox/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: 'Pending' })).toBeVisible()
    await expect(page.getByRole('tab', { name: 'Counter-filed' })).toBeVisible()
    await expect(page.getByRole('tab', { name: 'Take-down' })).toBeVisible()
  })

  test('opens the record-notice modal and surfaces required fields', async ({
    adminPage: page,
  }) => {
    await page.goto('/admin/marketplace/dmca')
    await page.getByTestId('admin-dmca-record-button').click()
    await expect(page.getByTestId('admin-dmca-target-kb-id')).toBeVisible()
    await expect(page.getByTestId('admin-dmca-claimant-email')).toBeVisible()
    await expect(page.getByTestId('admin-dmca-claimant-name')).toBeVisible()
    await expect(page.getByTestId('admin-dmca-notice-text')).toBeVisible()

    // Cancel closes the modal without dispatching.
    await page.getByRole('button', { name: 'Cancel' }).click()
    await expect(page.getByTestId('admin-dmca-target-kb-id')).not.toBeVisible()
  })

  test('counter-notice button opens the counter-notice modal on pending rows', async ({
    adminPage: page,
  }, testInfo) => {
    await page.goto('/admin/marketplace/dmca')
    const rowCount = await page.getByTestId('admin-dmca-row').count()
    testInfo.skip(
      rowCount === 0,
      'No DMCA notices seeded — POST /api/v1/admin/marketplace/dmca first',
    )

    const firstRow = page.getByTestId('admin-dmca-row').first()
    const counterButton = firstRow.locator('[data-testid^="admin-dmca-counter-"]').first()
    const visible = await counterButton.isVisible().catch(() => false)
    testInfo.skip(!visible, 'first row is not in pending — no counter-notice action')

    await counterButton.click()
    await expect(page.getByTestId('admin-dmca-counter-text')).toBeVisible()
    await page.getByRole('button', { name: 'Cancel' }).click()
  })
})

test.describe('Public: DMCA legal page', () => {
  test('renders the designated-agent details', async ({ page }) => {
    await page.goto('/legal/dmca')
    await expect(page.getByRole('heading', { name: /DMCA Notice/i })).toBeVisible()
    await expect(page.getByText(/dmca@ravencloak\.org/i)).toBeVisible()
    await expect(page.getByText(/17 U\.S\.C\. § 512/i).first()).toBeVisible()
  })
})

test.describe('Marketplace footer', () => {
  test('list view exposes the DMCA legal link', async ({ adminPage: page }) => {
    await page.goto('/marketplace')
    const link = page.getByRole('link', { name: /DMCA policy/i })
    await expect(link).toBeVisible()
    await link.click()
    await expect(page).toHaveURL(/\/legal\/dmca/)
  })
})
