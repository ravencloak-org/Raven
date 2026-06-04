import { test, expect } from '../fixtures'

// Marketplace happy-path: browse → preview → import → redirect to new KB.
// The test relies on at least one seeded Public KB being available on the
// target environment; on CI the seed step runs ahead of e2e (see
// `scripts/seed-marketplace-fixtures.sql`). Locally without a seed the test
// auto-skips by guarding on the empty-state element.

test.describe('Marketplace', () => {
  test('browse → preview → import flow', async ({ authenticatedPage: page }) => {
    await page.goto('/marketplace')
    await expect(page.getByTestId('marketplace-list-view')).toBeVisible()

    // If the marketplace is empty in the test env, skip the rest. Avoids a
    // brittle failure when seed data hasn't been provisioned.
    const empty = page.getByTestId('marketplace-empty-state')
    if (await empty.isVisible().catch(() => false)) {
      test.skip(true, 'No seeded Public KBs on this environment')
    }

    const firstCard = page.getByTestId('marketplace-card').first()
    await expect(firstCard).toBeVisible()
    await firstCard.click()

    await expect(page.getByTestId('marketplace-detail-view')).toBeVisible()
    await page.getByTestId('marketplace-detail-preview').click()
    await expect(page.getByTestId('marketplace-preview-dialog')).toBeVisible()

    // Use the Import CTA inside the preview dialog to keep the flow tight.
    await page.getByTestId('marketplace-preview-import-cta').click()
    await expect(page.getByTestId('marketplace-import-dialog')).toBeVisible()
    await page.getByTestId('marketplace-import-confirm').click()

    // Successful import redirects to the local KB detail page.
    await page.waitForURL(/\/orgs\/.+\/workspaces\/.+\/knowledge-bases\/.+/)
  })

  test('re-import button on an imported KB shows confirmation', async ({
    authenticatedPage: page,
  }) => {
    // Re-uses the test above's residue — assumes the user has at least one
    // imported KB. Soft-skips otherwise to avoid coupling.
    await page.goto('/dashboard')
    const importedKb = page.locator('[data-test="kb-re-import-button"]').first()
    if (!(await importedKb.isVisible().catch(() => false))) {
      test.skip(true, 'No imported KB available to exercise re-import')
    }
    await importedKb.click()
    await expect(page.getByTestId('kb-re-import-dialog')).toBeVisible()
    await page.getByTestId('kb-re-import-confirm').click()
    await expect(page.getByTestId('kb-re-import-dialog')).toBeHidden({ timeout: 15_000 })
  })
})
