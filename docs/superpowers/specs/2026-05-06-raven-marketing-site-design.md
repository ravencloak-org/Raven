# Raven Marketing Site — Design Spec

- **Date:** 2026-05-06
- **Status:** Approved by Jobin (2026-05-06)
- **Supersedes:** `docs/superpowers/specs/2026-04-13-raven-landing-page-design.md`
- **Target deploy:** `raven.ravencloak.org` (Cloudflare Pages project `raven-landing`)

## 1. Goal

Replace the current `raven/landing/` static site (62 KB hand-written `index.html` built with the Tailwind v4 CLI) with a polished, multi-page marketing site built on the **Tailwind Plus Salient** template (Next.js 15 + React + Tailwind v4 + Headless UI), retheming Salient's accounting-product visual language into Raven's identity.

The deploy target stays unchanged (Cloudflare Pages project `raven-landing` → `raven.ravencloak.org`) so no DNS work is needed.

## 2. Non-goals

- No dark mode in v1.
- No blog or MDX content system in v1.
- No customer-logo cloud or testimonials section in v1 (placeholder content would be dishonest for a pre-launch OSS product).
- No i18n.
- No analytics integration in this spec (PostHog is a Phase-2 product concern, not a marketing-site concern; revisit separately).
- No CMS — content is checked into the repo as TSX.

## 3. Source

The Salient template is downloaded at `/Users/jobinlawrance/Project/tailwind-plus-salient/` with both `salient-js` and `salient-ts` variants. We use `salient-ts`.

Salient's stack:

- Next.js 15 (App Router)
- React 19
- Tailwind CSS v4 (`@tailwindcss/postcss`)
- Headless UI for accessible interactive primitives
- `clsx` for conditional classes
- ESLint (`eslint-config-next`) + Prettier (`prettier-plugin-tailwindcss`)

## 4. Architecture

### Repository layout

`raven/landing/` is replaced wholesale. Final layout:

```
raven/landing/
├── package.json              # next, react, react-dom, tailwindcss, @tailwindcss/postcss,
│                             # @tailwindcss/forms, @headlessui/react, clsx, typescript
├── package-lock.json
├── next.config.mjs           # output: 'export', images: { unoptimized: true }, trailingSlash: true
├── tsconfig.json
├── eslint.config.mjs
├── postcss.config.js
├── prettier.config.js
├── next-env.d.ts             # gitignored
├── .gitignore                # node_modules/, .next/, out/, next-env.d.ts
├── public/
│   ├── favicon.ico
│   ├── favicon-16x16.png, favicon-32x32.png, apple-touch-icon.png
│   ├── android-chrome-192x192.png, android-chrome-512x512.png
│   ├── site.webmanifest
│   ├── og-image.png          # 1200×630 OG card
│   ├── robots.txt
│   ├── sitemap.xml
│   └── _headers              # CF Pages security headers
├── src/
│   ├── app/
│   │   ├── layout.tsx        # root layout: <Header/>, <main>, <Footer/>; loads fonts
│   │   ├── page.tsx          # home (full landing)
│   │   ├── pricing/page.tsx
│   │   ├── self-host/page.tsx
│   │   ├── about/page.tsx
│   │   ├── not-found.tsx
│   │   └── opengraph-image.tsx
│   ├── components/
│   │   ├── Container.tsx, Button.tsx, NavLink.tsx
│   │   ├── Logo.tsx          # variants: full | mark | mark-inverted
│   │   ├── Header.tsx, Footer.tsx
│   │   ├── Hero.tsx
│   │   ├── PrimaryFeatures.tsx
│   │   ├── SecondaryFeatures.tsx
│   │   ├── CallToAction.tsx
│   │   ├── PricingTeaser.tsx        # used on /
│   │   ├── PricingTable.tsx         # used on /pricing
│   │   ├── PricingFaqs.tsx
│   │   ├── SystemRequirements.tsx   # used on /self-host
│   │   ├── QuickStart.tsx           # used on /self-host (Compose snippet, install one-liner)
│   │   ├── UpgradeNotes.tsx
│   │   ├── CommunitySupport.tsx
│   │   ├── Mission.tsx              # used on /about
│   │   ├── Maintainers.tsx
│   │   └── Faqs.tsx
│   ├── images/
│   │   ├── logo-full.svg            # source: ~/Downloads/Raven Logo Final.svg
│   │   ├── logo-mark.svg
│   │   ├── hero-illustration.svg    # to-be-designed; placeholder uses logo mark
│   │   └── screenshots/
│   │       ├── chat.svg             # carry over from current landing/assets/screenshots/
│   │       ├── voice.svg
│   │       └── whatsapp.svg
│   └── styles/tailwind.css   # Tailwind v4 entry, CSS @theme block for tokens
├── scripts/
│   └── build-favicons.mjs    # one-shot generator from logo-mark.svg, run manually
├── tests/
│   └── smoke.spec.ts         # Playwright smoke tests
└── playwright.config.ts
```

### Component sizing rule

Each component file owns one section or one primitive and stays under ~200 lines. If a section grows past that, split it (e.g. `PricingTable` rows extracted into `PricingPlan.tsx`). Smaller files are easier to reason about and produce more reliable edits.

## 5. Branding & visual system

### Palette

CSS custom properties in `styles/tailwind.css` `@theme` block:

| Token | Value | Use |
|---|---|---|
| `--color-ink` | `oklch(0.18 0 0)` (≈ slate-950) | Headings, primary text |
| `--color-body` | `oklch(0.42 0 0)` (≈ slate-600) | Body copy |
| `--color-bg` | `oklch(0.98 0.005 75)` (warm off-white, ≈ stone-50) | Page background |
| `--color-surface` | `oklch(1 0 0)` (white) | Cards, elevated surfaces |
| `--color-accent` | `oklch(0.71 0.16 200)` (≈ cyan-500 / `#06B6D4`) | CTAs, links, highlights |
| `--color-accent-hover` | `oklch(0.65 0.18 200)` (≈ cyan-600) | Hover state |
| `--color-border` | `oklch(0.91 0.005 75)` (warm grey) | Hairlines |

Single accent (electric cyan) reads as AI/tech-forward without the generic indigo-purple LLM-startup look. Logo's monochrome character carries the rest.

### Typography

- **Body**: Inter (variable), self-hosted via `next/font/local` or `next/font/google` with `display: 'swap'` and `preload: true`.
- **Display headings**: Space Grotesk (variable), same loading strategy. Echoes the wordmark's geometric/wide character without trying to clone the Sathu Mac system font.
- **Mono** (for code snippets in `/self-host`): JetBrains Mono.
- All fonts loaded via `next/font` (no external requests at runtime — required for the strict CSP).

### Logo handling

Source: `~/Downloads/Raven Logo Final.svg`. Copied into the repo as:

- `src/images/logo-full.svg` — wing mark + "RAVEN" wordmark, horizontal lockup.
- `src/images/logo-mark.svg` — wing mark only.

`<Logo variant="full" | "mark" | "mark-inverted" />` component renders the right SVG inline (so it inherits `currentColor`). Header desktop uses `full`, header mobile and footer use `mark`. Favicon family is generated **once** from `logo-mark.svg` via a one-shot `scripts/build-favicons.mjs` (using `sharp`) and the outputs are checked into `public/`. The script is documented but not run in CI — favicons rarely change.

## 6. Information architecture

| Path | Purpose | Sections (in order) |
|---|---|---|
| `/` | Pitch + funnel | Hero · PrimaryFeatures (3-up) · SecondaryFeatures (tabbed) · CallToAction · PricingTeaser · Faqs · Footer |
| `/pricing` | Conversion | PricingTable (Self-Hosted Free / Cloud Starter / Cloud Pro) · PricingFaqs · Footer |
| `/self-host` | Trust + activation | Hero (compact) · SystemRequirements · QuickStart · UpgradeNotes · CommunitySupport · Footer |
| `/about` | Trust + signal | Mission · Maintainers · OpenSourceLicence note · Footer |
| `*` | 404 | NotFound · Footer |

### Hero pitch (placeholder copy)

> **Your team's knowledge, on your infrastructure.**
> A self-hostable, multi-tenant RAG platform with built-in voice, chat, and edge deployment. GDPR-ready out of the box.

CTAs: **Self-host in 5 min** (primary, cyan) → `/self-host` · **Star on GitHub** (secondary, ghost) → repo URL.

### Primary features (3-up)

1. **Self-hostable, by design.** Run on your own server, your own VPC, or a Raspberry Pi at the edge. Your data never leaves your network.
2. **Multi-tenant from day one.** Built for teams: workspaces, role-based access, audit trails. SOC2- and GDPR-aligned.
3. **AI that fits your stack.** Bring your own models — Ollama, OpenAI, Anthropic, or anything else. pgvector + BM25 hybrid search out of the box.

### Secondary features (tabbed, deeper detail)

Reuses the existing `chat.svg` / `voice.svg` / `whatsapp.svg` screenshots. Tabs: **Voice** (LiveKit-powered conversational search), **Chat** (real-time multi-user chat with citations), **Channels** (WhatsApp, Slack, email ingestion).

### Pricing tiers

| Tier | Headline | Price |
|---|---|---|
| Self-Hosted | Free forever, source-available | ₹0 |
| Cloud Starter | For small teams who want it managed | ₹X/seat/mo (TBD) |
| Cloud Pro | SSO, audit logs, priority support | ₹Y/seat/mo (TBD) |

Indian pricing routed through Hyperswitch (Razorpay/UPI/RuPay), per repo memory. **Actual numbers are TBD and tracked as an explicit follow-up issue, not blocking this site rebuild** — placeholder copy in the spec as `₹X` / `₹Y` until set.

### FAQs (drafts)

- Is Raven really free to self-host? — Yes. No telemetry. No upsell wall.
- What's the difference between Self-Hosted and Cloud? — Cloud is the same software, run by us, with SLA and SSO.
- Can it run on a Raspberry Pi? — Yes; that's a first-class deployment target.
- Where does my data go? — Self-hosted: nowhere it didn't already go. Cloud: only to our infrastructure (region of your choice).
- Which models does it support? — Anything OpenAI-API-compatible: Ollama, OpenAI, Anthropic, Groq, vLLM, etc.
- How is this different from a hosted Notion AI / a wrapper? — You own the data, the schema, and the keys.

## 7. Build & deploy pipeline

### Updated `.github/workflows/landing.yml`

```yaml
name: Deploy Landing to Cloudflare Pages

on:
  push:
    branches: [main]
    paths:
      - "landing/**"
      - ".github/workflows/landing.yml"
  pull_request:
    branches: [main]
    paths:
      - "landing/**"
      - ".github/workflows/landing.yml"

concurrency:
  group: landing-${{ github.ref }}
  cancel-in-progress: true

permissions: {}

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      deployments: write
      pull-requests: write
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-node@v6
        with:
          node-version: "22"
          cache: "npm"
          cache-dependency-path: landing/package-lock.json
      - name: Install
        working-directory: landing
        run: npm ci
      - name: Lint
        working-directory: landing
        run: npm run lint
      - name: Typecheck
        working-directory: landing
        run: npx tsc --noEmit
      - name: Build
        working-directory: landing
        run: npm run build
      - name: Deploy to Cloudflare Pages
        env:
          CLOUDFLARE_API_TOKEN: ${{ secrets.CLOUDFLARE_API_TOKEN }}
        if: github.event_name == 'push' && github.ref == 'refs/heads/main' && env.CLOUDFLARE_API_TOKEN != ''
        uses: cloudflare/wrangler-action@v3
        with:
          apiToken: ${{ env.CLOUDFLARE_API_TOKEN }}
          accountId: ${{ vars.CLOUDFLARE_ACCOUNT_ID }}
          command: pages deploy landing/out --project-name=raven-landing --branch=main --commit-dirty=true
          gitHubToken: ${{ secrets.GITHUB_TOKEN }}
```

### `next.config.mjs`

```js
const nextConfig = {
  output: 'export',
  images: { unoptimized: true },     // required for static export
  trailingSlash: true,                // CF Pages friendly: /pricing/index.html
  reactStrictMode: true,
};
export default nextConfig;
```

### Cloudflare Pages

The `raven-landing` Pages project is **not** modified through the CF dashboard — its build configuration is irrelevant because we deploy a pre-built `landing/out/` directory via Wrangler. Custom domain binding to `raven.ravencloak.org` stays untouched.

PR runs validate the build (lint, typecheck, `next build`, Playwright smoke) but **do not deploy a preview** — the deploy step is gated by `if: github.event_name == 'push' && github.ref == 'refs/heads/main'`, mirroring the existing `pages.yml` convention used by `frontend/`. If we later want PR previews, that's a small follow-up: drop the `--branch=main` flag and gate the deploy step to also allow PRs from internal branches.

## 8. SEO, metadata, security

### Per-page metadata

Every page exports a `metadata: Metadata` object (Next 15 App Router pattern) with `title`, `description`, `openGraph`, `twitter`, and `alternates.canonical`.

### `public/sitemap.xml`

Hand-maintained (4 URLs). Updated when adding pages. Listed in `robots.txt`.

### `public/robots.txt`

```
User-agent: *
Allow: /
Sitemap: https://raven.ravencloak.org/sitemap.xml
```

### `public/_headers` (Cloudflare Pages)

```
/*
  Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
  X-Content-Type-Options: nosniff
  X-Frame-Options: DENY
  Referrer-Policy: strict-origin-when-cross-origin
  Permissions-Policy: camera=(), microphone=(), geolocation=()
  Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'
```

The `'unsafe-inline'` on `style-src` is needed because Next 15's static export still emits inline `<style>` blocks for critical CSS. `script-src 'self'` is achievable for a no-dynamic-script presentational site; verified via deploy preview before merging to main.

### `opengraph-image.tsx`

Renders the wing mark in white on a `--color-ink` background with the tagline; emitted at build time as `/opengraph-image.png` (1200×630). Per-page OG can override.

## 9. Accessibility

- `lang="en"` on `<html>`.
- Semantic landmarks: `<header>`, `<main>`, `<footer>`, headings in proper hierarchy.
- All `<img>` have `alt`; decorative SVG marked `aria-hidden="true"`.
- Focus rings use `--color-accent` with sufficient contrast against `--color-bg`.
- Headless UI primitives (used in mobile nav and FAQ disclosures) carry their own keyboard support.
- `prefers-reduced-motion` respected: any subtle entrance animations gate on `@media (prefers-reduced-motion: no-preference)`.
- Colour contrast verified at AA for all text/background pairs.

## 10. Testing

| Gate | Tool | Where | Blocking? |
|---|---|---|---|
| Lint | ESLint (`eslint-config-next`) | CI + local pre-push | Yes |
| Typecheck | `tsc --noEmit` | CI + local pre-push | Yes |
| Build | `next build` | CI | Yes |
| Smoke | Playwright | CI (after build) | Yes |
| Manual click-through | Browser | Local pre-PR | Yes (per repo testing-gate rule) |

### Playwright smoke (`tests/smoke.spec.ts`)

Single test file. For each of the 4 pages plus `/non-existent-404`:

1. Page renders with status 200 (or 404 for `/non-existent-404`).
2. No `console.error` emitted during navigation.
3. Every `a[href^="/"]` resolves to a 200 within the static export.
4. Mobile viewport screenshot diff against committed baseline (only fails if the visual changes — baselines updated intentionally).

Runs against `npx serve landing/out` started by Playwright's `webServer` config.

No unit tests. Component tests on a presentational marketing site mostly assert JSX shape, which the visual snapshot already covers more honestly.

## 11. Branch, commit, rollout

- Branch: `feat/landing-salient-rebuild`
- Commit style: conventional commits, no `Co-Authored-By` trailers (per repo rule).
- Lint + typecheck + build + Playwright smoke must pass locally before push (per repo rule).
- PR opened with `gh pr create`, then **immediately** queued: `gh pr merge <PR#> --auto --squash` (per `CLAUDE.md`).
- First push to `main` after merge replaces the live site at `raven.ravencloak.org`.
- **Rollback:** `git revert` the merge commit and push; the next CI run redeploys the previous build to the same Pages project. Cloudflare Pages also retains every prior deployment and can be rolled back via the dashboard if CI is unavailable.

## 12. Out of scope, tracked as follow-ups (not blocking this build)

- Final pricing numbers for Cloud Starter / Cloud Pro (placeholder `₹X` / `₹Y` ship with the site).
- Final hero illustration (placeholder uses the logo mark).
- Real customer logos / testimonials (sections deliberately omitted, not stubbed).
- PostHog or similar analytics (Phase-2 product feature, separate spec).
- Dark mode (separate spec when prioritised).
- Blog / changelog (separate spec when prioritised).

## 13. Acceptance criteria

The implementation is complete when:

1. `raven/landing/` contains the Salient-derived Next.js project as described in §4.
2. `npm run dev` from `landing/` serves all 4 pages locally without console errors.
3. `npm run build` from `landing/` produces a `landing/out/` directory.
4. `npm run lint`, `npx tsc --noEmit`, and `npx playwright test` pass locally and in CI.
5. The updated `landing.yml` workflow deploys `landing/out` to the `raven-landing` Pages project.
6. After merge to `main`, `https://raven.ravencloak.org/` serves the new home page, `/pricing/`, `/self-host/`, and `/about/` all return 200, and `/sitemap.xml`, `/robots.txt`, `/og-image.png`, and `/favicon.ico` are reachable.
7. Lighthouse mobile score on the home page is ≥ 90 for Performance, Accessibility, Best Practices, and SEO (verified once on the production deploy; not a CI gate).
