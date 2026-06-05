/**
 * AdminTakedownsView happy path + pagination + forbidden state.
 *
 * The backend gate (RAVEN_ADMIN_EMAILS allowlist) is exercised in
 * Go unit/integration tests; here we use route interception to
 * exercise the Vue view's rendering, source-pill mapping, strike
 * column, and Next/Previous pagination wiring without touching a
 * real database.
 *
 * The spec runs in single-user mode (no SuperTokens login flow) by
 * mocking `/api/v1/config` to return `single_user: true`. The router
 * then mounts the protected `/admin/marketplace/takedowns` route
 * without bouncing through /login.
 */
import { test, expect } from '@playwright/test'

const PAGE_1_NEXT_CURSOR = 'cursor-page-2'

function pageOne() {
  return {
    items: [
      {
        takedown_id: '11111111-1111-1111-1111-111111111111',
        target_kb_id: '22222222-2222-2222-2222-222222222222',
        target_kb_name: 'Original Recipes',
        target_org_slug: 'acme',
        target_org_display_name: 'Acme Corp',
        source: 'admin',
        notes: 'report confirmed',
        created_at: '2026-05-28T12:00:00.000000Z',
        strikes_after_org_total: 2,
      },
      {
        takedown_id: '33333333-3333-3333-3333-333333333333',
        target_kb_id: '44444444-4444-4444-4444-444444444444',
        target_kb_name: 'Marketing Brief',
        target_org_slug: 'globex',
        target_org_display_name: 'Globex',
        source: 'dmca',
        notes: 'DMCA notice 2026-06-01',
        created_at: '2026-05-20T09:00:00.000000Z',
        strikes_after_org_total: 1,
      },
    ],
    next_cursor: PAGE_1_NEXT_CURSOR,
  }
}

function pageTwo() {
  return {
    items: [
      {
        takedown_id: '55555555-5555-5555-5555-555555555555',
        target_kb_id: '66666666-6666-6666-6666-666666666666',
        target_kb_name: 'Sales Playbook',
        target_org_slug: 'initech',
        target_org_display_name: 'Initech',
        source: 'publisher',
        notes: '',
        created_at: '2026-05-10T18:00:00.000000Z',
        strikes_after_org_total: 0,
      },
    ],
  }
}

test.describe('AdminTakedownsView', () => {
  test.beforeEach(async ({ page }) => {
    // Single-user mode so the router skips the SuperTokens flow.
    await page.route('**/api/v1/config', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ single_user: true }),
      }),
    )
    // SuperTokens probe blocked so the auth init doesn't hang.
    await page.route('**/auth/**', (route) => route.abort())
  })

  test('happy path: renders rows + pagination forward and back', async ({ page }) => {
    let callCount = 0
    await page.route('**/api/v1/admin/marketplace/takedowns*', (route) => {
      callCount += 1
      const url = new URL(route.request().url())
      const cursor = url.searchParams.get('cursor')
      if (!cursor) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(pageOne()),
        })
      }
      if (cursor === PAGE_1_NEXT_CURSOR) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(pageTwo()),
        })
      }
      return route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({ detail: 'invalid cursor' }),
      })
    })

    await page.goto('/admin/marketplace/takedowns')
    await expect(page.getByTestId('admin-takedowns-view')).toBeVisible({
      timeout: 10_000,
    })
    await expect(page.getByTestId('admin-takedowns-table')).toBeVisible()

    // Two rows on page 1.
    let rows = page.getByTestId('admin-takedowns-row')
    await expect(rows).toHaveCount(2)
    await expect(rows.first()).toContainText('Original Recipes')
    await expect(rows.first()).toContainText('Acme Corp')
    await expect(rows.first()).toContainText('Admin')
    await expect(rows.nth(1)).toContainText('DMCA')

    // Click Next → page 2 (one row, publisher pill).
    await page.getByTestId('admin-takedowns-next').click()
    await expect(page.getByTestId('admin-takedowns-row')).toHaveCount(1)
    await expect(page.getByTestId('admin-takedowns-row').first()).toContainText(
      'Sales Playbook',
    )
    await expect(page.getByTestId('admin-takedowns-row').first()).toContainText(
      'Publisher',
    )
    // Next cursor empty → Next disabled.
    await expect(page.getByTestId('admin-takedowns-next')).toBeDisabled()

    // Click Previous → back to page 1.
    await page.getByTestId('admin-takedowns-prev').click()
    rows = page.getByTestId('admin-takedowns-row')
    await expect(rows).toHaveCount(2)
    await expect(rows.first()).toContainText('Original Recipes')

    // Sanity: backend was hit at least three times (page 1, page 2, page 1 again).
    expect(callCount).toBeGreaterThanOrEqual(3)
  })

  test('forbidden state when backend returns 403', async ({ page }) => {
    await page.route('**/api/v1/admin/marketplace/takedowns*', (route) =>
      route.fulfill({
        status: 403,
        contentType: 'application/json',
        body: JSON.stringify({ detail: 'raven admin required' }),
      }),
    )
    await page.goto('/admin/marketplace/takedowns')
    await expect(page.getByTestId('admin-takedowns-forbidden')).toBeVisible({
      timeout: 10_000,
    })
    await expect(page.getByTestId('admin-takedowns-table')).toHaveCount(0)
  })

  test('empty state when no takedowns', async ({ page }) => {
    await page.route('**/api/v1/admin/marketplace/takedowns*', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [] }),
      }),
    )
    await page.goto('/admin/marketplace/takedowns')
    await expect(page.getByTestId('admin-takedowns-empty')).toBeVisible({
      timeout: 10_000,
    })
  })
})
