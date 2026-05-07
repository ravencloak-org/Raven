import { test, expect, type ConsoleMessage, type Page } from '@playwright/test'

const PAGES = ['/', '/pricing/', '/self-host/', '/about/'] as const

function captureConsoleErrors(page: Page): string[] {
  const errors: string[] = []
  page.on('console', (msg: ConsoleMessage) => {
    if (msg.type() === 'error') errors.push(msg.text())
  })
  page.on('pageerror', (err) => errors.push(err.message))
  return errors
}

for (const path of PAGES) {
  test(`page ${path} renders cleanly`, async ({ page }) => {
    const errors = captureConsoleErrors(page)
    const response = await page.goto(path)
    expect(response?.status(), `status for ${path}`).toBe(200)
    await expect(page.locator('header')).toBeVisible()
    await expect(page.locator('footer')).toBeVisible()
    expect(errors, `console errors on ${path}`).toEqual([])
  })

  test(`page ${path} has working internal links`, async ({ page }) => {
    await page.goto(path)
    const hrefs = await page.$$eval('a[href^="/"]', (els) =>
      els.map((el) => el.getAttribute('href')).filter((h): h is string => !!h),
    )
    const unique = Array.from(new Set(hrefs.map((h) => h.split('#')[0]).filter(Boolean)))
    for (const href of unique) {
      const res = await page.request.get(href)
      expect(res.status(), `link ${href} from ${path}`).toBe(200)
    }
  })
}

test('non-existent page returns 404', async ({ page }) => {
  const res = await page.goto('/no-such-page/', { waitUntil: 'domcontentloaded' })
  expect(res?.status()).toBe(404)
  await expect(page.locator('h1')).toContainText('Page not found')
})

// Visual snapshot diffs run locally only — font rendering differs enough across
// macOS / Linux / Windows that committed baselines would false-positive in CI.
// Smoke (status / console / link integrity) above runs everywhere.
test.describe('visual', () => {
  test.skip(!!process.env.CI, 'visual snapshots run locally only')
  for (const path of PAGES) {
    test(`${path} matches snapshot`, async ({ page }, testInfo) => {
      await page.goto(path)
      await expect(page).toHaveScreenshot(`${path.replace(/\//g, '_') || '_'}.png`, {
        fullPage: true,
        maxDiffPixelRatio: 0.02,
      })
    })
  }
})
