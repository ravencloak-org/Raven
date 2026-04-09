# Playwright E2E Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Write comprehensive Playwright E2E tests covering all frontend user journeys and REST API flows, including auth, KB management, documents, chat, voice, WhatsApp, API keys, analytics, the embeddable chat widget, and all EE journeys (SSO, WAF, webhooks, licensing).

**Architecture:** Tests live in `frontend/e2e/` — the existing `frontend/playwright.config.ts` has `testDir: './e2e'` which resolves to `frontend/e2e/`. **Note:** The spec's directory diagram shows `tests/e2e/` at repo root, but this plan follows the existing Playwright config. Do NOT move or create a second config at repo root — use `frontend/e2e/` throughout. Page Object Models for reusable interactions. Shared auth fixture for Keycloak login. API mode tests alongside UI tests. Playwright `chromium` only (per existing config).

**Tech Stack:** `@playwright/test ^1.59.1`, TypeScript, `baseURL: http://localhost:5173` (dev) / `http://localhost:4173` (CI preview). Run via `npm run test:e2e` from `frontend/`.

---

## Pre-flight: Audit Existing E2E Tests

- [ ] **Step 1: Check what already exists**

```bash
find /Users/jobinlawrance/Project/raven/frontend/e2e -name '*.ts' 2>/dev/null | sort || echo "e2e dir empty/missing"
cat /Users/jobinlawrance/Project/raven/frontend/playwright.config.ts
```

Do not rewrite passing tests. Build on top of existing structure.

- [ ] **Step 2: Verify Playwright installation and run existing tests**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx playwright install chromium --with-deps && npm run test:e2e -- --reporter=list 2>&1 | tail -20
```

---

## Task 1: Directory Structure & Shared Fixtures

**Files:**
- Create: `frontend/e2e/fixtures/auth.ts`
- Create: `frontend/e2e/fixtures/api.ts`
- Create: `frontend/e2e/fixtures/index.ts`
- Create: `frontend/e2e/pages/KBPage.ts`
- Create: `frontend/e2e/pages/ChatPage.ts`
- Create: `frontend/e2e/pages/DocumentPage.ts`
- Create: `frontend/e2e/pages/APIKeyPage.ts`

- [ ] **Step 1: Create auth fixture** (`frontend/e2e/fixtures/auth.ts`)

```typescript
import { test as base, Page } from '@playwright/test'

export type AuthFixtures = {
  authenticatedPage: Page
  adminPage: Page
}

// Keycloak test credentials — set in .env.test or CI secrets
const TEST_USER = process.env.E2E_USER ?? 'testuser@example.com'
const TEST_PASS = process.env.E2E_PASS ?? 'testpassword'
const TEST_ADMIN = process.env.E2E_ADMIN ?? 'admin@example.com'
const TEST_ADMIN_PASS = process.env.E2E_ADMIN_PASS ?? 'adminpassword'

export async function loginAs(page: Page, email: string, password: string) {
  await page.goto('/')
  // Wait for Keycloak redirect
  await page.waitForURL(/\/realms\/raven\/protocol\/openid-connect\/auth/)
  await page.getByLabel('Email').fill(email)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Sign In' }).click()
  // Wait for redirect back to app
  await page.waitForURL('/')
  await page.waitForSelector('[data-testid="dashboard"]', { timeout: 10000 })
}

export const test = base.extend<AuthFixtures>({
  authenticatedPage: async ({ page }, use) => {
    await loginAs(page, TEST_USER, TEST_PASS)
    await use(page)
  },
  adminPage: async ({ page }, use) => {
    await loginAs(page, TEST_ADMIN, TEST_ADMIN_PASS)
    await use(page)
  },
})

export { expect } from '@playwright/test'
```

- [ ] **Step 2: Create API fixture** (`frontend/e2e/fixtures/api.ts`)

```typescript
import { APIRequestContext, request } from '@playwright/test'

export class APIClient {
  constructor(private req: APIRequestContext, private baseURL: string) {}

  async createKB(workspaceId: string, name: string) {
    const resp = await this.req.post(`${this.baseURL}/api/v1/knowledge-bases`, {
      data: { workspace_id: workspaceId, name },
    })
    return resp.json()
  }

  async createAPIKey(workspaceId: string, kbId?: string) {
    const resp = await this.req.post(`${this.baseURL}/api/v1/api-keys`, {
      data: { workspace_id: workspaceId, kb_id: kbId ?? null },
    })
    return resp.json()
  }

  async uploadDocument(kbId: string, content: Buffer, filename: string) {
    const resp = await this.req.post(`${this.baseURL}/api/v1/documents`, {
      multipart: {
        kb_id: kbId,
        file: { name: filename, mimeType: 'text/plain', buffer: content },
      },
    })
    return resp.json()
  }
}
```

- [ ] **Step 3: Create KBPage Page Object** (`frontend/e2e/pages/KBPage.ts`)

```typescript
import { Page, expect } from '@playwright/test'

export class KBPage {
  constructor(private page: Page) {}

  async navigate() {
    await this.page.goto('/knowledge-bases')
    await this.page.waitForSelector('[data-testid="kb-list"]')
  }

  async create(name: string) {
    await this.page.getByRole('button', { name: 'New Knowledge Base' }).click()
    await this.page.getByLabel('Name').fill(name)
    await this.page.getByRole('button', { name: 'Create' }).click()
    await expect(this.page.getByText(name)).toBeVisible({ timeout: 5000 })
    return name
  }

  async delete(name: string) {
    await this.page.getByText(name).hover()
    await this.page.getByRole('button', { name: 'Delete' }).click()
    await this.page.getByRole('button', { name: 'Confirm' }).click()
    await expect(this.page.getByText(name)).not.toBeVisible({ timeout: 5000 })
  }

  async open(name: string) {
    await this.page.getByText(name).click()
    await this.page.waitForURL(/\/knowledge-bases\//)
  }
}
```

- [ ] **Step 4: Create ChatPage Page Object** (`frontend/e2e/pages/ChatPage.ts`)

```typescript
import { Page, expect } from '@playwright/test'

export class ChatPage {
  constructor(private page: Page) {}

  async sendMessage(text: string) {
    await this.page.getByRole('textbox', { name: 'Message' }).fill(text)
    await this.page.getByRole('button', { name: 'Send' }).click()
  }

  async waitForResponse() {
    // SSE streaming — wait for assistant bubble to appear and stop loading
    await this.page.waitForSelector('[data-testid="assistant-message"]', { timeout: 30000 })
    await this.page.waitForSelector('[data-testid="message-loading"]', { state: 'detached', timeout: 30000 })
  }

  async getLastResponse() {
    const messages = await this.page.getByTestId('assistant-message').all()
    return messages[messages.length - 1].innerText()
  }

  async getCitations() {
    return this.page.getByTestId('citation-link').all()
  }
}
```

- [ ] **Step 5: Create DocumentPage Page Object** (`frontend/e2e/pages/DocumentPage.ts`)

```typescript
import { Page, expect } from '@playwright/test'
import path from 'path'

export class DocumentPage {
  constructor(private page: Page) {}

  async uploadFile(filePath: string) {
    await this.page.getByRole('button', { name: 'Upload' }).click()
    await this.page.locator('input[type="file"]').setInputFiles(filePath)
    await this.page.getByRole('button', { name: 'Start Upload' }).click()
  }

  async addURL(url: string) {
    await this.page.getByRole('button', { name: 'Add URL' }).click()
    await this.page.getByLabel('URL').fill(url)
    await this.page.getByRole('button', { name: 'Add' }).click()
  }

  async waitForProcessingComplete(docName: string) {
    // Poll for status badge to change from "processing" to "ready"
    await expect(
      this.page.getByText(docName).locator('..').getByTestId('status-badge')
    ).toHaveText('Ready', { timeout: 60000 })
  }
}
```

- [ ] **Step 6: Commit page objects and fixtures**

```bash
git add frontend/e2e/
git commit -m "test(e2e): add Playwright fixtures and page object models"
```

---

## Task 2: Auth Journeys (`frontend/e2e/auth/`)

**Files:**
- Create: `frontend/e2e/auth/login.spec.ts`

- [ ] **Step 1: Write login tests**

```typescript
// frontend/e2e/auth/login.spec.ts
import { test, expect } from '../fixtures'

test.describe('Authentication', () => {
  test('login via Keycloak SSO succeeds', async ({ page }) => {
    await page.goto('/')
    await page.waitForURL(/\/realms\/raven\/protocol\/openid-connect\/auth/)
    await page.getByLabel('Email').fill(process.env.E2E_USER!)
    await page.getByLabel('Password').fill(process.env.E2E_PASS!)
    await page.getByRole('button', { name: 'Sign In' }).click()
    await page.waitForURL('/')
    await expect(page.getByTestId('dashboard')).toBeVisible()
  })

  test('logout clears session and redirects to login', async ({ authenticatedPage: page }) => {
    await page.getByTestId('user-menu').click()
    await page.getByRole('button', { name: 'Logout' }).click()
    await page.waitForURL(/\/realms\/raven\/protocol\/openid-connect\/auth/)
    await expect(page.getByLabel('Email')).toBeVisible()
  })

  test('session expiry redirects to login', async ({ page }) => {
    // Navigate to protected route without login
    await page.goto('/knowledge-bases')
    await page.waitForURL(/\/realms\/raven\/protocol\/openid-connect\/auth/)
    await expect(page.getByLabel('Email')).toBeVisible()
  })

  test('invalid credentials shows error', async ({ page }) => {
    await page.goto('/')
    await page.waitForURL(/\/realms\/raven\/protocol\/openid-connect\/auth/)
    await page.getByLabel('Email').fill('wrong@example.com')
    await page.getByLabel('Password').fill('wrongpass')
    await page.getByRole('button', { name: 'Sign In' }).click()
    await expect(page.getByText(/Invalid credentials|Login failed/i)).toBeVisible()
  })
})
```

- [ ] **Step 2: Run auth tests**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx playwright test e2e/auth/ --reporter=list 2>&1 | tail -20
```

- [ ] **Step 3: Commit**

```bash
git add frontend/e2e/auth/
git commit -m "test(e2e): auth journeys (login, logout, session expiry, invalid credentials)"
```

---

## Task 3: Org/Workspace & RBAC Journeys

**Files:**
- Create: `frontend/e2e/workspaces/workspace.spec.ts`

```typescript
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
```

- [ ] **Commit**

```bash
git add frontend/e2e/workspaces/
git commit -m "test(e2e): workspace and RBAC journeys"
```

---

## Task 4: Knowledge Base & Document Journeys

**Files:**
- Create: `frontend/e2e/knowledge-bases/kb.spec.ts`
- Create: `frontend/e2e/documents/documents.spec.ts`
- Create: `frontend/tests/fixtures/sample.txt` (test file for upload)

- [ ] **Step 1: Create sample test file**

```bash
echo "This is a sample document for Playwright E2E testing. It contains enough text to be chunked and processed." > /Users/jobinlawrance/Project/raven/frontend/e2e/fixtures/sample.txt
```

- [ ] **Step 2: Write KB CRUD tests**

```typescript
// frontend/e2e/knowledge-bases/kb.spec.ts
import { test, expect } from '../fixtures'
import { KBPage } from '../pages/KBPage'

test.describe('Knowledge Base', () => {
  test('create, view, and delete a KB', async ({ authenticatedPage: page }) => {
    const kb = new KBPage(page)
    await kb.navigate()
    await kb.create('E2E Test KB')
    await kb.open('E2E Test KB')
    await expect(page).toHaveURL(/\/knowledge-bases\//)
    await kb.navigate()
    await kb.delete('E2E Test KB')
  })

  test('edit KB settings', async ({ authenticatedPage: page }) => {
    const kb = new KBPage(page)
    await kb.navigate()
    await kb.create('Settings Test KB')
    await kb.open('Settings Test KB')
    await page.getByRole('tab', { name: 'Settings' }).click()
    await page.getByLabel('Description').fill('Updated description')
    await page.getByRole('button', { name: 'Save' }).click()
    await expect(page.getByText('Saved')).toBeVisible()
  })
})
```

- [ ] **Step 3: Write document upload/URL tests**

```typescript
// frontend/e2e/documents/documents.spec.ts
import { test, expect } from '../fixtures'
import { DocumentPage } from '../pages/DocumentPage'
import path from 'path'

test.describe('Documents', () => {
  test('upload TXT file and see it processing', async ({ authenticatedPage: page }) => {
    await page.goto('/knowledge-bases/test-kb/documents')
    const docs = new DocumentPage(page)
    await docs.uploadFile(path.join(__dirname, '../fixtures/sample.txt'))
    await expect(page.getByText('sample.txt')).toBeVisible()
    await expect(page.getByTestId('status-badge').first()).toBeVisible()
  })

  test('add URL source', async ({ authenticatedPage: page }) => {
    await page.goto('/knowledge-bases/test-kb/documents')
    const docs = new DocumentPage(page)
    await docs.addURL('https://en.wikipedia.org/wiki/Retrieval-augmented_generation')
    await expect(page.getByText('wikipedia.org')).toBeVisible()
  })

  test('view chunk list after processing', async ({ authenticatedPage: page }) => {
    // Navigate to a pre-processed document
    await page.goto('/knowledge-bases/test-kb/documents/processed-doc-id/chunks')
    await expect(page.getByTestId('chunk-item').first()).toBeVisible()
  })

  test('delete document', async ({ authenticatedPage: page }) => {
    await page.goto('/knowledge-bases/test-kb/documents')
    await page.getByTestId('doc-item').first().hover()
    await page.getByRole('button', { name: 'Delete' }).click()
    await page.getByRole('button', { name: 'Confirm' }).click()
    await expect(page.getByText('Document deleted')).toBeVisible()
  })
})
```

- [ ] **Step 4: Run**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx playwright test e2e/knowledge-bases/ e2e/documents/ --reporter=list 2>&1 | tail -20
```

- [ ] **Step 5: Commit**

```bash
git add frontend/e2e/knowledge-bases/ frontend/e2e/documents/ frontend/e2e/fixtures/sample.txt
git commit -m "test(e2e): KB CRUD and document upload/URL/chunk journeys"
```

---

## Task 5: Chat Journeys (including SSE streaming)

**Files:**
- Create: `frontend/e2e/chat/chat.spec.ts`

```typescript
import { test, expect } from '../fixtures'
import { ChatPage } from '../pages/ChatPage'

test.describe('Chat', () => {
  test('send message and receive streaming response', async ({ authenticatedPage: page }) => {
    await page.goto('/knowledge-bases/test-kb/chat')
    const chat = new ChatPage(page)
    await chat.sendMessage('What is this knowledge base about?')
    await chat.waitForResponse()
    const response = await chat.getLastResponse()
    expect(response.length).toBeGreaterThan(0)
  })

  test('citation links point to source documents', async ({ authenticatedPage: page }) => {
    await page.goto('/knowledge-bases/test-kb/chat')
    const chat = new ChatPage(page)
    await chat.sendMessage('Tell me about the main topics')
    await chat.waitForResponse()
    const citations = await chat.getCitations()
    if (citations.length > 0) {
      // Click first citation and verify it opens source
      await citations[0].click()
      await expect(page.getByTestId('source-preview')).toBeVisible()
    }
  })

  test('view session history', async ({ authenticatedPage: page }) => {
    await page.goto('/knowledge-bases/test-kb/chat')
    const chat = new ChatPage(page)
    await chat.sendMessage('First message')
    await chat.waitForResponse()
    // Reload and check history persists
    await page.reload()
    await expect(page.getByText('First message')).toBeVisible()
  })

  test('start new session clears history', async ({ authenticatedPage: page }) => {
    await page.goto('/knowledge-bases/test-kb/chat')
    const chat = new ChatPage(page)
    await chat.sendMessage('Old message')
    await chat.waitForResponse()
    await page.getByRole('button', { name: 'New Chat' }).click()
    await expect(page.getByText('Old message')).not.toBeVisible()
  })
})
```

- [ ] **Commit**

```bash
git add frontend/e2e/chat/
git commit -m "test(e2e): chat journeys (streaming response, citations, session history)"
```

---

## Task 6: API Keys, LLM Providers, Voice, WhatsApp

**Files:**
- Create: `frontend/e2e/api-keys/apikeys.spec.ts`
- Create: `frontend/e2e/llm-providers/providers.spec.ts`
- Create: `frontend/e2e/voice/voice.spec.ts`
- Create: `frontend/e2e/whatsapp/whatsapp.spec.ts`

- [ ] **Step 1: API key tests**

```typescript
// frontend/e2e/api-keys/apikeys.spec.ts
test.describe('API Keys', () => {
  test('create workspace-scoped key', async ({ authenticatedPage: page }) => {
    await page.goto('/api-keys')
    await page.getByRole('button', { name: 'Create Key' }).click()
    await page.getByLabel('Scope').selectOption('workspace')
    await page.getByRole('button', { name: 'Generate' }).click()
    await expect(page.getByTestId('api-key-value')).toBeVisible()
  })

  test('create KB-scoped key', async ({ authenticatedPage: page }) => {
    await page.goto('/api-keys')
    await page.getByRole('button', { name: 'Create Key' }).click()
    await page.getByLabel('Scope').selectOption('knowledge_base')
    await page.getByLabel('Knowledge Base').selectOption({ index: 0 })
    await page.getByRole('button', { name: 'Generate' }).click()
    await expect(page.getByTestId('api-key-value')).toBeVisible()
  })

  test('revoke key removes it from list', async ({ authenticatedPage: page }) => {
    await page.goto('/api-keys')
    const keyCount = await page.getByTestId('api-key-row').count()
    if (keyCount > 0) {
      await page.getByTestId('api-key-row').first().getByRole('button', { name: 'Revoke' }).click()
      await page.getByRole('button', { name: 'Confirm' }).click()
      await expect(page.getByTestId('api-key-row')).toHaveCount(keyCount - 1)
    }
  })
})
```

- [ ] **Step 2: LLM provider tests**

```typescript
// frontend/e2e/llm-providers/providers.spec.ts
test('add OpenAI BYOK config', async ({ adminPage: page }) => {
  await page.goto('/llm-providers')
  await page.getByRole('button', { name: 'Add Provider' }).click()
  await page.getByLabel('Provider').selectOption('openai')
  await page.getByLabel('API Key').fill('sk-test-fake-key-for-e2e')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('openai')).toBeVisible()
})
```

- [ ] **Step 3: Voice tests**

```typescript
// frontend/e2e/voice/voice.spec.ts
test('initiate LiveKit session via UI (mocked SFU)', async ({ adminPage: page }) => {
  await page.goto('/knowledge-bases/test-kb/voice')
  // Click "Start Voice Session" — the SFU is mocked in E2E env via env var
  await page.getByRole('button', { name: 'Start Voice Session' }).click()
  // Session should transition to "connecting" or "active" state
  await expect(page.getByTestId('voice-session-status')).toContainText(/connecting|active/i, { timeout: 10000 })
})

test('view active voice sessions list', async ({ adminPage: page }) => {
  await page.goto('/voice/sessions')
  await expect(page.getByTestId('sessions-list')).toBeVisible()
})

test('end voice session', async ({ adminPage: page }) => {
  await page.goto('/voice/sessions')
  const sessionCount = await page.getByTestId('session-row').count()
  if (sessionCount > 0) {
    await page.getByTestId('session-row').first().getByRole('button', { name: 'End' }).click()
    await page.getByRole('button', { name: 'Confirm' }).click()
    await expect(page.getByText(/ended|terminated/i)).toBeVisible()
  }
})
```

- [ ] **Step 4: WhatsApp tests**

```typescript
// frontend/e2e/whatsapp/whatsapp.spec.ts
test('view incoming webhook events', async ({ adminPage: page }) => {
  await page.goto('/whatsapp/events')
  await expect(page.getByTestId('events-list')).toBeVisible()
})

test('trigger test callback endpoint', async ({ adminPage: page }) => {
  await page.goto('/whatsapp/settings')
  await page.getByRole('button', { name: 'Test Callback' }).click()
  await expect(page.getByTestId('callback-result')).toBeVisible({ timeout: 10000 })
  const result = await page.getByTestId('callback-result').innerText()
  expect(result).toMatch(/success|200|ok/i)
})

test('view webhook delivery status', async ({ adminPage: page }) => {
  await page.goto('/whatsapp/events')
  const events = await page.getByTestId('event-row').all()
  if (events.length > 0) {
    await expect(page.getByTestId('delivery-status').first()).toBeVisible()
  }
})
```

- [ ] **Step 5: Commit**

```bash
git add frontend/e2e/api-keys/ frontend/e2e/llm-providers/ frontend/e2e/voice/ frontend/e2e/whatsapp/
git commit -m "test(e2e): API keys, LLM providers, voice sessions, WhatsApp events"
```

---

## Task 7: Chat Widget Tests

**Files:**
- Create: `frontend/e2e/chat-widget/widget.spec.ts`
- Create: `frontend/e2e/chat-widget/widget-sandbox.html`

- [ ] **Step 1: Create sandbox HTML page**

```html
<!-- frontend/e2e/chat-widget/widget-sandbox.html -->
<!DOCTYPE html>
<html>
<head><title>Widget Sandbox</title></head>
<body>
  <h1>Widget Test Page</h1>
  <raven-chat
    api-key="test-api-key-from-env"
    kb-id="test-kb-id"
    base-url="http://localhost:8080"
  ></raven-chat>
  <script src="http://localhost:5173/chat-widget.js"></script>
</body>
</html>
```

- [ ] **Step 2: Write widget tests**

```typescript
// frontend/e2e/chat-widget/widget.spec.ts
import { test, expect } from '@playwright/test'

test.describe('Chat Widget', () => {
  test('valid API key: widget loads and accepts messages', async ({ page }) => {
    await page.goto('/e2e/chat-widget/widget-sandbox.html')
    // Wait for web component to register
    await page.waitForSelector('raven-chat', { timeout: 10000 })
    // Widget should show chat input
    const shadowInput = page.locator('raven-chat').locator('css=input[type="text"]')
    await shadowInput.fill('Hello from widget test')
    await shadowInput.press('Enter')
    // Wait for response in shadow DOM
    await page.waitForTimeout(3000)
    const messages = await page.locator('raven-chat').locator('css=[data-role="assistant"]').all()
    expect(messages.length).toBeGreaterThan(0)
  })

  test('invalid API key: widget shows error state, not blank or crash', async ({ page }) => {
    // Serve sandbox with invalid key
    await page.goto('/e2e/chat-widget/widget-sandbox-invalid-key.html')
    await page.waitForSelector('raven-chat', { timeout: 10000 })
    const errorEl = page.locator('raven-chat').locator('css=[data-testid="error-state"]')
    await expect(errorEl).toBeVisible({ timeout: 8000 })
    const errorText = await errorEl.innerText()
    expect(errorText).toMatch(/invalid|unauthorized|error/i)
  })
})
```

Create `widget-sandbox-invalid-key.html` with `api-key="invalid-key-12345"`.

- [ ] **Step 3: Commit**

```bash
git add frontend/e2e/chat-widget/
git commit -m "test(e2e): chat widget happy path and invalid API key error state"
```

---

## Task 8: Analytics & Notifications

**Files:**
- Create: `frontend/e2e/analytics/analytics.spec.ts`
- Create: `frontend/e2e/notifications/notifications.spec.ts`

```typescript
// analytics.spec.ts
test('view usage dashboard with date filter', async ({ adminPage: page }) => {
  await page.goto('/analytics')
  await expect(page.getByTestId('usage-chart')).toBeVisible()
  await page.getByLabel('Date Range').selectOption('last_7_days')
  await expect(page.getByTestId('usage-chart')).toBeVisible()
})

test('export analytics data', async ({ adminPage: page }) => {
  await page.goto('/analytics')
  // Intercept the download
  const downloadPromise = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Export' }).click()
  const download = await downloadPromise
  expect(download.suggestedFilename()).toMatch(/analytics.*\.(csv|json|xlsx)/)
})

// notifications.spec.ts
test('create notification rule', async ({ adminPage: page }) => {
  await page.goto('/notifications')
  await page.getByRole('button', { name: 'New Rule' }).click()
  await page.getByLabel('Event').selectOption('document_processed')
  await page.getByLabel('Email').fill('notify@example.com')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('notify@example.com')).toBeVisible()
})

test('receive in-app notification after triggering event', async ({ adminPage: page }) => {
  // Upload a document to trigger 'document_processed' notification
  await page.goto('/knowledge-bases/test-kb/documents')
  await page.locator('input[type="file"]').setInputFiles({
    name: 'notify-test.txt',
    mimeType: 'text/plain',
    buffer: Buffer.from('notification trigger content'),
  })
  await page.getByRole('button', { name: 'Start Upload' }).click()
  // Wait for in-app notification badge or toast
  await expect(page.getByTestId('notification-badge')).toBeVisible({ timeout: 30000 })
})
```

- [ ] **Commit**

```bash
git add frontend/e2e/analytics/ frontend/e2e/notifications/
git commit -m "test(e2e): analytics dashboard and notification rules"
```

---

## Task 9: API Mode Tests (REST Endpoints)

**Files:**
- Create: `frontend/e2e/api/auth.spec.ts`
- Create: `frontend/e2e/api/rate-limit.spec.ts`
- Create: `frontend/e2e/api/webhooks.spec.ts`
- Create: `frontend/e2e/api/streaming.spec.ts`
- Create: `frontend/e2e/api/health.spec.ts`

- [ ] **Step 1: Write auth API tests**

```typescript
// frontend/e2e/api/auth.spec.ts
import { test, expect } from '@playwright/test'

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080'

test.describe('API Auth', () => {
  test('valid JWT returns 200', async ({ request }) => {
    // Obtain a valid JWT from Keycloak test realm
    const tokenResp = await request.post(`${process.env.KEYCLOAK_URL}/realms/raven/protocol/openid-connect/token`, {
      form: {
        grant_type: 'password',
        client_id: 'raven-api',
        username: process.env.E2E_USER!,
        password: process.env.E2E_PASS!,
      },
    })
    const { access_token } = await tokenResp.json()

    const resp = await request.get(`${API_BASE}/api/v1/knowledge-bases`, {
      headers: { Authorization: `Bearer ${access_token}` },
    })
    expect(resp.status()).toBe(200)
  })

  test('expired JWT returns 401', async ({ request }) => {
    const resp = await request.get(`${API_BASE}/api/v1/knowledge-bases`, {
      headers: { Authorization: 'Bearer eyJhbGciOiJSUzI1NiJ9.eyJleHAiOjF9.fake' },
    })
    expect(resp.status()).toBe(401)
  })

  test('valid API key returns 200', async ({ request }) => {
    const resp = await request.post(`${API_BASE}/api/v1/chat`, {
      headers: { 'X-API-Key': process.env.E2E_API_KEY! },
      data: { message: 'hello', kb_id: process.env.E2E_KB_ID! },
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
    // Key scoped to kb-A cannot access kb-B
    const resp = await request.post(`${API_BASE}/api/v1/chat`, {
      headers: { 'X-API-Key': process.env.E2E_KB_A_KEY! },
      data: { message: 'hello', kb_id: process.env.E2E_KB_B_ID! },
    })
    expect(resp.status()).toBe(403)
  })
})
```

- [ ] **Step 2: Write rate limiting test**

```typescript
// frontend/e2e/api/rate-limit.spec.ts
test('burst beyond rate limit returns 429 with Retry-After', async ({ request }) => {
  const key = process.env.E2E_API_KEY!
  const results: number[] = []
  // Fire 20 requests in parallel — threshold is likely lower
  await Promise.all(Array.from({ length: 20 }, () =>
    request.post(`${API_BASE}/api/v1/chat`, {
      headers: { 'X-API-Key': key },
      data: { message: 'ping', kb_id: process.env.E2E_KB_ID! },
    }).then(r => results.push(r.status()))
  ))
  expect(results).toContain(429)
})
```

- [ ] **Step 3: Write webhook reception test**

```typescript
// frontend/e2e/api/webhooks.spec.ts
import crypto from 'crypto'

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
    headers: { 'X-Hub-Signature-256': 'sha256=invalidsignature', 'Content-Type': 'application/json' },
    data: JSON.stringify({ object: 'whatsapp_business_account' }),
  })
  expect(resp.status()).toBe(403)
})
```

- [ ] **Step 4: Write SSE streaming test**

```typescript
// frontend/e2e/api/streaming.spec.ts
test('chat SSE endpoint delivers chunked events', async ({ page }) => {
  // Use page.evaluate to test SSE in browser context
  const chunks = await page.evaluate(async ({ apiBase, apiKey, kbId }) => {
    return new Promise<string[]>((resolve) => {
      const received: string[] = []
      const source = new EventSource(`${apiBase}/api/v1/chat/stream?kb_id=${kbId}&message=hello`, {
        // @ts-ignore — headers via URL params for SSE
      })
      source.onmessage = (e) => received.push(e.data)
      setTimeout(() => { source.close(); resolve(received) }, 5000)
    })
  }, { apiBase: API_BASE, apiKey: process.env.E2E_API_KEY, kbId: process.env.E2E_KB_ID })

  expect(chunks.length).toBeGreaterThan(0)
  const assembled = chunks.join('')
  expect(assembled.length).toBeGreaterThan(0)
})
```

- [ ] **Step 5: Write health check test**

```typescript
// frontend/e2e/api/health.spec.ts
test('GET /healthz returns 200 with DB and cache status', async ({ request }) => {
  const resp = await request.get(`${API_BASE}/healthz`)
  expect(resp.status()).toBe(200)
  const body = await resp.json()
  expect(body).toHaveProperty('database')
  expect(body).toHaveProperty('cache')
})
```

- [ ] **Step 6: Run API tests**

```bash
cd /Users/jobinlawrance/Project/raven/frontend && npx playwright test e2e/api/ --reporter=list 2>&1 | tail -20
```

- [ ] **Step 7: Commit**

```bash
git add frontend/e2e/api/
git commit -m "test(e2e): API mode tests (auth, rate limiting, webhook HMAC, SSE streaming, health)"
```

---

## Task 10: EE Journeys

**Files:**
- Create: `frontend/e2e/ee/sso.spec.ts`
- Create: `frontend/e2e/ee/webhooks.spec.ts`
- Create: `frontend/e2e/ee/licensing.spec.ts`
- Create: `frontend/e2e/ee/security-rules.spec.ts`

- [ ] **Step 1: SSO flow tests**

```typescript
// frontend/e2e/ee/sso.spec.ts
test('OIDC login redirects to Keycloak and back', async ({ page }) => {
  await page.goto('/')
  await page.waitForURL(/keycloak/)
  expect(page.url()).toContain('protocol/openid-connect/auth')
})

test('SSO-only org blocks password login', async ({ page }) => {
  // Navigate to org that enforces SSO-only
  await page.goto('/org-sso-only/login')
  await expect(page.getByLabel('Password')).not.toBeVisible()
  await expect(page.getByRole('button', { name: /Login with SSO|Sign in with/i })).toBeVisible()
})
```

- [ ] **Step 2: EE webhook UI tests**

```typescript
// frontend/e2e/ee/webhooks.spec.ts
test('configure webhook endpoint', async ({ adminPage: page }) => {
  await page.goto('/settings/webhooks')
  await page.getByRole('button', { name: 'Add Webhook' }).click()
  await page.getByLabel('URL').fill('https://webhook.site/test')
  await page.getByLabel('Events').check('document.processed')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('webhook.site')).toBeVisible()
})

test('dead-lettered webhook can be replayed', async ({ adminPage: page }) => {
  await page.goto('/settings/webhooks/failed')
  const failedCount = await page.getByTestId('failed-delivery').count()
  if (failedCount > 0) {
    await page.getByTestId('failed-delivery').first().getByRole('button', { name: 'Replay' }).click()
    await expect(page.getByText('Replayed')).toBeVisible()
  }
})
```

- [ ] **Step 3: Licensing gate tests**

```typescript
// frontend/e2e/ee/licensing.spec.ts
test('EE feature is accessible with valid license', async ({ adminPage: page }) => {
  await page.goto('/settings/security-rules')
  await expect(page.getByTestId('security-rules-panel')).toBeVisible()
})

test('EE feature shows upgrade prompt without license', async ({ page }) => {
  // Use a non-EE tenant
  await page.goto('/settings/security-rules')
  await expect(page.getByText(/Upgrade|Enterprise/i)).toBeVisible()
})
```

- [ ] **Step 4: Security rules UI tests**

```typescript
// frontend/e2e/ee/security-rules.spec.ts
test('create a block rule', async ({ adminPage: page }) => {
  await page.goto('/settings/security-rules')
  await page.getByRole('button', { name: 'Add Rule' }).click()
  await page.getByLabel('Pattern').fill('DROP TABLE')
  await page.getByLabel('Action').selectOption('block')
  await page.getByRole('button', { name: 'Save Rule' }).click()
  await expect(page.getByText('DROP TABLE')).toBeVisible()
})
```

- [ ] **Step 5: Commit**

```bash
git add frontend/e2e/ee/
git commit -m "test(e2e): EE journeys (SSO, webhooks, licensing, security rules)"
```

---

## Task 11: CI Configuration Update

**Files:**
- Modify: `.github/workflows/frontend.yml`

- [ ] **Step 1: Update frontend CI to fast-fail Vitest before Playwright**

In `.github/workflows/frontend.yml`, ensure the two steps are sequential with explicit dependency:

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'npm'
          cache-dependency-path: frontend/package-lock.json

      - name: Install dependencies
        run: npm ci
        working-directory: frontend

      - name: Vitest unit tests (fast-fail gate)
        run: npm run test:unit
        working-directory: frontend

      # Playwright only runs if Vitest passed
      - name: Install Playwright browsers
        run: npx playwright install chromium --with-deps
        working-directory: frontend

      - name: Build for preview
        run: npm run build
        working-directory: frontend

      - name: Playwright E2E tests
        run: npm run test:e2e
        working-directory: frontend
        env:
          CI: true
          E2E_USER: ${{ secrets.E2E_USER }}
          E2E_PASS: ${{ secrets.E2E_PASS }}
          E2E_ADMIN: ${{ secrets.E2E_ADMIN }}
          E2E_ADMIN_PASS: ${{ secrets.E2E_ADMIN_PASS }}
          E2E_API_KEY: ${{ secrets.E2E_API_KEY }}
          E2E_KB_ID: ${{ secrets.E2E_KB_ID }}
          META_WEBHOOK_SECRET: ${{ secrets.META_WEBHOOK_SECRET }}
          API_BASE_URL: http://localhost:8080
          KEYCLOAK_URL: http://localhost:8180

      - name: Upload Playwright report
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: playwright-report
          path: frontend/playwright-report/
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/frontend.yml
git commit -m "ci: split Vitest and Playwright as sequential steps with fast-fail gate"
```
