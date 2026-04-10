import { test, expect } from '../fixtures'

test.describe('Org & Workspace', () => {
  test('create workspace', async ({ adminPage: page }) => {
    await page.goto('/workspaces')
    await page.getByRole('button', { name: 'New Workspace' }).click()
    await page.getByLabel('Name').fill('Test Workspace E2E')
    await page.getByRole('button', { name: 'Create' }).click()
    await expect(page.getByText('Test Workspace E2E')).toBeVisible()
  })

  test('invite member to workspace', async ({ adminPage: page }) => {
    await page.goto('/workspaces/test-ws/members')
    await page.getByRole('button', { name: 'Invite' }).click()
    await page.getByLabel('Email').fill('newmember@example.com')
    await page.getByRole('button', { name: 'Send Invite' }).click()
    await expect(page.getByText('newmember@example.com')).toBeVisible()
  })

  test('remove member from workspace', async ({ adminPage: page }) => {
    await page.goto('/workspaces/test-ws/members')
    const memberCount = await page.getByTestId('member-row').count()
    if (memberCount > 1) {
      await page.getByTestId('member-row').last().getByRole('button', { name: 'Remove' }).click()
      await page.getByRole('button', { name: 'Confirm' }).click()
      await expect(page.getByTestId('member-row')).toHaveCount(memberCount - 1)
    }
  })

  test('member denied workspace-admin action (RBAC)', async ({ authenticatedPage: page }) => {
    // authenticatedPage is a regular member, not admin
    await page.goto('/workspaces/test-ws/settings')
    // Should see access denied or redirect, not settings form
    await expect(page.getByText(/Access denied|Forbidden|Not authorized/i)).toBeVisible()
  })

  test('viewer role cannot access KB settings', async ({ authenticatedPage: page }) => {
    // Navigate to KB settings as viewer
    await page.goto('/knowledge-bases/test-kb/settings')
    await expect(page.getByRole('button', { name: 'Save' })).not.toBeVisible()
  })
})
