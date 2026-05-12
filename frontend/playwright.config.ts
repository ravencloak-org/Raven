import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: 'html',
  use: {
    baseURL: process.env.CI ? 'http://localhost:4173' : 'http://localhost:3000',
    trace: 'on-first-retry',
    // Components in this repo use `data-test="..."` (not the Playwright
    // default `data-testid`). Tell `getByTestId(...)` to look for our
    // attribute so the existing convention works.
    testIdAttribute: 'data-test',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
      testIgnore: 'e2e/mobile/**',
    },
    {
      name: 'mobile-chrome',
      use: { ...devices['Pixel 7'] },
      testMatch: 'e2e/mobile/**/*.spec.ts',
    },
  ],
  webServer: {
    command: process.env.CI ? 'npx vite preview' : 'npm run dev',
    url: process.env.CI ? 'http://localhost:4173' : 'http://localhost:3000',
    reuseExistingServer: !process.env.CI,
    timeout: 120000,
  },
})
