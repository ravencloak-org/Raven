import { test, expect } from '../fixtures'

// E2E for the admin marketplace review queue (#734).
//
// Skipped unless E2E_ADMIN is configured — the gate is server-side
// (RAVEN_ADMIN_EMAILS) so a non-admin user can't see the page. The
// `adminPage` fixture signs in as the configured admin operator.
//
// Seeding a marketplace_report from the SPA isn't a primary user flow
// (and the report-submission UI is a sibling deliverable). We
// therefore exercise the navigate -> render -> action loop and trust
// the backend integration test (TestAdminModeration_Approve_...) to
// pin the atomic four-side-effect commitment.

test.describe('Admin: Marketplace report queue', () => {
  test('renders the queue and exposes status filter pills', async ({ adminPage: page }) => {
    await page.goto('/admin/marketplace/reports')
    await expect(page.getByRole('heading', { name: /report queue/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: 'Open' })).toBeVisible()
    await expect(page.getByRole('tab', { name: 'Resolved' })).toBeVisible()
  })

  test('approve flow surfaces the confirm modal then closes on cancel', async ({
    adminPage: page,
  }, testInfo) => {
    await page.goto('/admin/marketplace/reports')
    const rowCount = await page.getByTestId('admin-reports-row').count()
    testInfo.skip(
      rowCount === 0,
      'No open reports — seed one via POST /api/v1/marketplace/reports first',
    )

    const firstRow = page.getByTestId('admin-reports-row').first()
    const reportId = await firstRow.getAttribute('data-report-id')
    expect(reportId).toBeTruthy()

    await firstRow.getByText('Approve').click()
    await expect(page.getByText(/unpublish the KB/i)).toBeVisible()

    // Cancel — row should still be present.
    await page.getByRole('button', { name: 'Cancel' }).click()
    await expect(firstRow).toBeVisible()
  })

  test('approve commits then the resolved row reappears under the Resolved tab', async ({
    adminPage: page,
  }, testInfo) => {
    await page.goto('/admin/marketplace/reports')
    const rowCount = await page.getByTestId('admin-reports-row').count()
    testInfo.skip(
      rowCount === 0,
      'No open reports — seed one via POST /api/v1/marketplace/reports first',
    )

    const firstRow = page.getByTestId('admin-reports-row').first()
    const reportId = await firstRow.getAttribute('data-report-id')
    expect(reportId).toBeTruthy()

    await firstRow.getByText('Approve').click()
    await page.getByTestId('admin-reports-confirm').click()

    // Toast surfaces the new strike count.
    await expect(page.getByTestId('admin-reports-approve-toast')).toBeVisible()

    // Switch to Resolved — the row should now appear there with the
    // matching report id.
    await page.getByRole('tab', { name: 'Resolved' }).click()
    await expect(page.locator(`[data-report-id="${reportId}"]`)).toBeVisible()
  })
})
