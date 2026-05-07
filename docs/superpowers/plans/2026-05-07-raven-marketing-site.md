# Raven Marketing Site Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `raven/landing/` (current 62 KB hand-written `index.html` + Tailwind CLI v4) with a Salient-derived Next.js 15 marketing site (4 pages: `/`, `/pricing`, `/self-host`, `/about`), built as a static export and deployed to the existing `raven-landing` Cloudflare Pages project at `raven.ravencloak.org`.

**Architecture:** Wholesale replace `landing/` contents with the Tailwind Plus Salient TypeScript template, retheme to Raven's monochrome+cyan brand system, swap Salient's accounting copy for Raven's positioning, build with `output: 'export'` to produce `landing/out/` for Wrangler to deploy. No backend, no SSR, no dynamic routes.

**Tech Stack:** Next.js 15 (App Router) · React 19 · TypeScript · Tailwind CSS v4 · Headless UI · Inter + Space Grotesk + JetBrains Mono fonts (via `next/font`) · Playwright (smoke tests) · GitHub Actions + `cloudflare/wrangler-action@v3` (deploy).

**Spec:** `docs/superpowers/specs/2026-05-06-raven-marketing-site-design.md`

**Worktree:** `/Users/jobinlawrance/Project/raven/.worktrees/landing-salient` on branch `feat/landing-salient-rebuild`.

**Conventions (from `CLAUDE.md` and stash memory):**
- Conventional commits, no `Co-Authored-By` trailers.
- Lint + typecheck + build + smoke tests pass locally before every push.
- Squash merge only; queue `gh pr merge <#> --auto --squash` immediately after `gh pr create`.

---

## File Structure

| Path | Responsibility | Created in task |
|---|---|---|
| `landing/package.json` | npm package, scripts | T1 |
| `landing/package-lock.json` | resolved deps | T1 |
| `landing/next.config.mjs` | Next.js config (static export) | T1 |
| `landing/tsconfig.json` | TS compiler config | T1 |
| `landing/eslint.config.mjs` | ESLint config | T1 |
| `landing/postcss.config.js` | PostCSS / Tailwind v4 entry | T1 |
| `landing/prettier.config.js` | Prettier config | T1 |
| `landing/.gitignore` | ignore node_modules/.next/out/ | T1 |
| `landing/src/styles/tailwind.css` | Tailwind v4 entry + `@theme` palette + ravenicons `@font-face` | T2 |
| `landing/src/app/layout.tsx` | root layout, fonts, Header/Footer | T2 |
| `landing/public/fonts/ravenicons.woff2` | brand wordmark font (vendored from `ravenlogoassets/`) | T2 |
| `landing/src/images/logo-mark.svg` | bird/wing mark (vendored from `ravenlogoassets/logo/`) | T3 |
| `landing/scripts/build-favicons.mjs` | one-shot favicon generator | T3 |
| `landing/public/favicon*.png`, `apple-touch-icon.png`, `site.webmanifest` | favicon family | T3 |
| `landing/src/components/Logo.tsx` | logo component (mark + ravenicons wordmark) | T3 |
| `landing/src/components/Container.tsx`, `Button.tsx`, `NavLink.tsx` | layout primitives | T4 |
| `landing/src/components/Header.tsx`, `Footer.tsx` | site chrome | T4 |
| `landing/src/components/Hero.tsx` | home hero | T5 |
| `landing/src/components/PrimaryFeatures.tsx` | home 3-up features | T6 |
| `landing/src/components/SecondaryFeatures.tsx` | home tabbed features | T7 |
| `landing/src/components/CallToAction.tsx` | home CTA strip | T8 |
| `landing/src/components/PricingTeaser.tsx` | home pricing teaser | T9 |
| `landing/src/components/Faqs.tsx` | home FAQs | T10 |
| `landing/src/app/page.tsx` | home page composition | T10 |
| `landing/src/app/pricing/page.tsx`, `src/components/PricingTable.tsx`, `PricingFaqs.tsx` | pricing page | T11 |
| `landing/src/app/self-host/page.tsx`, `src/components/SystemRequirements.tsx`, `QuickStart.tsx`, `UpgradeNotes.tsx`, `CommunitySupport.tsx` | self-host page | T12 |
| `landing/src/app/about/page.tsx`, `src/components/Mission.tsx`, `Maintainers.tsx` | about page | T13 |
| `landing/src/app/not-found.tsx` | 404 | T14 |
| `landing/src/app/opengraph-image.tsx` | dynamic OG image | T15 |
| `landing/public/robots.txt`, `sitemap.xml` | crawl + index | T15 |
| `landing/public/_headers` | Cloudflare Pages security headers | T16 |
| `landing/playwright.config.ts`, `tests/smoke.spec.ts` | smoke tests | T17 |
| `.github/workflows/landing.yml` | CI/CD update | T18 |
| (existing landing files removed) | clean slate | T1 |

Each component file owns one section/primitive and stays under ~200 lines (per spec §4).

---

### Task 1: Wipe existing `landing/` and scaffold from Salient

**Files:**
- Delete: everything in `landing/` except `landing/.gitignore` (we replace it)
- Copy from: `/Users/jobinlawrance/Project/tailwind-plus-salient/salient-ts/` (everything except `node_modules/`, `package-lock.json`, `(auth)` route group, accounting content)
- Create: `landing/next.config.mjs` (replaces `next.config.js`)
- Modify: `landing/package.json` (rename, scripts)
- Modify: `landing/.gitignore`

- [ ] **Step 1: Wipe `landing/`**

```bash
cd /Users/jobinlawrance/Project/raven/.worktrees/landing-salient
git rm -r landing/
mkdir landing
```

- [ ] **Step 2: Copy Salient TS template into `landing/`**

```bash
rsync -a --exclude='node_modules' --exclude='package-lock.json' \
  /Users/jobinlawrance/Project/tailwind-plus-salient/salient-ts/ landing/
# Drop accounting-product app subdir we don't need
rm -rf landing/src/app/'(auth)'
```

- [ ] **Step 3: Replace `next.config.js` with `next.config.mjs`**

```bash
rm landing/next.config.js
```

Create `landing/next.config.mjs`:

```js
/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
  images: { unoptimized: true },
  trailingSlash: true,
  reactStrictMode: true,
  poweredByHeader: false,
};

export default nextConfig;
```

- [ ] **Step 4: Update `landing/package.json`**

Edit `landing/package.json`:
- `name`: `"raven-landing"`
- `private`: `true`
- `version`: `"0.0.0"`
- Replace `scripts` with:

```json
"scripts": {
  "dev": "next dev",
  "build": "next build",
  "start": "next start",
  "lint": "next lint",
  "typecheck": "tsc --noEmit",
  "test": "playwright test"
}
```

Keep all `dependencies` and `devDependencies` from Salient as-is. Playwright will be added in T17.

- [ ] **Step 5: Write `landing/.gitignore`**

```
node_modules/
.next/
out/
next-env.d.ts
.env*.local
.DS_Store
playwright-report/
test-results/
```

- [ ] **Step 6: Install deps**

```bash
cd landing
npm install
```

Expected: lockfile created, no errors.

- [ ] **Step 7: Smoke — build the unmodified template**

```bash
npm run build
```

Expected: succeeds, `landing/out/` produced. (Content is still Salient's TaxPal accounting template; we replace it in subsequent tasks. This step proves the toolchain is wired up.)

- [ ] **Step 8: Commit**

```bash
git add landing/ -A
git commit -m "chore(landing): scaffold from Tailwind Plus Salient template

Wipe existing static landing/ and seed with the salient-ts template
(Next.js 15 + Tailwind v4 + Headless UI), configured for static export
output: 'export' and trailingSlash: true so Cloudflare Pages serves
clean URLs. Drops Salient's (auth) route group; content rebrand follows
in subsequent commits."
```

---

### Task 2: Brand tokens + fonts (incl. ravenicons)

**Files:**
- Modify: `landing/src/styles/tailwind.css`
- Modify: `landing/src/app/layout.tsx`
- Create: `landing/public/fonts/ravenicons.woff2` (copy from `ravenlogoassets/font/ravenicons.woff2`)

**Why ravenicons:** PR #408 merged a vendored `ravenicons` icon font under `ravenlogoassets/`. Each glyph is mapped to its real ASCII codepoint, so `<span style={{fontFamily: 'ravenicons'}}>RAVEN</span>` renders the brand wordmark. We use this for the wordmark in Header / Footer / OG card. Copying just the `.woff2` (modern browsers, smallest file) into `landing/public/fonts/` keeps the build self-contained without cross-package import paths.

- [ ] **Step 1: Vendor the ravenicons woff2 into `landing/public/fonts/`**

```bash
mkdir -p landing/public/fonts
cp ravenlogoassets/font/ravenicons.woff2 landing/public/fonts/ravenicons.woff2
```

Verify size > 0:

```bash
ls -la landing/public/fonts/
```

- [ ] **Step 2: Replace `landing/src/styles/tailwind.css` with Raven `@theme` palette + ravenicons @font-face**

Overwrite the file:

```css
@import 'tailwindcss';

@font-face {
  font-family: 'ravenicons';
  src: url('/fonts/ravenicons.woff2') format('woff2');
  font-display: swap;
}

@theme {
  --color-ink: oklch(0.18 0 0);          /* slate-950 */
  --color-body: oklch(0.42 0 0);          /* slate-600 */
  --color-bg: oklch(0.98 0.005 75);       /* warm off-white */
  --color-surface: oklch(1 0 0);          /* white */
  --color-accent: oklch(0.71 0.16 200);   /* cyan-500 */
  --color-accent-hover: oklch(0.65 0.18 200); /* cyan-600 */
  --color-border: oklch(0.91 0.005 75);

  --font-sans: var(--font-inter), ui-sans-serif, system-ui, sans-serif;
  --font-display: var(--font-space-grotesk), var(--font-inter), ui-sans-serif, system-ui, sans-serif;
  --font-mono: var(--font-jetbrains-mono), ui-monospace, SFMono-Regular, Menlo, monospace;
  --font-wordmark: 'ravenicons', var(--font-display);
}

html {
  background-color: var(--color-bg);
  color: var(--color-body);
}

body {
  font-family: var(--font-sans);
}

h1, h2, h3, h4 {
  font-family: var(--font-display);
  color: var(--color-ink);
}

.raven-wordmark {
  font-family: var(--font-wordmark);
  letter-spacing: 0.05em;
}
```

- [ ] **Step 3: Replace `landing/src/app/layout.tsx`**

Overwrite the file:

```tsx
import { type Metadata } from 'next'
import { Inter, Space_Grotesk, JetBrains_Mono } from 'next/font/google'
import clsx from 'clsx'

import '@/styles/tailwind.css'

export const metadata: Metadata = {
  title: {
    template: '%s — Raven',
    default: 'Raven — Self-hostable AI knowledge platform for teams',
  },
  description:
    "Raven is a self-hostable, multi-tenant RAG platform with built-in voice, chat, and edge deployment. Your team's knowledge, on your infrastructure.",
  metadataBase: new URL('https://raven.ravencloak.org'),
}

const inter = Inter({
  subsets: ['latin'],
  display: 'swap',
  variable: '--font-inter',
})

const spaceGrotesk = Space_Grotesk({
  subsets: ['latin'],
  display: 'swap',
  variable: '--font-space-grotesk',
})

const jetbrainsMono = JetBrains_Mono({
  subsets: ['latin'],
  display: 'swap',
  variable: '--font-jetbrains-mono',
})

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html
      lang="en"
      className={clsx(
        'h-full scroll-smooth antialiased',
        inter.variable,
        spaceGrotesk.variable,
        jetbrainsMono.variable,
      )}
    >
      <body className="flex min-h-full flex-col">{children}</body>
    </html>
  )
}
```

(Header/Footer are added to pages individually in T4–T13, not the root layout — gives flexibility per page.)

- [ ] **Step 4: Smoke — dev server renders without errors**

```bash
npm run dev
```

Open `http://localhost:3000`. Expected: page renders (still with Salient's TaxPal copy — that gets replaced later); Inter + Space Grotesk fonts loaded (verify in DevTools → Fonts → both Inter, Space Grotesk, JetBrains Mono, **and ravenicons** appear); no console errors. Quick visual probe: in DevTools Console run `getComputedStyle(document.documentElement).getPropertyValue('--color-accent')` — should print a non-empty cyan value. Stop dev server.

- [ ] **Step 5: Lint + typecheck**

```bash
npm run lint && npm run typecheck
```

Expected: both pass.

- [ ] **Step 6: Commit**

```bash
git add landing/src/styles/tailwind.css landing/src/app/layout.tsx \
        landing/public/fonts/ravenicons.woff2
git commit -m "feat(landing): wire Raven brand palette, fonts, and ravenicons

@theme tokens for ink/body/bg/accent (electric cyan), and load Inter +
Space Grotesk + JetBrains Mono via next/font (self-hosted). Vendors
ravenicons.woff2 from ravenlogoassets/ into public/fonts/ and exposes
it via the .raven-wordmark utility class so any element with class
'raven-wordmark' and the literal text 'RAVEN' renders the brand
wordmark. Sets metadata defaults to Raven; per-page overrides follow."
```

---

### Task 3: Logo assets, Logo component, favicon family

**Files:**
- Create: `landing/src/images/logo-mark.svg` (copy from `ravenlogoassets/logo/Asset *.svg`, choosing the cleanest standalone bird mark)
- Modify: `landing/src/components/Logo.tsx`
- Create: `landing/scripts/build-favicons.mjs`
- Create: `landing/public/favicon-16x16.png`, `favicon-32x32.png`, `apple-touch-icon.png`, `android-chrome-192x192.png`, `android-chrome-512x512.png`, `site.webmanifest`
- Replace: `landing/src/app/favicon.ico` (with the bird mark)

**Source:** `ravenlogoassets/` (added by PR #408, merged on main). It contains four bird/wing SVGs at `ravenlogoassets/logo/Asset 7.svg`, `Asset 8.svg`, `Asset 11.svg`, `Asset 12.svg`. **There is no separate logo-full SVG** — the lockup is composed in JSX as `<bird mark> + <span class="raven-wordmark">RAVEN</span>` so the wordmark is rendered via the `ravenicons` font wired in T2.

- [ ] **Step 1: Inspect the four bird-mark SVGs and pick one**

```bash
for f in ravenlogoassets/logo/Asset\ {7,8,11,12}.svg; do
  echo "=== $f ==="
  head -3 "$f"
done
```

Open each SVG in a browser or VS Code preview. **Pick the standalone wing/bird mark** (no circle background, no wordmark, no extra container shapes — just the bird in monochrome on transparent). Likely candidate: `Asset 7.svg` (smallest path-count file is usually the cleanest standalone). If unclear, check `git log -1 --format=%B 970252e0 -- 'ravenlogoassets/logo/'` for any context from the PR description, then default to the smallest file.

- [ ] **Step 2: Copy the chosen bird SVG into the landing source tree**

```bash
mkdir -p landing/src/images
cp 'ravenlogoassets/logo/Asset 7.svg' landing/src/images/logo-mark.svg
```

(Substitute the chosen filename if you picked a different Asset. Filename in the destination must be `logo-mark.svg`.)

- [ ] **Step 3: Replace `landing/src/components/Logo.tsx`**

Overwrite the file:

```tsx
import Image from 'next/image'
import clsx from 'clsx'

import logoMark from '@/images/logo-mark.svg'

type LogoProps = {
  variant?: 'full' | 'mark'
  inverted?: boolean
  className?: string
  /** Tailwind height class for the bird mark. Defaults to `h-8`. */
  markClassName?: string
}

export function Logo({
  variant = 'full',
  inverted = false,
  className,
  markClassName = 'h-8 w-auto',
}: LogoProps) {
  const mark = (
    <Image
      src={logoMark}
      alt=""
      aria-hidden="true"
      className={clsx(markClassName, inverted && 'invert')}
      priority
    />
  )

  if (variant === 'mark') {
    return (
      <span aria-label="Raven" className={clsx('inline-flex items-center', className)}>
        {mark}
      </span>
    )
  }

  return (
    <span
      aria-label="Raven"
      className={clsx('inline-flex items-baseline gap-2', className)}
    >
      {mark}
      <span
        className={clsx(
          'raven-wordmark text-2xl leading-none',
          inverted ? 'text-white' : 'text-[var(--color-ink)]',
        )}
      >
        RAVEN
      </span>
    </span>
  )
}
```

The wordmark uses the `.raven-wordmark` utility (defined in T2's `tailwind.css`), which sets `font-family: ravenicons`. The literal text `RAVEN` then renders the brand wordmark. Inverted variant flips the bird to white via `invert` and switches the wordmark colour.

- [ ] **Step 4: Write `landing/scripts/build-favicons.mjs`**

```js
// One-shot favicon generator. Run manually after logo updates.
// Usage: node scripts/build-favicons.mjs
import sharp from 'sharp'
import { promises as fs } from 'node:fs'
import path from 'node:path'

const SRC = path.resolve('src/images/logo-mark.svg')
const PUB = path.resolve('public')
const APP = path.resolve('src/app')

const sizes = {
  'favicon-16x16.png': 16,
  'favicon-32x32.png': 32,
  'apple-touch-icon.png': 180,
  'android-chrome-192x192.png': 192,
  'android-chrome-512x512.png': 512,
}

async function main() {
  const svg = await fs.readFile(SRC)
  for (const [name, size] of Object.entries(sizes)) {
    const out = path.join(PUB, name)
    await sharp(svg, { density: 384 })
      .resize(size, size, { fit: 'contain', background: { r: 0, g: 0, b: 0, alpha: 0 } })
      .png()
      .toFile(out)
    console.log('wrote', out)
  }
  // App-router convention: src/app/favicon.ico (binary 32×32 PNG renamed; Next handles it).
  await sharp(svg, { density: 384 })
    .resize(32, 32, { fit: 'contain' })
    .toFormat('png')
    .toFile(path.join(APP, 'favicon.ico'))
  console.log('wrote', path.join(APP, 'favicon.ico'))

  await fs.writeFile(
    path.join(PUB, 'site.webmanifest'),
    JSON.stringify(
      {
        name: 'Raven',
        short_name: 'Raven',
        icons: [
          { src: '/android-chrome-192x192.png', sizes: '192x192', type: 'image/png' },
          { src: '/android-chrome-512x512.png', sizes: '512x512', type: 'image/png' },
        ],
        theme_color: '#0a0a0a',
        background_color: '#fafaf9',
        display: 'standalone',
      },
      null,
      2,
    ),
  )
  console.log('wrote site.webmanifest')
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
```

- [ ] **Step 5: Run the favicon script**

```bash
node scripts/build-favicons.mjs
```

Expected: prints 7 "wrote" lines, no errors.

- [ ] **Step 6: Smoke — dev server, check Logo renders**

```bash
npm run dev
```

Expected: existing pages still render; the Salient `Header` (which already uses `<Logo />`) shows the Raven wing mark instead of TaxPal's logo. Stop dev server.

- [ ] **Step 7: Lint + typecheck**

```bash
npm run lint && npm run typecheck
```

- [ ] **Step 8: Commit**

```bash
git add landing/src/images/logo-mark.svg \
        landing/src/components/Logo.tsx landing/scripts/build-favicons.mjs \
        landing/public/favicon-16x16.png landing/public/favicon-32x32.png \
        landing/public/apple-touch-icon.png \
        landing/public/android-chrome-192x192.png landing/public/android-chrome-512x512.png \
        landing/public/site.webmanifest landing/src/app/favicon.ico
git commit -m "feat(landing): wire Raven bird mark, ravenicons wordmark, favicons

Vendors the bird mark SVG from ravenlogoassets/logo/ into the landing
source tree. The Logo component composes the lockup in JSX as bird
mark + a span using the .raven-wordmark utility (font-family:
ravenicons) so the literal text 'RAVEN' renders the brand wordmark.
A one-shot scripts/build-favicons.mjs generates the favicon family
from logo-mark.svg via sharp."
```

---

### Task 4: Header, Footer, layout primitives (Container, Button, NavLink)

**Files:**
- Modify: `landing/src/components/Container.tsx` (keep, no rebrand needed)
- Modify: `landing/src/components/Button.tsx` (re-skin to cyan accent)
- Modify: `landing/src/components/NavLink.tsx` (keep, no rebrand needed)
- Modify: `landing/src/components/Header.tsx` (replace Salient nav links with Raven's)
- Modify: `landing/src/components/Footer.tsx` (replace Salient links with Raven's)

- [ ] **Step 1: Re-skin `landing/src/components/Button.tsx`**

Overwrite the file:

```tsx
import Link from 'next/link'
import clsx from 'clsx'

const baseStyles = {
  solid:
    'inline-flex items-center justify-center rounded-full px-5 py-2.5 text-sm font-semibold tracking-tight transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-[var(--color-accent)]',
  outline:
    'inline-flex items-center justify-center rounded-full border px-5 py-2.5 text-sm font-semibold tracking-tight transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-[var(--color-accent)]',
}

const variantStyles = {
  solid: {
    accent:
      'bg-[var(--color-accent)] text-white hover:bg-[var(--color-accent-hover)] active:bg-[var(--color-accent-hover)]',
    ink:
      'bg-[var(--color-ink)] text-white hover:bg-[var(--color-body)] active:bg-[var(--color-body)]',
  },
  outline: {
    accent:
      'border-[var(--color-accent)] text-[var(--color-accent)] hover:bg-[var(--color-accent)]/5',
    ink:
      'border-[var(--color-border)] text-[var(--color-ink)] hover:border-[var(--color-ink)] hover:bg-[var(--color-ink)]/5',
  },
}

type ButtonProps = (
  | { variant?: 'solid'; color?: keyof (typeof variantStyles)['solid'] }
  | { variant: 'outline'; color?: keyof (typeof variantStyles)['outline'] }
) &
  (
    | (Omit<React.ComponentPropsWithoutRef<typeof Link>, 'color'> & { href: string })
    | (Omit<React.ComponentPropsWithoutRef<'button'>, 'color'> & { href?: undefined })
  )

export function Button({ className, ...props }: ButtonProps) {
  const variant = props.variant ?? 'solid'
  const color = props.color ?? 'accent'
  className = clsx(
    baseStyles[variant],
    variant === 'solid'
      ? variantStyles.solid[color as keyof typeof variantStyles.solid]
      : variantStyles.outline[color as keyof typeof variantStyles.outline],
    className,
  )
  return typeof props.href === 'undefined' ? (
    <button className={className} {...(props as React.ComponentPropsWithoutRef<'button'>)} />
  ) : (
    <Link className={className} {...(props as React.ComponentPropsWithoutRef<typeof Link>)} />
  )
}
```

- [ ] **Step 2: Replace `landing/src/components/Header.tsx`**

Overwrite the file:

```tsx
'use client'

import Link from 'next/link'
import { Popover, PopoverButton, PopoverPanel, Transition } from '@headlessui/react'
import clsx from 'clsx'

import { Button } from '@/components/Button'
import { Container } from '@/components/Container'
import { Logo } from '@/components/Logo'
import { NavLink } from '@/components/NavLink'

const REPO_URL = 'https://github.com/ravencloak-org/Raven'

function MobileNavLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <PopoverButton as={Link} href={href} className="block w-full p-2">
      {children}
    </PopoverButton>
  )
}

function MobileNavIcon({ open }: { open: boolean }) {
  return (
    <svg
      aria-hidden="true"
      className="h-3.5 w-3.5 overflow-visible stroke-[var(--color-ink)]"
      fill="none"
      strokeWidth={2}
      strokeLinecap="round"
    >
      <path d="M0 1H14M0 7H14M0 13H14" className={clsx('origin-center transition', open && 'scale-90 opacity-0')} />
      <path d="M2 2L12 12M12 2L2 12" className={clsx('origin-center transition', !open && 'scale-90 opacity-0')} />
    </svg>
  )
}

function MobileNavigation() {
  return (
    <Popover>
      <PopoverButton
        className="relative z-10 flex h-8 w-8 items-center justify-center focus:outline-none"
        aria-label="Toggle navigation"
      >
        {({ open }) => <MobileNavIcon open={open} />}
      </PopoverButton>
      <Transition
        enter="duration-150 ease-out"
        enterFrom="opacity-0"
        enterTo="opacity-100"
        leave="duration-150 ease-in"
        leaveFrom="opacity-100"
        leaveTo="opacity-0"
      >
        <PopoverPanel className="absolute inset-x-0 top-full mt-4 flex origin-top flex-col rounded-2xl bg-white p-4 text-lg tracking-tight text-[var(--color-ink)] shadow-xl ring-1 ring-[var(--color-border)]">
          <MobileNavLink href="/#features">Features</MobileNavLink>
          <MobileNavLink href="/pricing">Pricing</MobileNavLink>
          <MobileNavLink href="/self-host">Self-host</MobileNavLink>
          <MobileNavLink href="/about">About</MobileNavLink>
          <hr className="my-2 border-[var(--color-border)]" />
          <MobileNavLink href={REPO_URL}>GitHub</MobileNavLink>
        </PopoverPanel>
      </Transition>
    </Popover>
  )
}

export function Header() {
  return (
    <header className="py-6">
      <Container>
        <nav className="relative z-50 flex justify-between">
          <div className="flex items-center md:gap-x-12">
            <Link href="/" aria-label="Home">
              <Logo variant="mark" markClassName="h-8 w-auto md:hidden" />
              <Logo variant="full" markClassName="h-8 w-auto" className="hidden md:inline-flex" />
            </Link>
            <div className="hidden md:flex md:gap-x-6">
              <NavLink href="/#features">Features</NavLink>
              <NavLink href="/pricing">Pricing</NavLink>
              <NavLink href="/self-host">Self-host</NavLink>
              <NavLink href="/about">About</NavLink>
            </div>
          </div>
          <div className="flex items-center gap-x-5 md:gap-x-8">
            <Button href={REPO_URL} variant="outline" color="ink" className="hidden md:inline-flex">
              GitHub
            </Button>
            <Button href="/self-host">Self-host in 5 min</Button>
            <div className="-mr-1 md:hidden">
              <MobileNavigation />
            </div>
          </div>
        </nav>
      </Container>
    </header>
  )
}
```

- [ ] **Step 3: Replace `landing/src/components/Footer.tsx`**

Overwrite the file:

```tsx
import Link from 'next/link'

import { Container } from '@/components/Container'
import { Logo } from '@/components/Logo'

const REPO_URL = 'https://github.com/ravencloak-org/Raven'

export function Footer() {
  return (
    <footer className="bg-[var(--color-ink)] text-[var(--color-bg)]">
      <Container>
        <div className="py-16">
          <Logo variant="mark" inverted markClassName="h-10 w-auto" className="mx-auto" />
          <nav className="mt-10 text-sm" aria-label="Footer">
            <ul className="-my-1 flex flex-wrap justify-center gap-x-6 gap-y-1">
              <li><Link href="/#features" className="hover:text-white">Features</Link></li>
              <li><Link href="/pricing" className="hover:text-white">Pricing</Link></li>
              <li><Link href="/self-host" className="hover:text-white">Self-host</Link></li>
              <li><Link href="/about" className="hover:text-white">About</Link></li>
              <li><Link href={REPO_URL} className="hover:text-white">GitHub</Link></li>
            </ul>
          </nav>
        </div>
        <div className="flex flex-col items-center border-t border-white/10 py-10 sm:flex-row-reverse sm:justify-between">
          <div className="flex gap-x-6">
            <Link href={REPO_URL} className="text-sm text-white/60 hover:text-white" aria-label="Raven on GitHub">
              GitHub
            </Link>
          </div>
          <p className="mt-6 text-sm text-white/60 sm:mt-0">
            © {new Date().getFullYear()} Ravencloak. Source-available under the Raven licence.
          </p>
        </div>
      </Container>
    </footer>
  )
}
```

- [ ] **Step 4: Inspect `Container.tsx` and `NavLink.tsx`**

These ship from Salient unmodified and are visually neutral. Open both and verify they don't reference Salient brand colours. If they do, replace any `text-slate-*`, `text-blue-*` etc. with `text-[var(--color-ink)]` / `text-[var(--color-body)]`. Otherwise no changes.

- [ ] **Step 5: Smoke — dev server**

```bash
npm run dev
```

Expected: Header shows Raven logo + 4 nav links + GitHub button + cyan "Self-host in 5 min" CTA. Mobile viewport (DevTools, 375px) shows hamburger toggle. Footer shows mark + links on dark ink background. No console errors. Stop dev server.

- [ ] **Step 6: Lint + typecheck**

```bash
npm run lint && npm run typecheck
```

- [ ] **Step 7: Commit**

```bash
git add landing/src/components/Button.tsx landing/src/components/Header.tsx \
        landing/src/components/Footer.tsx landing/src/components/Container.tsx \
        landing/src/components/NavLink.tsx
git commit -m "feat(landing): rebrand Header/Footer/Button with Raven nav and palette

- Header: 4-link nav (/#features, /pricing, /self-host, /about) plus
  GitHub link and cyan 'Self-host in 5 min' primary CTA. Headless UI
  popover for the mobile sheet.
- Footer: dark ink background with the inverted wing mark and a single
  link row.
- Button: solid/outline × accent/ink variants using --color-accent
  (cyan) and --color-ink tokens."
```

---

### Task 5: Hero section

**Files:**
- Modify: `landing/src/components/Hero.tsx`

- [ ] **Step 1: Replace `landing/src/components/Hero.tsx`**

Overwrite the file:

```tsx
import { Button } from '@/components/Button'
import { Container } from '@/components/Container'

const REPO_URL = 'https://github.com/ravencloak-org/Raven'

export function Hero() {
  return (
    <Container className="pt-20 pb-16 text-center lg:pt-32">
      <h1 className="mx-auto max-w-4xl font-display text-5xl font-medium tracking-tight text-[var(--color-ink)] sm:text-7xl">
        Your team's knowledge,{' '}
        <span className="relative whitespace-nowrap text-[var(--color-accent)]">
          <span className="relative">on your infrastructure.</span>
        </span>
      </h1>
      <p className="mx-auto mt-6 max-w-2xl text-lg tracking-tight text-[var(--color-body)]">
        A self-hostable, multi-tenant RAG platform with built-in voice, chat, and
        edge deployment. GDPR-ready out of the box. Bring your own models.
      </p>
      <div className="mt-10 flex justify-center gap-x-6">
        <Button href="/self-host">Self-host in 5 min</Button>
        <Button href={REPO_URL} variant="outline" color="ink">
          <svg
            aria-hidden="true"
            className="-mr-1 h-5 w-5 flex-none"
            viewBox="0 0 24 24"
            fill="currentColor"
          >
            <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.111.82-.261.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.4 3-.405 1.02.005 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12" />
          </svg>
          Star on GitHub
        </Button>
      </div>
    </Container>
  )
}
```

- [ ] **Step 2: Smoke — dev server, view home**

```bash
npm run dev
```

Open `http://localhost:3000`. Expected: Hero renders with the new headline, sub-pitch, and two CTAs; cyan accent on "on your infrastructure." Stop dev server.

- [ ] **Step 3: Lint + typecheck**

```bash
npm run lint && npm run typecheck
```

- [ ] **Step 4: Commit**

```bash
git add landing/src/components/Hero.tsx
git commit -m "feat(landing): Hero with Raven pitch and primary/GitHub CTAs"
```

---

### Task 6: PrimaryFeatures (3-up)

**Files:**
- Modify: `landing/src/components/PrimaryFeatures.tsx`

- [ ] **Step 1: Replace `landing/src/components/PrimaryFeatures.tsx`**

Overwrite the file:

```tsx
import { Container } from '@/components/Container'

const features = [
  {
    title: 'Self-hostable, by design.',
    body:
      'Run on your own server, your own VPC, or a Raspberry Pi at the edge. Your data never leaves your network. No upsell wall, no hidden telemetry.',
  },
  {
    title: 'Multi-tenant from day one.',
    body:
      'Built for teams: workspaces, role-based access, audit trails. SOC 2 and GDPR alignment baked into the schema, not bolted on later.',
  },
  {
    title: 'AI that fits your stack.',
    body:
      'Bring your own models — Ollama, OpenAI, Anthropic, Groq, vLLM, anything OpenAI-API-compatible. pgvector + BM25 hybrid search out of the box.',
  },
]

export function PrimaryFeatures() {
  return (
    <section
      id="features"
      aria-label="Primary features of Raven"
      className="bg-[var(--color-ink)] py-20 text-white sm:py-32"
    >
      <Container>
        <div className="mx-auto max-w-2xl text-center md:mx-0 md:text-left">
          <h2 className="font-display text-3xl tracking-tight text-white sm:text-4xl md:text-5xl">
            Built for teams who own their infrastructure.
          </h2>
          <p className="mt-6 text-lg tracking-tight text-white/70">
            Three properties Raven holds to that most hosted RAG products don't.
          </p>
        </div>
        <ul className="mt-16 grid grid-cols-1 gap-x-8 gap-y-12 md:grid-cols-3">
          {features.map((f) => (
            <li key={f.title}>
              <h3 className="font-display text-xl font-medium text-white">{f.title}</h3>
              <p className="mt-4 text-base text-white/70">{f.body}</p>
            </li>
          ))}
        </ul>
      </Container>
    </section>
  )
}
```

- [ ] **Step 2: Smoke + lint + typecheck**

```bash
npm run dev   # verify section renders, then stop
npm run lint && npm run typecheck
```

- [ ] **Step 3: Commit**

```bash
git add landing/src/components/PrimaryFeatures.tsx
git commit -m "feat(landing): PrimaryFeatures three-up on dark ink"
```

---

### Task 7: SecondaryFeatures (tabbed: Voice / Chat / Channels)

**Files:**
- Modify: `landing/src/components/SecondaryFeatures.tsx`
- Move screenshots from old landing/assets to new repo: copy from `~/Project/raven` git history (current static landing has them) into `landing/src/images/screenshots/`

- [ ] **Step 1: Restore screenshot assets**

```bash
# From the worktree root
git show main:landing/assets/screenshots/chat.svg > landing/src/images/screenshots/chat.svg
git show main:landing/assets/screenshots/voice.svg > landing/src/images/screenshots/voice.svg
git show main:landing/assets/screenshots/whatsapp.svg > landing/src/images/screenshots/whatsapp.svg
```

(Note: `main` here is the local branch ref, which still has the old landing/ until PR #407 lands and we rebase. If `main` has already been overwritten by T1's wipe in the same worktree, use `git rev-parse origin/main` for the commit before T1, or copy from `/Users/jobinlawrance/Project/raven/landing/assets/screenshots/` in the main checkout — that copy is unmodified.)

Verify:

```bash
ls landing/src/images/screenshots/
```

Expected: `chat.svg  voice.svg  whatsapp.svg`.

- [ ] **Step 2: Replace `landing/src/components/SecondaryFeatures.tsx`**

Overwrite the file:

```tsx
'use client'

import { useState } from 'react'
import Image, { type StaticImageData } from 'next/image'
import { Tab, TabGroup, TabList, TabPanel, TabPanels } from '@headlessui/react'
import clsx from 'clsx'

import { Container } from '@/components/Container'
import voiceImg from '@/images/screenshots/voice.svg'
import chatImg from '@/images/screenshots/chat.svg'
import whatsappImg from '@/images/screenshots/whatsapp.svg'

type Feature = { title: string; summary: string; image: StaticImageData | string }

const features: Feature[] = [
  {
    title: 'Voice',
    summary:
      'Conversational search with low-latency LiveKit. Talk to your knowledge base, get cited answers in real time.',
    image: voiceImg,
  },
  {
    title: 'Chat',
    summary:
      'Real-time multi-user chat with citations and source previews. Share threads with your team without losing context.',
    image: chatImg,
  },
  {
    title: 'Channels',
    summary:
      'Ingest from WhatsApp, Slack, email, and the web. Raven keeps the source link so every answer stays traceable.',
    image: whatsappImg,
  },
]

export function SecondaryFeatures() {
  const [tab, setTab] = useState(0)
  return (
    <section
      id="secondary-features"
      aria-label="Surfaces and integrations"
      className="bg-[var(--color-bg)] py-20 sm:py-32"
    >
      <Container>
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)] sm:text-4xl">
            Three surfaces, one knowledge base.
          </h2>
          <p className="mt-4 text-lg tracking-tight text-[var(--color-body)]">
            Voice, chat, and ingestion channels — share the same indexes, the same access controls, the same audit trail.
          </p>
        </div>
        <TabGroup
          className="mt-16 grid grid-cols-1 items-center gap-y-2 pt-10 sm:gap-y-6 md:mt-20 lg:grid-cols-12 lg:pt-0"
          selectedIndex={tab}
          onChange={setTab}
        >
          <TabList className="-mx-4 flex overflow-x-auto pb-4 sm:mx-0 sm:flex-col sm:overflow-visible sm:pb-0 lg:col-span-5">
            {features.map((f, i) => (
              <Tab
                key={f.title}
                className={({ selected }) =>
                  clsx(
                    'group relative rounded-lg px-4 py-1 text-left ring-1 transition focus:outline-none lg:rounded-l-xl lg:rounded-r-none lg:p-6',
                    selected
                      ? 'bg-white ring-[var(--color-border)]'
                      : 'ring-transparent hover:bg-white/40',
                  )
                }
              >
                <h3 className="font-display text-lg text-[var(--color-ink)]">{f.title}</h3>
                <p className="mt-2 hidden text-sm text-[var(--color-body)] lg:block">{f.summary}</p>
              </Tab>
            ))}
          </TabList>
          <TabPanels className="lg:col-span-7">
            {features.map((f) => (
              <TabPanel key={f.title} className="rounded-2xl bg-white p-4 ring-1 ring-[var(--color-border)]">
                <Image
                  src={f.image}
                  alt={f.title + ' screenshot'}
                  className="w-full"
                  width={760}
                  height={460}
                />
              </TabPanel>
            ))}
          </TabPanels>
        </TabGroup>
      </Container>
    </section>
  )
}
```

- [ ] **Step 3: Smoke + lint + typecheck**

```bash
npm run dev   # verify tabs switch and screenshots render, then stop
npm run lint && npm run typecheck
```

- [ ] **Step 4: Commit**

```bash
git add landing/src/components/SecondaryFeatures.tsx \
        landing/src/images/screenshots/chat.svg \
        landing/src/images/screenshots/voice.svg \
        landing/src/images/screenshots/whatsapp.svg
git commit -m "feat(landing): SecondaryFeatures with Voice/Chat/Channels tabs

Carries screenshot SVGs over from the previous landing site and wires
them behind a Headless UI TabGroup."
```

---

### Task 8: CallToAction strip

**Files:**
- Modify: `landing/src/components/CallToAction.tsx`

- [ ] **Step 1: Replace `landing/src/components/CallToAction.tsx`**

Overwrite the file:

```tsx
import { Button } from '@/components/Button'
import { Container } from '@/components/Container'

export function CallToAction() {
  return (
    <section
      id="get-started"
      className="relative overflow-hidden bg-[var(--color-ink)] py-32"
    >
      <Container className="relative">
        <div className="mx-auto max-w-lg text-center">
          <h2 className="font-display text-3xl tracking-tight text-white sm:text-4xl">
            Ready to give your team a brain that doesn't leak?
          </h2>
          <p className="mt-4 text-lg tracking-tight text-white/70">
            Five minutes from <code className="font-mono text-[var(--color-accent)]">docker compose up</code> to a
            working voice + chat search over your team's documents.
          </p>
          <Button href="/self-host" color="accent" className="mt-10">
            Read the self-host guide
          </Button>
        </div>
      </Container>
    </section>
  )
}
```

- [ ] **Step 2: Smoke + lint + typecheck**

```bash
npm run dev && npm run lint && npm run typecheck
```

- [ ] **Step 3: Commit**

```bash
git add landing/src/components/CallToAction.tsx
git commit -m "feat(landing): CallToAction strip pointing to /self-host"
```

---

### Task 9: PricingTeaser (used on home)

**Files:**
- Create: `landing/src/components/PricingTeaser.tsx`

- [ ] **Step 1: Create `landing/src/components/PricingTeaser.tsx`**

```tsx
import { Button } from '@/components/Button'
import { Container } from '@/components/Container'

const plans = [
  {
    name: 'Self-Hosted',
    headline: 'Free forever. Source-available.',
    price: '₹0',
    cta: { label: 'Self-host in 5 min', href: '/self-host', variant: 'solid' as const },
  },
  {
    name: 'Cloud',
    headline: 'Managed by us, ready to scale.',
    price: 'From ₹X / seat / month',
    cta: { label: 'See pricing', href: '/pricing', variant: 'outline' as const },
  },
]

export function PricingTeaser() {
  return (
    <section
      id="pricing"
      aria-label="Pricing summary"
      className="bg-[var(--color-bg)] py-20 sm:py-32"
    >
      <Container>
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)] sm:text-4xl">
            Free if you run it. Reasonable if we run it.
          </h2>
          <p className="mt-4 text-lg tracking-tight text-[var(--color-body)]">
            Self-host the whole thing for free, or let us run it for you on managed Cloud with SLA, SSO, and audit logs.
          </p>
        </div>
        <div className="mx-auto mt-16 grid max-w-4xl grid-cols-1 gap-8 md:grid-cols-2">
          {plans.map((p) => (
            <div
              key={p.name}
              className="rounded-3xl bg-white p-8 ring-1 ring-[var(--color-border)]"
            >
              <h3 className="font-display text-xl text-[var(--color-ink)]">{p.name}</h3>
              <p className="mt-2 text-[var(--color-body)]">{p.headline}</p>
              <p className="mt-6 font-display text-3xl text-[var(--color-ink)]">{p.price}</p>
              <Button
                href={p.cta.href}
                variant={p.cta.variant}
                color={p.cta.variant === 'outline' ? 'ink' : 'accent'}
                className="mt-8 w-full"
              >
                {p.cta.label}
              </Button>
            </div>
          ))}
        </div>
      </Container>
    </section>
  )
}
```

- [ ] **Step 2: Smoke + lint + typecheck + commit**

```bash
npm run dev && npm run lint && npm run typecheck
git add landing/src/components/PricingTeaser.tsx
git commit -m "feat(landing): PricingTeaser two-card teaser linking to /pricing"
```

---

### Task 10: Faqs (home) + assemble home page

**Files:**
- Modify: `landing/src/components/Faqs.tsx`
- Modify: `landing/src/app/page.tsx`

- [ ] **Step 1: Replace `landing/src/components/Faqs.tsx`**

Overwrite the file:

```tsx
import { Container } from '@/components/Container'

const faqs = [
  [
    {
      q: 'Is Raven really free to self-host?',
      a: 'Yes. No telemetry, no upsell wall. The source is public and you keep it.',
    },
    {
      q: 'Can it run on a Raspberry Pi?',
      a: 'Yes. Edge deployment is a first-class target — Raven ships a Compose variant tuned for ARM64 + low memory.',
    },
  ],
  [
    {
      q: 'What is the difference between Self-Hosted and Cloud?',
      a: 'Identical software. Cloud is run by us with SLA, SSO, and audit logs in the region of your choice.',
    },
    {
      q: 'Where does my data go?',
      a: 'Self-hosted: nowhere it didn\'t already go. Cloud: only to our infrastructure, in the region you pick.',
    },
  ],
  [
    {
      q: 'Which models does it support?',
      a: 'Anything OpenAI-API-compatible — Ollama, OpenAI, Anthropic, Groq, vLLM. Embeddings via pgvector with BM25 hybrid search.',
    },
    {
      q: 'How is this different from a hosted Notion AI or a wrapper?',
      a: 'You own the data, the schema, and the keys. Raven is the platform; the surface is yours to extend.',
    },
  ],
]

export function Faqs() {
  return (
    <section
      id="faq"
      aria-label="Frequently asked questions"
      className="bg-[var(--color-bg)] py-20 sm:py-32"
    >
      <Container className="relative">
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)] sm:text-4xl">
            Frequently asked questions
          </h2>
        </div>
        <ul className="mx-auto mt-16 grid max-w-2xl grid-cols-1 gap-8 lg:max-w-7xl lg:grid-cols-3">
          {faqs.map((column, i) => (
            <li key={i}>
              <ul className="flex flex-col gap-y-8">
                {column.map((faq) => (
                  <li key={faq.q}>
                    <h3 className="font-display text-lg text-[var(--color-ink)]">{faq.q}</h3>
                    <p className="mt-4 text-sm text-[var(--color-body)]">{faq.a}</p>
                  </li>
                ))}
              </ul>
            </li>
          ))}
        </ul>
      </Container>
    </section>
  )
}
```

- [ ] **Step 2: Replace `landing/src/app/page.tsx`**

Overwrite the file:

```tsx
import { type Metadata } from 'next'

import { CallToAction } from '@/components/CallToAction'
import { Faqs } from '@/components/Faqs'
import { Footer } from '@/components/Footer'
import { Header } from '@/components/Header'
import { Hero } from '@/components/Hero'
import { PricingTeaser } from '@/components/PricingTeaser'
import { PrimaryFeatures } from '@/components/PrimaryFeatures'
import { SecondaryFeatures } from '@/components/SecondaryFeatures'

export const metadata: Metadata = {
  alternates: { canonical: 'https://raven.ravencloak.org/' },
}

export default function Home() {
  return (
    <>
      <Header />
      <main>
        <Hero />
        <PrimaryFeatures />
        <SecondaryFeatures />
        <CallToAction />
        <PricingTeaser />
        <Faqs />
      </main>
      <Footer />
    </>
  )
}
```

- [ ] **Step 3: Delete unused Salient components**

```bash
git rm landing/src/components/Pricing.tsx landing/src/components/Testimonials.tsx \
       landing/src/components/SlimLayout.tsx landing/src/components/Fields.tsx
```

- [ ] **Step 4: Smoke — full home page**

```bash
npm run dev
```

Open `http://localhost:3000`. Expected: Hero → PrimaryFeatures → SecondaryFeatures → CallToAction → PricingTeaser → Faqs → Footer in order. No console errors. Stop dev server.

- [ ] **Step 5: Lint + typecheck + build**

```bash
npm run lint && npm run typecheck && npm run build
```

Expected: build succeeds, `landing/out/index.html` produced.

- [ ] **Step 6: Commit**

```bash
git add landing/src/components/Faqs.tsx landing/src/app/page.tsx
git commit -m "feat(landing): Faqs + home page composition; drop unused Salient bits

Removes Pricing, Testimonials, SlimLayout, Fields components from the
Salient template — Raven uses PricingTeaser/PricingTable instead and
the marketing site has no auth or testimonials surface."
```

---

### Task 11: `/pricing` page

**Files:**
- Create: `landing/src/app/pricing/page.tsx`
- Create: `landing/src/components/PricingTable.tsx`
- Create: `landing/src/components/PricingFaqs.tsx`

- [ ] **Step 1: Create `landing/src/components/PricingTable.tsx`**

```tsx
import { Button } from '@/components/Button'
import { Container } from '@/components/Container'

const plans = [
  {
    name: 'Self-Hosted',
    price: '₹0',
    cadence: 'forever',
    description: 'Run on your own hardware. No upsell wall, no telemetry.',
    cta: { label: 'Self-host in 5 min', href: '/self-host', variant: 'solid' as const, color: 'ink' as const },
    features: [
      'Unlimited workspaces, users, documents',
      'Voice + chat + channel ingestion',
      'pgvector + BM25 hybrid search',
      'Bring your own models (Ollama, OpenAI, Anthropic, …)',
      'Community support on GitHub',
    ],
  },
  {
    name: 'Cloud Starter',
    price: '₹X',
    cadence: 'per seat / month',
    description: 'Managed by us, in the region of your choice.',
    cta: { label: 'Talk to us', href: 'mailto:hello@ravencloak.org', variant: 'solid' as const, color: 'accent' as const },
    features: [
      'Everything in Self-Hosted',
      'Managed PostgreSQL, Valkey, LiveKit',
      '99.9% uptime SLA',
      'Email support',
    ],
    highlight: true,
  },
  {
    name: 'Cloud Pro',
    price: '₹Y',
    cadence: 'per seat / month',
    description: 'For teams that need SSO, audit logs, and priority support.',
    cta: { label: 'Talk to us', href: 'mailto:hello@ravencloak.org', variant: 'outline' as const, color: 'ink' as const },
    features: [
      'Everything in Cloud Starter',
      'SSO (SAML, OIDC)',
      'Immutable audit logs',
      'Priority support, named contact',
      'Custom data-residency on request',
    ],
  },
]

export function PricingTable() {
  return (
    <section className="bg-[var(--color-bg)] py-20 sm:py-32">
      <Container>
        <div className="mx-auto max-w-3xl text-center">
          <h1 className="font-display text-4xl tracking-tight text-[var(--color-ink)] sm:text-5xl">
            Pricing
          </h1>
          <p className="mt-4 text-lg tracking-tight text-[var(--color-body)]">
            Self-host for free. Pay us only if you want us to run it for you.
          </p>
        </div>
        <div className="mx-auto mt-16 grid max-w-7xl grid-cols-1 gap-8 lg:grid-cols-3">
          {plans.map((p) => (
            <div
              key={p.name}
              className={
                p.highlight
                  ? 'rounded-3xl bg-[var(--color-ink)] p-10 text-white ring-1 ring-[var(--color-ink)]'
                  : 'rounded-3xl bg-white p-10 text-[var(--color-ink)] ring-1 ring-[var(--color-border)]'
              }
            >
              <h3 className="font-display text-xl">{p.name}</h3>
              <p className={p.highlight ? 'mt-2 text-white/70' : 'mt-2 text-[var(--color-body)]'}>
                {p.description}
              </p>
              <p className="mt-6 font-display text-4xl">
                {p.price}{' '}
                <span className={p.highlight ? 'text-base text-white/70' : 'text-base text-[var(--color-body)]'}>
                  {p.cadence}
                </span>
              </p>
              <Button
                href={p.cta.href}
                variant={p.cta.variant}
                color={p.cta.color}
                className="mt-8 w-full"
              >
                {p.cta.label}
              </Button>
              <ul className={p.highlight ? 'mt-8 space-y-3 text-sm text-white/80' : 'mt-8 space-y-3 text-sm text-[var(--color-body)]'}>
                {p.features.map((f) => (
                  <li key={f} className="flex gap-x-3">
                    <span aria-hidden="true">✓</span>
                    <span>{f}</span>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
        <p className="mt-12 text-center text-sm text-[var(--color-body)]">
          Cloud pricing in INR; payment via Hyperswitch (UPI / RuPay / Razorpay). Contact us for non-INR billing.
        </p>
      </Container>
    </section>
  )
}
```

- [ ] **Step 2: Create `landing/src/components/PricingFaqs.tsx`**

```tsx
import { Container } from '@/components/Container'

const items = [
  {
    q: 'Can I move from Cloud to Self-Hosted?',
    a: 'Yes. We provide a one-shot export tool that reproduces your workspaces, documents, embeddings, and audit history on a fresh self-hosted instance.',
  },
  {
    q: 'Can I move from Self-Hosted to Cloud?',
    a: 'Yes. The same tool runs in reverse.',
  },
  {
    q: 'How are seats counted?',
    a: 'A seat is one human user with sign-in access. Service accounts and read-only API consumers are not seats.',
  },
  {
    q: 'Do you offer a non-profit or open-source discount?',
    a: 'Yes. Email hello@ravencloak.org with a short note about your project.',
  },
]

export function PricingFaqs() {
  return (
    <section className="bg-white py-20 sm:py-32">
      <Container>
        <div className="mx-auto max-w-2xl">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)] sm:text-4xl">
            Pricing FAQs
          </h2>
          <dl className="mt-12 space-y-8">
            {items.map((it) => (
              <div key={it.q}>
                <dt className="font-display text-lg text-[var(--color-ink)]">{it.q}</dt>
                <dd className="mt-2 text-[var(--color-body)]">{it.a}</dd>
              </div>
            ))}
          </dl>
        </div>
      </Container>
    </section>
  )
}
```

- [ ] **Step 3: Create `landing/src/app/pricing/page.tsx`**

```tsx
import { type Metadata } from 'next'

import { Footer } from '@/components/Footer'
import { Header } from '@/components/Header'
import { PricingFaqs } from '@/components/PricingFaqs'
import { PricingTable } from '@/components/PricingTable'

export const metadata: Metadata = {
  title: 'Pricing',
  description:
    'Raven is free to self-host. Cloud starts from ₹X / seat / month. Compare Self-Hosted, Cloud Starter, and Cloud Pro.',
  alternates: { canonical: 'https://raven.ravencloak.org/pricing/' },
}

export default function PricingPage() {
  return (
    <>
      <Header />
      <main>
        <PricingTable />
        <PricingFaqs />
      </main>
      <Footer />
    </>
  )
}
```

- [ ] **Step 4: Smoke + lint + typecheck + commit**

```bash
npm run dev   # visit /pricing/, then stop
npm run lint && npm run typecheck
git add landing/src/app/pricing/ landing/src/components/PricingTable.tsx landing/src/components/PricingFaqs.tsx
git commit -m "feat(landing): /pricing page with three-tier table and pricing FAQs

Self-Hosted (₹0), Cloud Starter (₹X placeholder), Cloud Pro (₹Y
placeholder). Final numbers tracked as a separate follow-up; site
ships with placeholder values per spec §6."
```

---

### Task 12: `/self-host` page

**Files:**
- Create: `landing/src/app/self-host/page.tsx`
- Create: `landing/src/components/SystemRequirements.tsx`
- Create: `landing/src/components/QuickStart.tsx`
- Create: `landing/src/components/UpgradeNotes.tsx`
- Create: `landing/src/components/CommunitySupport.tsx`

- [ ] **Step 1: Create `landing/src/components/SystemRequirements.tsx`**

```tsx
import { Container } from '@/components/Container'

const rows = [
  { label: 'CPU', min: '2 cores (x86-64 or ARM64)', recommended: '4+ cores' },
  { label: 'RAM', min: '4 GB', recommended: '8 GB+ (16 GB if running local LLMs)' },
  { label: 'Disk', min: '20 GB SSD', recommended: '100 GB+ for embeddings + objects' },
  { label: 'OS', min: 'Linux with Docker 24+', recommended: 'Ubuntu 24.04 LTS or Debian 13' },
  { label: 'Network', min: 'Outbound to your model provider', recommended: 'Same; or fully air-gapped with Ollama' },
]

export function SystemRequirements() {
  return (
    <section className="bg-white py-20 sm:py-28">
      <Container>
        <div className="mx-auto max-w-3xl">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)]">
            System requirements
          </h2>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            Raven runs comfortably on a modest VPS or a Raspberry Pi 5. The numbers
            below are guidance — the actual footprint depends on the embedding model
            and corpus size.
          </p>
          <table className="mt-10 w-full text-left text-sm">
            <thead className="border-b border-[var(--color-border)] text-[var(--color-ink)]">
              <tr>
                <th className="pb-3 font-display font-medium">Resource</th>
                <th className="pb-3 font-display font-medium">Minimum</th>
                <th className="pb-3 font-display font-medium">Recommended</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {rows.map((r) => (
                <tr key={r.label}>
                  <td className="py-3 font-display text-[var(--color-ink)]">{r.label}</td>
                  <td className="py-3 text-[var(--color-body)]">{r.min}</td>
                  <td className="py-3 text-[var(--color-body)]">{r.recommended}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Container>
    </section>
  )
}
```

- [ ] **Step 2: Create `landing/src/components/QuickStart.tsx`**

```tsx
import { Container } from '@/components/Container'

const compose = `# 1. Grab the production Compose file
curl -fsSL https://raven.ravencloak.org/compose.yml -o docker-compose.yml

# 2. Generate secrets and start
docker compose up -d

# 3. Open the app
open http://localhost:8080
`

export function QuickStart() {
  return (
    <section className="bg-[var(--color-bg)] py-20 sm:py-28">
      <Container>
        <div className="mx-auto max-w-3xl">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)]">
            Five-minute quick start
          </h2>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            One command brings up Raven, PostgreSQL with pgvector, Valkey, and the AI worker.
            Out of the box it points at Ollama on the host; swap in OpenAI/Anthropic by editing one env file.
          </p>
          <pre className="mt-8 overflow-x-auto rounded-2xl bg-[var(--color-ink)] p-6 font-mono text-sm leading-relaxed text-[var(--color-bg)]">
            <code>{compose}</code>
          </pre>
          <p className="mt-6 text-sm text-[var(--color-body)]">
            Full guide and edge / Raspberry Pi variants:{' '}
            <a className="text-[var(--color-accent)] underline" href="https://github.com/ravencloak-org/Raven#self-hosting">
              github.com/ravencloak-org/Raven
            </a>.
          </p>
        </div>
      </Container>
    </section>
  )
}
```

- [ ] **Step 3: Create `landing/src/components/UpgradeNotes.tsx`**

```tsx
import { Container } from '@/components/Container'

export function UpgradeNotes() {
  return (
    <section className="bg-white py-20 sm:py-28">
      <Container>
        <div className="mx-auto max-w-3xl">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)]">
            Upgrades and rollbacks
          </h2>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            Every release is tagged with semver and ships its own migration plan.
            Upgrade in place with <code className="font-mono text-[var(--color-ink)]">docker compose pull &amp;&amp; docker compose up -d</code>.
            Roll back by pinning the previous tag in your Compose file —
            migrations are forward-only but always backward-compatible within a minor version.
          </p>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            Subscribe to release notifications by watching the GitHub repo, or follow{' '}
            <a className="text-[var(--color-accent)] underline" href="https://github.com/ravencloak-org/Raven/releases">
              the release feed
            </a>.
          </p>
        </div>
      </Container>
    </section>
  )
}
```

- [ ] **Step 4: Create `landing/src/components/CommunitySupport.tsx`**

```tsx
import { Container } from '@/components/Container'

export function CommunitySupport() {
  return (
    <section className="bg-[var(--color-bg)] py-20 sm:py-28">
      <Container>
        <div className="mx-auto max-w-3xl">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)]">
            Community support
          </h2>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            Self-hosted Raven is fully supported by the open community. Open an issue or
            a discussion on GitHub — the maintainers and other operators are usually
            responsive within a day.
          </p>
          <ul className="mt-6 list-disc space-y-2 pl-6 text-[var(--color-body)]">
            <li>
              Bugs / feature requests:{' '}
              <a className="text-[var(--color-accent)] underline" href="https://github.com/ravencloak-org/Raven/issues">
                GitHub Issues
              </a>
            </li>
            <li>
              Operational questions, pattern advice:{' '}
              <a className="text-[var(--color-accent)] underline" href="https://github.com/ravencloak-org/Raven/discussions">
                GitHub Discussions
              </a>
            </li>
            <li>Need a paid SLA? <a className="text-[var(--color-accent)] underline" href="/pricing/">See Cloud Pro</a>.</li>
          </ul>
        </div>
      </Container>
    </section>
  )
}
```

- [ ] **Step 5: Create `landing/src/app/self-host/page.tsx`**

```tsx
import { type Metadata } from 'next'

import { Container } from '@/components/Container'
import { Footer } from '@/components/Footer'
import { Header } from '@/components/Header'
import { CommunitySupport } from '@/components/CommunitySupport'
import { QuickStart } from '@/components/QuickStart'
import { SystemRequirements } from '@/components/SystemRequirements'
import { UpgradeNotes } from '@/components/UpgradeNotes'

export const metadata: Metadata = {
  title: 'Self-host',
  description:
    'Run Raven on your own infrastructure in five minutes. System requirements, one-command Docker Compose, upgrade notes, and community support.',
  alternates: { canonical: 'https://raven.ravencloak.org/self-host/' },
}

export default function SelfHostPage() {
  return (
    <>
      <Header />
      <main>
        <Container className="pt-20 pb-10 text-center lg:pt-28">
          <h1 className="mx-auto max-w-3xl font-display text-4xl tracking-tight text-[var(--color-ink)] sm:text-5xl">
            Self-host Raven on your own infrastructure.
          </h1>
          <p className="mx-auto mt-6 max-w-2xl text-lg text-[var(--color-body)]">
            One Docker Compose command. No telemetry, no callback to a vendor. Runs on
            anything from a 4-GB VPS to a Raspberry Pi 5.
          </p>
        </Container>
        <SystemRequirements />
        <QuickStart />
        <UpgradeNotes />
        <CommunitySupport />
      </main>
      <Footer />
    </>
  )
}
```

- [ ] **Step 6: Smoke + lint + typecheck + commit**

```bash
npm run dev   # visit /self-host/, then stop
npm run lint && npm run typecheck
git add landing/src/app/self-host/ \
        landing/src/components/SystemRequirements.tsx \
        landing/src/components/QuickStart.tsx \
        landing/src/components/UpgradeNotes.tsx \
        landing/src/components/CommunitySupport.tsx
git commit -m "feat(landing): /self-host page (requirements, quickstart, upgrades, support)"
```

---

### Task 13: `/about` page

**Files:**
- Create: `landing/src/app/about/page.tsx`
- Create: `landing/src/components/Mission.tsx`
- Create: `landing/src/components/Maintainers.tsx`

- [ ] **Step 1: Create `landing/src/components/Mission.tsx`**

```tsx
import { Container } from '@/components/Container'

export function Mission() {
  return (
    <section className="bg-white py-20 sm:py-32">
      <Container>
        <div className="mx-auto max-w-3xl">
          <h1 className="font-display text-4xl tracking-tight text-[var(--color-ink)] sm:text-5xl">
            Knowledge platforms shouldn't be data brokers.
          </h1>
          <p className="mt-6 text-lg text-[var(--color-body)]">
            Raven exists because most "AI for your team" products are wrappers that
            ship your documents to someone else's servers, lock you into one model
            vendor, and treat self-hosting as a nice-to-have at the enterprise tier.
            We think it should be the default.
          </p>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            Raven is a self-hostable, multi-tenant RAG platform with first-class
            voice and chat surfaces. It runs on your hardware, with your models,
            under your access controls — and it does that on a Raspberry Pi just as
            well as it does on a fleet of VMs.
          </p>
        </div>
      </Container>
    </section>
  )
}
```

- [ ] **Step 2: Create `landing/src/components/Maintainers.tsx`**

```tsx
import { Container } from '@/components/Container'

export function Maintainers() {
  return (
    <section className="bg-[var(--color-bg)] py-20 sm:py-28">
      <Container>
        <div className="mx-auto max-w-3xl">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)]">
            Who's behind it
          </h2>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            Raven is maintained by Jobin Lawrance — previously building Keycloak
            Service Provider Interfaces, now full-time on auth and platform
            infrastructure. Based in Bengaluru, India.
          </p>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            The codebase is open on{' '}
            <a className="text-[var(--color-accent)] underline" href="https://github.com/ravencloak-org/Raven">
              GitHub
            </a>
            . Contributions, issues, and pattern discussions welcome.
          </p>
          <p className="mt-8 text-sm text-[var(--color-body)]">
            Source-available under the Raven licence; see{' '}
            <a className="text-[var(--color-accent)] underline" href="https://github.com/ravencloak-org/Raven/blob/main/LICENSE">
              LICENSE
            </a>{' '}
            and{' '}
            <a className="text-[var(--color-accent)] underline" href="https://github.com/ravencloak-org/Raven/blob/main/ee-LICENSE">
              ee-LICENSE
            </a>.
          </p>
        </div>
      </Container>
    </section>
  )
}
```

- [ ] **Step 3: Create `landing/src/app/about/page.tsx`**

```tsx
import { type Metadata } from 'next'

import { Footer } from '@/components/Footer'
import { Header } from '@/components/Header'
import { Maintainers } from '@/components/Maintainers'
import { Mission } from '@/components/Mission'

export const metadata: Metadata = {
  title: 'About',
  description:
    'Why Raven exists, and who maintains it. A self-hostable, multi-tenant RAG platform built by Jobin Lawrance.',
  alternates: { canonical: 'https://raven.ravencloak.org/about/' },
}

export default function AboutPage() {
  return (
    <>
      <Header />
      <main>
        <Mission />
        <Maintainers />
      </main>
      <Footer />
    </>
  )
}
```

- [ ] **Step 4: Smoke + lint + typecheck + commit**

```bash
npm run dev   # visit /about/, then stop
npm run lint && npm run typecheck
git add landing/src/app/about/ landing/src/components/Mission.tsx landing/src/components/Maintainers.tsx
git commit -m "feat(landing): /about page (mission + maintainers)"
```

---

### Task 14: 404 page

**Files:**
- Modify: `landing/src/app/not-found.tsx`

- [ ] **Step 1: Replace `landing/src/app/not-found.tsx`**

Overwrite the file:

```tsx
import Link from 'next/link'

import { Container } from '@/components/Container'
import { Footer } from '@/components/Footer'
import { Header } from '@/components/Header'

export default function NotFound() {
  return (
    <>
      <Header />
      <main>
        <Container className="flex min-h-[60vh] flex-col items-center justify-center text-center">
          <p className="font-mono text-sm text-[var(--color-body)]">404</p>
          <h1 className="mt-4 font-display text-4xl tracking-tight text-[var(--color-ink)] sm:text-5xl">
            Page not found
          </h1>
          <p className="mt-4 max-w-md text-lg text-[var(--color-body)]">
            That URL doesn't exist on raven.ravencloak.org. Try the home page or the
            self-host guide.
          </p>
          <div className="mt-10 flex gap-x-4">
            <Link href="/" className="text-[var(--color-accent)] underline">Home</Link>
            <Link href="/self-host" className="text-[var(--color-accent)] underline">Self-host</Link>
          </div>
        </Container>
      </main>
      <Footer />
    </>
  )
}
```

- [ ] **Step 2: Smoke + lint + typecheck + commit**

```bash
npm run dev   # visit /no-such-page, then stop
npm run lint && npm run typecheck
git add landing/src/app/not-found.tsx
git commit -m "feat(landing): branded 404 page"
```

---

### Task 15: SEO — sitemap, robots, OpenGraph image

**Files:**
- Create: `landing/public/robots.txt`
- Create: `landing/public/sitemap.xml`
- Create: `landing/src/app/opengraph-image.tsx`

- [ ] **Step 1: Create `landing/public/robots.txt`**

```
User-agent: *
Allow: /
Sitemap: https://raven.ravencloak.org/sitemap.xml
```

- [ ] **Step 2: Create `landing/public/sitemap.xml`**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://raven.ravencloak.org/</loc><changefreq>weekly</changefreq><priority>1.0</priority></url>
  <url><loc>https://raven.ravencloak.org/pricing/</loc><changefreq>monthly</changefreq><priority>0.8</priority></url>
  <url><loc>https://raven.ravencloak.org/self-host/</loc><changefreq>monthly</changefreq><priority>0.9</priority></url>
  <url><loc>https://raven.ravencloak.org/about/</loc><changefreq>yearly</changefreq><priority>0.5</priority></url>
</urlset>
```

- [ ] **Step 3: Create `landing/src/app/opengraph-image.tsx`**

```tsx
import { ImageResponse } from 'next/og'
import { readFileSync } from 'node:fs'
import path from 'node:path'

export const alt = 'Raven — Self-hostable AI knowledge platform for teams'
export const size = { width: 1200, height: 630 }
export const contentType = 'image/png'

export default async function OG() {
  const ravenicons = readFileSync(
    path.join(process.cwd(), 'public/fonts/ravenicons.woff2'),
  )

  return new ImageResponse(
    (
      <div
        style={{
          height: '100%',
          width: '100%',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          background: 'oklch(0.18 0 0)',
          color: 'white',
        }}
      >
        <div
          style={{
            fontFamily: 'ravenicons',
            fontSize: 220,
            letterSpacing: 18,
            lineHeight: 1,
          }}
        >
          RAVEN
        </div>
        <div
          style={{
            marginTop: 40,
            fontSize: 36,
            color: 'oklch(0.71 0.16 200)',
            maxWidth: 980,
            textAlign: 'center',
            fontFamily: 'sans-serif',
          }}
        >
          Your team's knowledge, on your infrastructure.
        </div>
      </div>
    ),
    {
      ...size,
      fonts: [{ name: 'ravenicons', data: ravenicons, weight: 400, style: 'normal' }],
    },
  )
}
```

Note: no `runtime = 'edge'` — under `output: 'export'`, the Node default runtime renders the OG image at build time, which lets us read the `.woff2` from disk.

- [ ] **Step 4: Smoke + lint + typecheck + build**

```bash
npm run lint && npm run typecheck && npm run build
```

Expected: build succeeds; `landing/out/` contains `opengraph-image.png`, `sitemap.xml`, `robots.txt`.

- [ ] **Step 5: Commit**

```bash
git add landing/public/robots.txt landing/public/sitemap.xml landing/src/app/opengraph-image.tsx
git commit -m "feat(landing): robots.txt, sitemap.xml, OpenGraph image"
```

---

### Task 16: Cloudflare security headers

**Files:**
- Create: `landing/public/_headers`

- [ ] **Step 1: Create `landing/public/_headers`**

```
/*
  Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
  X-Content-Type-Options: nosniff
  X-Frame-Options: DENY
  Referrer-Policy: strict-origin-when-cross-origin
  Permissions-Policy: camera=(), microphone=(), geolocation=()
  Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'
```

- [ ] **Step 2: Build and verify the headers file is in `out/`**

```bash
npm run build
ls landing/out/_headers
```

Expected: file exists.

- [ ] **Step 3: Commit**

```bash
git add landing/public/_headers
git commit -m "feat(landing): Cloudflare Pages security headers (HSTS, CSP, …)"
```

---

### Task 17: Playwright smoke tests

**Files:**
- Create: `landing/playwright.config.ts`
- Create: `landing/tests/smoke.spec.ts`
- Modify: `landing/package.json` (add `@playwright/test` dev-dep + `serve` for the static export)
- Create: `landing/tests/__screenshots__/` (auto-created on first run)

- [ ] **Step 1: Add Playwright + serve as devDeps**

```bash
cd landing
npm install --save-dev @playwright/test serve
npx playwright install --with-deps chromium
```

- [ ] **Step 2: Create `landing/playwright.config.ts`**

```ts
import { defineConfig, devices } from '@playwright/test'

const PORT = 4173

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  fullyParallel: true,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: `http://127.0.0.1:${PORT}`,
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile', use: { ...devices['Pixel 5'] } },
  ],
  webServer: {
    command: `npx serve out -l ${PORT} --no-clipboard --no-port-switching`,
    url: `http://127.0.0.1:${PORT}`,
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
})
```

- [ ] **Step 3: Write the failing smoke test**

Create `landing/tests/smoke.spec.ts`:

```ts
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

test.describe('visual', () => {
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
```

- [ ] **Step 4: Run the test to verify the toolchain works (snapshots will be created on first run)**

```bash
npm run build
npm test -- --update-snapshots
```

Expected: First run writes baseline snapshots under `tests/smoke.spec.ts-snapshots/`. Inspect a couple of them visually before committing — if they look broken (blank page, missing fonts), pause and fix the underlying page rather than committing the broken baseline.

- [ ] **Step 5: Re-run without `--update-snapshots` to confirm green**

```bash
npm test
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add landing/playwright.config.ts landing/tests/ landing/package.json landing/package-lock.json
git commit -m "test(landing): Playwright smoke tests + visual baselines

For each of /, /pricing/, /self-host/, /about/, plus the 404 page:
status 200, header/footer visible, no console errors, internal links
resolve, full-page screenshot diff against baseline (2% pixel
tolerance) on desktop and mobile viewports. Tests run against the
static export served by 'npx serve out'."
```

---

### Task 18: Update `landing.yml` workflow

**Files:**
- Modify: `.github/workflows/landing.yml`

- [ ] **Step 1: Replace `.github/workflows/landing.yml`**

Overwrite the file:

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
    name: Build & Deploy
    runs-on: ubuntu-latest
    permissions:
      contents: read
      deployments: write
      pull-requests: write
    steps:
      - name: Checkout
        uses: actions/checkout@v6

      - name: Setup Node.js
        uses: actions/setup-node@v6
        with:
          node-version: "22"
          cache: "npm"
          cache-dependency-path: landing/package-lock.json

      - name: Install dependencies
        working-directory: landing
        run: npm ci

      - name: Lint
        working-directory: landing
        run: npm run lint

      - name: Typecheck
        working-directory: landing
        run: npm run typecheck

      - name: Build
        working-directory: landing
        run: npm run build

      - name: Install Playwright browsers
        working-directory: landing
        run: npx playwright install --with-deps chromium

      - name: Smoke tests
        working-directory: landing
        run: npm test

      - name: Upload Playwright report on failure
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: playwright-report
          path: landing/playwright-report/
          retention-days: 7

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

- [ ] **Step 2: Validate the YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/landing.yml'))"
```

Expected: no output (= valid YAML).

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/landing.yml
git commit -m "ci(landing): build, typecheck, smoke-test, then deploy to CF Pages

Replaces the previous workflow that uploaded the raw landing/ folder
with a full pipeline: install → lint → typecheck → next build →
Playwright smoke → wrangler pages deploy landing/out. PR runs do
everything except deploy. Uploads Playwright report as an artifact
when smoke fails."
```

---

### Task 19: Final local verification, push, PR, queue auto-merge

**Files:** none (workflow gate)

- [ ] **Step 1: Final local gate**

```bash
cd /Users/jobinlawrance/Project/raven/.worktrees/landing-salient/landing
rm -rf .next out
npm ci
npm run lint
npm run typecheck
npm run build
npm test
```

Expected: all five steps green. If anything fails, fix it before pushing.

- [ ] **Step 2: Manual click-through (per repo testing-gate rule)**

```bash
npx serve out -l 4173
```

Open `http://127.0.0.1:4173` in a real browser. Click through every nav link, every CTA, both viewports (desktop + mobile via DevTools). No broken links, no layout breaks. Stop the server.

- [ ] **Step 3: Push the branch**

```bash
cd /Users/jobinlawrance/Project/raven/.worktrees/landing-salient
git push -u origin feat/landing-salient-rebuild
```

- [ ] **Step 4: Open the PR**

```bash
gh pr create --title "feat(landing): rebuild marketing site on Tailwind Plus Salient" --body "$(cat <<'EOF'
## Summary
- Replace `raven/landing/` (62 KB hand-written `index.html` + Tailwind CLI v4) with a Next.js 15 / Tailwind v4 / Headless UI marketing site derived from the Tailwind Plus Salient template, retheming the visual language to Raven (electric-cyan accent on a monochrome base, Inter + Space Grotesk + JetBrains Mono fonts, new logo lockup and favicon family).
- 4 pages: `/` (Hero · 3-up features · tabbed surfaces · CTA · pricing teaser · FAQs), `/pricing` (3-tier table + pricing FAQs), `/self-host` (system requirements · 5-min Docker Compose quickstart · upgrade notes · community support), `/about` (mission + maintainers). Branded 404.
- Static export (`output: 'export'`) → `landing/out/` deployed unchanged to the existing `raven-landing` Cloudflare Pages project at `raven.ravencloak.org` (no DNS work needed).
- Playwright smoke tests: status, console errors, internal-link resolution, and full-page visual diffs (desktop + mobile) for every page; all gated in CI before deploy.
- Updated `.github/workflows/landing.yml`: install → lint → typecheck → build → Playwright → wrangler deploy.
- `public/_headers` ships HSTS, X-Frame-Options DENY, conservative CSP.
- Spec: `docs/superpowers/specs/2026-05-06-raven-marketing-site-design.md`. Implementation plan: `docs/superpowers/plans/2026-05-07-raven-marketing-site.md`.

Pricing numbers (`₹X` / `₹Y`) and the hero illustration are tracked as separate follow-ups per spec §12 — placeholders ship.

## Test plan
- [ ] CI green (lint, typecheck, build, Playwright smoke)
- [ ] Cloudflare Pages preview URL renders all 4 pages
- [ ] Production deploy after merge: visit https://raven.ravencloak.org/ and verify Hero, /pricing/, /self-host/, /about/, /sitemap.xml, /robots.txt, /og-image, /favicon.ico
- [ ] Lighthouse mobile ≥ 90 across Performance / Accessibility / Best Practices / SEO on the production URL
EOF
)"
```

- [ ] **Step 5: Queue auto-squash-merge (per CLAUDE.md)**

```bash
gh pr merge $(gh pr view --json number -q .number) --auto --squash
```

- [ ] **Step 6: Verify production after merge**

After the PR auto-merges and `landing.yml` runs on `main`, open `https://raven.ravencloak.org/` and:

- All 4 pages return 200
- `/sitemap.xml`, `/robots.txt`, `/favicon.ico`, `/opengraph-image.png` all return 200
- View-source shows `<meta name="description">` per spec
- Header sticky logo + nav, footer dark with mark
- Run a one-shot Lighthouse on the home page; record the score in the PR comments

If any of the above is broken, `git revert` the merge commit and push — Cloudflare Pages will redeploy the previous build automatically (per spec §11 rollback).

---

## Self-review notes

Skimmed against the spec:

- **§4 architecture / file structure** — covered by the file table at top + per-task file lists.
- **§5 branding (palette, fonts, logo)** — T2 (palette + fonts), T3 (logo + favicons).
- **§6 IA (4 pages + sections)** — T5–T14.
- **§7 build pipeline + Cloudflare Pages target** — T1 (`next.config.mjs`), T16 (headers), T18 (workflow).
- **§8 SEO + metadata** — T2 (root metadata), each page task (per-page metadata), T15 (sitemap/robots/OG).
- **§9 a11y** — `lang="en"` in T2, semantic landmarks throughout, alt text on every Image, focus rings via `--color-accent` token in T4.
- **§10 testing** — T17 + T18 (CI gates) + T19 step 2 (manual).
- **§11 branch / rollout** — T19.
- **§12 out of scope** — placeholders intentional and labelled (`₹X` / `₹Y`, hero illustration uses logo mark, no dark mode, no testimonials/logo-cloud).
- **§13 acceptance criteria** — T19 step 6 covers points 6 + 7; points 1–5 covered by tasks 1–18.

No placeholder strings ("TBD", "implement later", "add appropriate error handling") in steps. Snippets are complete. Type names consistent: `Button`'s `color` prop accepts `'accent'` or `'ink'` everywhere; `Logo`'s `variant` prop accepts `'full'` or `'mark'` everywhere.
