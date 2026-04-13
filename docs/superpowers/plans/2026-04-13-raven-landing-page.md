# Raven Landing Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a production-ready marketing landing page for Raven at raven.ravencloak.org using Tailwind Plus components, with dark/light theme, black/white/amber color scheme, and an embedded demo chat widget.

**Architecture:** Static HTML page built with Tailwind CSS v4 + Tailwind Plus marketing components. The page is a single `index.html` with a Tailwind CLI build step for CSS purging. Theme switching via `dark:` class on `<html>`. The `<raven-chat>` web component is loaded as an external script for the hero demo section.

**Tech Stack:** HTML, Tailwind CSS v4 (Plus license), vanilla JavaScript (theme toggle, tabs, scroll animations, mobile menu), Inter font via Google Fonts, Heroicons for icons.

**Spec:** `docs/superpowers/specs/2026-04-13-raven-landing-page-design.md`

---

## File Structure

```
landing/
├── index.html              # Full landing page (all 10 sections)
├── src/
│   └── input.css           # Tailwind entry point (@import "tailwindcss")
├── dist/
│   └── output.css          # Built/purged Tailwind CSS (gitignored)
├── assets/
│   ├── raven-logo-dark.svg # White logo for dark theme
│   ├── raven-logo-light.svg# Black logo for light theme
│   ├── og-image.png        # 1200x630 Open Graph image
│   └── screenshots/        # Product screenshots (chat, voice, whatsapp)
│       ├── chat.png
│       ├── voice.png
│       └── whatsapp.png
└── package.json            # Tailwind CLI build scripts
```

**Notes:**
- `landing/` is the existing directory — we replace `index.html` wholesale (the old page uses CDN Tailwind play script + indigo theme — do NOT carry over any of it)
- Tailwind v4 uses CSS-based config via `@theme` in `input.css` — there is NO `tailwind.config.js`
- `dist/output.css` is generated and gitignored
- Screenshots can be placeholder images initially; replaced with real captures later
- The hero demo widget is a **hardcoded HTML mockup** for Phase 1 launch — the real `<raven-chat>` web component will be wired in Phase 2 once the SSE chat API is implemented
- Tailwind Plus components are React/Vue source — extract the **HTML structure and Tailwind classes only**, strip any framework-specific markup (`className` → `class`, no JSX)
- Icons: copy inline SVG markup from [heroicons.com](https://heroicons.com) — do NOT install `@heroicons/react` or any npm icon package

---

### Task 1: Project Scaffolding & Tailwind Build

**Files:**
- Create: `landing/package.json`
- Create: `landing/src/input.css`
- Modify: `landing/.gitignore` (create if not exists)

- [ ] **Step 1: Initialize package.json with Tailwind CLI**

```json
{
  "name": "raven-landing",
  "private": true,
  "scripts": {
    "dev": "npx @tailwindcss/cli -i src/input.css -o dist/output.css --watch",
    "build": "npx @tailwindcss/cli -i src/input.css -o dist/output.css --minify"
  },
  "devDependencies": {
    "@tailwindcss/cli": "^4.2.2"
  }
}
```

- [ ] **Step 2: Create Tailwind input CSS**

```css
/* landing/src/input.css */
@import "tailwindcss";
@source "../";

@theme {
  --font-sans: "Inter", ui-sans-serif, system-ui, sans-serif;
}
```

The `@source "../"` directive tells Tailwind v4 CLI to scan `landing/` (parent of `src/`) for class usage in HTML files. Without this, the CLI scans from `landing/src/` and may miss `index.html`. Tailwind v4's default palette already includes all amber colors, so no custom color tokens are needed.

- [ ] **Step 3: Create .gitignore**

```
node_modules/
dist/
```

- [ ] **Step 4: Install dependencies and test build**

```bash
cd landing && npm install && npm run build
```

Expected: `dist/output.css` is generated with minimal Tailwind output.

- [ ] **Step 5: Commit**

```bash
git add landing/package.json landing/src/input.css landing/.gitignore
git commit -m "feat(landing): scaffold Tailwind CSS v4 build tooling"
```

---

### Task 2: HTML Skeleton with SEO & Theme Toggle

**Files:**
- Modify: `landing/index.html` (replace entirely)

- [ ] **Step 1: Create the base HTML document**

Write `landing/index.html` with:
- DOCTYPE, lang, `<head>` with all meta tags from spec (title, description, OG tags, canonical, twitter card)
- Google Fonts preconnect + Inter font load with `font-display: swap`
- Link to `dist/output.css`
- Theme detection script in `<head>` (before body renders to prevent flash):

```html
<script>
  if (localStorage.theme === 'dark' || (!('theme' in localStorage) && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
</script>
```

- `<body>` with `class="bg-white dark:bg-black text-black dark:text-white transition-colors duration-300"` — also add `@media (prefers-reduced-motion: reduce) { *, *::before, *::after { transition-duration: 0.01ms !important; } }` in `input.css` to disable transitions for reduced-motion users
- Skip-to-content link as first element in `<body>`: `<a href="#main-content" class="sr-only focus:not-sr-only focus:absolute focus:top-4 focus:left-4 focus:z-50 focus:bg-amber-400 focus:text-black focus:px-4 focus:py-2 focus:rounded-lg">Skip to content</a>`
- Empty `<nav>`, `<main id="main-content">` (with 8 empty `<section>` placeholders for sections 2-9), `<footer>`
- Each section has `id` and `aria-label` for accessibility
- Favicon link (placeholder)

- [ ] **Step 2: Run build and open in browser**

```bash
cd landing && npm run build && open index.html
```

Expected: Blank page with correct title in tab, dark theme if system prefers dark.

- [ ] **Step 3: Commit**

```bash
git add landing/index.html
git commit -m "feat(landing): HTML skeleton with SEO meta tags and theme detection"
```

---

### Task 3: Navigation Bar

**Files:**
- Modify: `landing/index.html` (populate `<nav>` section)

Use Tailwind Plus **Marketing > Headers** component as base. Adapt to spec:

- [ ] **Step 1: Build the desktop nav**

Sticky nav with `backdrop-blur-md bg-white/80 dark:bg-black/80 border-b border-neutral-200 dark:border-neutral-800`. Structure:
- Left: Raven logo (text placeholder "🪶 Raven" until SVG is ready, swap with `<img>` later)
- Center links: Features (`#features`), Pricing (`#pricing`), Docs (external), GitHub (external with hardcoded star count badge)
- Right: Theme toggle button (sun/moon SVG icons from Heroicons, inline), Sign In (`https://app.ravencloak.org`, ghost button), Get Started Free (`https://app.ravencloak.org/register`, amber button)

- [ ] **Step 2: Build the mobile menu**

Hamburger icon button (hidden on `md:` and above). On click, toggles a full-screen overlay menu with all nav links stacked vertically. "Get Started Free" amber button always visible in mobile nav. Close button (X icon) in top-right.

- [ ] **Step 3: Add theme toggle JavaScript**

```javascript
document.getElementById('theme-toggle').addEventListener('click', () => {
  const html = document.documentElement
  html.classList.toggle('dark')
  localStorage.theme = html.classList.contains('dark') ? 'dark' : 'light'
})
```

- [ ] **Step 4: Test nav**

```bash
cd landing && npm run build && open index.html
```

Expected: Sticky nav visible, theme toggle works, mobile menu opens/closes, all links point to correct targets.

- [ ] **Step 5: Commit**

```bash
git add landing/index.html
git commit -m "feat(landing): responsive nav bar with theme toggle and mobile menu"
```

---

### Task 4: Hero Section with Demo Chat Widget

**Files:**
- Modify: `landing/index.html` (populate hero `<section>`)

Use Tailwind Plus **Marketing > Hero Sections** (centered layout) as base.

- [ ] **Step 1: Build the centered hero content**

Structure:
- Badge: `Open Source · MIT Licensed` — `text-xs uppercase tracking-widest text-amber-400 font-medium`
- Headline: "The AI Brain for Your Entire Team" — `text-5xl sm:text-6xl lg:text-7xl font-extrabold tracking-tight`
- Subhead: spec copy — `text-base sm:text-lg text-neutral-400 dark:text-neutral-400 max-w-2xl mx-auto leading-relaxed`
- CTAs: `Get Started Free` (amber bg, black text, rounded-lg, font-semibold) + `Try Live Demo ↓` (ghost button with border, scrolls to `#demo-widget`)
- Trust bar: 4 items in a flex row with gap, `text-sm text-neutral-500`, icon + text

- [ ] **Step 2: Add the demo chat widget container**

Below the hero text, add a styled container:
- Max width `max-w-xl mx-auto`
- Card with `bg-neutral-100 dark:bg-neutral-950` styling, rounded-xl, border (light-first with dark override — there is NO `light:` variant in Tailwind)
- Header row: "💬 Try Raven — Ask anything" + "Interactive Demo" badge
- Pre-populated conversation (hardcoded HTML, not the actual widget yet):
  - User message: "How does Raven handle document ingestion?"
  - Bot response with citation styling
- Input bar with placeholder text and amber Send button
- `id="demo-widget"` for smooth scroll target

- [ ] **Step 3: Add smooth scroll for "Try Live Demo" CTA**

```javascript
document.querySelector('[href="#demo-widget"]').addEventListener('click', (e) => {
  e.preventDefault()
  document.getElementById('demo-widget').scrollIntoView({ behavior: 'smooth' })
})
```

- [ ] **Step 4: Test hero**

```bash
cd landing && npm run build && open index.html
```

Expected: Centered hero with headline, badges, CTAs, and styled chat mockup. "Try Live Demo" smooth-scrolls to widget. Both themes look correct.

- [ ] **Step 5: Commit**

```bash
git add landing/index.html
git commit -m "feat(landing): centered hero section with demo chat widget mockup"
```

---

### Task 5: Problem Statement Section

**Files:**
- Modify: `landing/index.html` (populate problem `<section>`)

Use Tailwind Plus **Marketing > Feature Sections** (icon grid) as base.

- [ ] **Step 1: Build the problem section**

Structure:
- Section headline: "Your team's knowledge is scattered. Your people are stuck." — centered, `text-3xl sm:text-4xl font-bold`
- 3 cards in `grid grid-cols-1 md:grid-cols-3 gap-8`:
  - Each card: `bg-neutral-50 dark:bg-neutral-950 border border-neutral-200 dark:border-neutral-800 rounded-xl p-6`
  - Icon (from Heroicons — DocumentText, MagnifyingGlass, ChatBubble — inline SVG)
  - Card headline: `text-xl font-semibold`
  - Card copy: `text-neutral-600 dark:text-neutral-400`
- Transition line below cards: spec copy, centered, `text-lg text-neutral-500`

- [ ] **Step 2: Test in both themes**

Expected: 3 cards side-by-side on desktop, stacked on mobile. Both themes have correct backgrounds and borders.

- [ ] **Step 3: Commit**

```bash
git add landing/index.html
git commit -m "feat(landing): problem statement section with 3 pain-point cards"
```

---

### Task 6: Tabbed Features Section

**Files:**
- Modify: `landing/index.html` (populate features `<section>`)

This requires **custom tab JS** — no direct Tailwind Plus drop-in for interactive tabs. Use Tailwind Plus **Feature Sections** layout for each tab's content, and **Application UI > Tabs** for the tab bar styling.

- [ ] **Step 1: Build the tab bar**

3 tabs: "AI Chat", "Voice Agent", "WhatsApp". Active tab has `border-b-2 border-amber-400 text-amber-400` indicator. Inactive tabs: `text-neutral-500 hover:text-neutral-300`.

- [ ] **Step 2: Build the 3 tab panels**

Each panel (initially hidden except first):
- Split layout: screenshot placeholder on one side (a styled `div` with `bg-neutral-900 rounded-xl aspect-video` and centered text "Screenshot: [feature name]"), description on the other
- Headline, copy, and a small "Learn more →" link
- Wrap in a container with `data-tab="chat|voice|whatsapp"` attribute

- [ ] **Step 3: Add tab switching JavaScript**

```javascript
document.querySelectorAll('[data-tab-trigger]').forEach(tab => {
  tab.addEventListener('click', () => {
    // Remove active from all triggers
    document.querySelectorAll('[data-tab-trigger]').forEach(t => {
      t.classList.remove('border-amber-400', 'text-amber-400')
      t.classList.add('text-neutral-500')
    })
    // Add active to clicked
    tab.classList.add('border-amber-400', 'text-amber-400')
    tab.classList.remove('text-neutral-500')
    // Show matching panel, hide others
    const target = tab.dataset.tabTrigger
    document.querySelectorAll('[data-tab-panel]').forEach(p => {
      p.classList.toggle('hidden', p.dataset.tabPanel !== target)
    })
  })
})
```

- [ ] **Step 4: Add mobile accordion behavior**

Use **two separate markup structures**: the tab bar (hidden on mobile via `hidden md:flex`) and a `<details>/<summary>` accordion (hidden on desktop via `md:hidden`). Each `<details>` element contains the same content as its corresponding tab panel. This avoids complex conditional JS — CSS handles the switch:

```html
<!-- Desktop tabs (hidden on mobile) -->
<div class="hidden md:flex border-b border-neutral-800 gap-8">
  <button data-tab-trigger="chat" class="...">AI Chat</button>
  <!-- ... -->
</div>
<div class="hidden md:block">
  <div data-tab-panel="chat"><!-- content --></div>
  <!-- ... -->
</div>

<!-- Mobile accordion (hidden on desktop) -->
<div class="md:hidden space-y-4">
  <details open>
    <summary class="cursor-pointer font-semibold text-lg py-3 border-b border-neutral-800">AI Chat</summary>
    <div class="py-4"><!-- same content --></div>
  </details>
  <!-- ... -->
</div>
```

- [ ] **Step 5: Test tabs**

Expected: Desktop — 3 horizontal tabs, click switches content with correct highlight. Mobile — accordion with toggleable panels.

- [ ] **Step 6: Commit**

```bash
git add landing/index.html
git commit -m "feat(landing): tabbed features section with chat/voice/whatsapp panels"
```

---

### Task 7: How It Works Section

**Files:**
- Modify: `landing/index.html` (populate how-it-works `<section>`)

**Custom layout** — 3 numbered steps connected by a line.

- [ ] **Step 1: Build the steps layout**

Structure:
- Section headline: centered
- 3 cards in `grid grid-cols-1 md:grid-cols-3 gap-8 relative`:
  - Each card: number badge (1/2/3 in `w-10 h-10 rounded-full bg-amber-400 text-black font-bold flex items-center justify-center`), headline, description, muted technical detail below
  - On desktop: a horizontal connecting line between cards (using `::after` pseudo-element or a positioned `<div>` with `h-px bg-neutral-800` between step numbers)

- [ ] **Step 2: Test responsive behavior**

Expected: Horizontal 3-step flow on desktop. On mobile, cards stack vertically with a vertical line connecting them (or no line — cleaner).

- [ ] **Step 3: Commit**

```bash
git add landing/index.html
git commit -m "feat(landing): how-it-works 3-step flow section"
```

---

### Task 8: Deploy Your Way Section

**Files:**
- Modify: `landing/index.html` (populate deploy `<section>`)

Use Tailwind Plus **Marketing > Bento Grids** or **Feature Sections** (icon grid).

- [ ] **Step 1: Build the 4-card deploy grid**

Structure:
- Section headline: "Your infrastructure. Your rules." — centered
- `grid grid-cols-1 sm:grid-cols-2 gap-6 max-w-4xl mx-auto`
- Each card: icon (inline SVG or emoji), headline, one-line description
- Card styling: `bg-neutral-50 dark:bg-neutral-950 border border-neutral-200 dark:border-neutral-800 rounded-xl p-6 hover:-translate-y-1 hover:border-amber-400/50 transition-all duration-200`

- [ ] **Step 2: Test**

Expected: 2x2 grid on desktop, single column on mobile. Hover lift effect works.

- [ ] **Step 3: Commit**

```bash
git add landing/index.html
git commit -m "feat(landing): deploy-your-way section with 4 deployment options"
```

---

### Task 9: Enterprise Ready Section

**Files:**
- Modify: `landing/index.html` (populate enterprise `<section>`)

Use Tailwind Plus **Marketing > Feature Sections** (icon grid).

- [ ] **Step 1: Build the 6-card enterprise grid**

Structure:
- Section headline: "Built for teams that don't compromise." — centered
- `grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6`
- Each card: icon (Heroicons — ShieldCheck, Key, ClipboardDocumentList, etc.), headline, description
- "Coming soon" features (Audit Logs, SCIM) get a small `text-xs text-amber-400` badge
- Same card styling as deploy section for consistency

- [ ] **Step 2: Test**

Expected: 3x2 grid on desktop, 2-col on tablet, single column on mobile.

- [ ] **Step 3: Commit**

```bash
git add landing/index.html
git commit -m "feat(landing): enterprise-ready section with 6 feature cards"
```

---

### Task 10: Pricing Section

**Files:**
- Modify: `landing/index.html` (populate pricing `<section>`)

Use Tailwind Plus **Marketing > Pricing Sections** (3-tier) as base.

- [ ] **Step 1: Build the 3-tier pricing table**

Structure:
- Section headline: "Simple pricing. Generous free tier." — centered
- `grid grid-cols-1 md:grid-cols-3 gap-8 max-w-5xl mx-auto`
- Each tier card:
  - Plan name + price (prominent)
  - Feature list with checkmarks (✓ in `text-amber-400`)
  - CTA button at bottom
- **Pro card** gets special treatment:
  - Amber "Recommended" badge at top
  - `border-amber-400 ring-1 ring-amber-400` border (stands out)
  - Slightly larger with `scale-105` or shadow
- **Free card**: ghost CTA → `https://app.ravencloak.org/register`
- **Pro card**: amber CTA → `https://app.ravencloak.org/register?plan=pro`
- **Enterprise card**: ghost CTA → `mailto:sales@ravencloak.org` or contact form

- [ ] **Step 2: Add the limit details**

Below each feature list, show the key limits: Users, Workspaces, Knowledge Bases, Storage, Voice Sessions, Voice Minutes — using `text-sm text-neutral-500` with the specific values.

- [ ] **Step 3: Test**

Expected: 3 cards side-by-side on desktop, Pro elevated. On mobile, stacked with Pro first (reorder via `order-first` on mobile).

- [ ] **Step 4: Commit**

```bash
git add landing/index.html
git commit -m "feat(landing): pricing section with free/pro/enterprise tiers"
```

---

### Task 11: Bottom CTA & Footer

**Files:**
- Modify: `landing/index.html` (populate CTA `<section>` and `<footer>`)

Use Tailwind Plus **Marketing > CTA Sections** and **Marketing > Footers**.

- [ ] **Step 1: Build the bottom CTA**

Structure:
- Dark section with slightly different background (`bg-neutral-950 dark:bg-neutral-950`)
- Headline: "Start building with Raven in under 5 minutes."
- Code block: `docker compose up -d` in a styled `<code>` block with `bg-neutral-900 text-amber-400 px-4 py-2 rounded-lg font-mono text-sm`
- Large amber CTA button: "Get Started Free" → `https://app.ravencloak.org/register`

- [ ] **Step 2: Build the footer**

4-column grid:
- Product, Resources, Community, Legal — each with links
- Bottom row: copyright + tagline
- `text-sm text-neutral-500` for links, `hover:text-amber-400` for hover
- Raven logo (small) in footer

- [ ] **Step 3: Test**

Expected: CTA section is visually distinct. Footer links work. Both themes correct.

- [ ] **Step 4: Commit**

```bash
git add landing/index.html
git commit -m "feat(landing): bottom CTA section and multi-column footer"
```

---

### Task 12: Scroll Animations & Polish

**Files:**
- Modify: `landing/index.html` (add animation JS and final CSS)
- Modify: `landing/src/input.css` (add animation keyframes if needed)

- [ ] **Step 1: Add scroll-triggered fade-in animations**

```javascript
const observer = new IntersectionObserver((entries) => {
  entries.forEach(entry => {
    if (entry.isIntersecting) {
      entry.target.classList.add('animate-fade-in-up')
      observer.unobserve(entry.target)
    }
  })
}, { threshold: 0.1 })

document.querySelectorAll('[data-animate]').forEach(el => {
  el.classList.add('opacity-0', 'translate-y-4')
  observer.observe(el)
})
```

Add `data-animate` attribute to each section's content container.

Add to `input.css`:
```css
@keyframes fade-in-up {
  from { opacity: 0; transform: translateY(1rem); }
  to { opacity: 1; transform: translateY(0); }
}

.animate-fade-in-up {
  animation: fade-in-up 0.6s ease-out forwards;
}

@media (prefers-reduced-motion: reduce) {
  .animate-fade-in-up { animation: none; opacity: 1; transform: none; }
  [data-animate] { opacity: 1 !important; transform: none !important; }
}
```

- [ ] **Step 2: Add focus ring styles for accessibility**

Ensure all interactive elements (buttons, links, tabs) have visible focus ring: `focus-visible:ring-2 focus-visible:ring-amber-400 focus-visible:ring-offset-2 focus-visible:ring-offset-black dark:focus-visible:ring-offset-black`

- [ ] **Step 3: Final responsive QA**

Test all breakpoints: `sm` (640px), `md` (768px), `lg` (1024px), `xl` (1280px). Fix any layout issues.

- [ ] **Step 4: Run production build**

```bash
cd landing && npm run build
```

Verify `dist/output.css` is under 50KB (purged Tailwind).

- [ ] **Step 5: Commit**

```bash
git add landing/
git commit -m "feat(landing): scroll animations, accessibility polish, production build"
```

---

### Task 13: Asset Placeholders & OG Image

**Files:**
- Create: `landing/assets/raven-logo-dark.svg` (placeholder)
- Create: `landing/assets/raven-logo-light.svg` (placeholder)
- Create: `landing/assets/og-image.png` (placeholder)
- Create: `landing/assets/screenshots/chat.png` (placeholder)
- Create: `landing/assets/screenshots/voice.png` (placeholder)
- Create: `landing/assets/screenshots/whatsapp.png` (placeholder)

- [ ] **Step 1: Create SVG logo placeholders**

Minimal text-based logo: "Raven" in Inter Bold with a simple bird icon outline. Dark version (white text on transparent), light version (black text on transparent).

- [ ] **Step 2: Create screenshot placeholders**

Styled gradient placeholder images (1280x720) with centered text describing what they represent. These will be replaced with real screenshots later.

- [ ] **Step 3: Create OG image placeholder**

1200x630 dark background with Raven logo and tagline "The AI Brain for Your Entire Team".

- [ ] **Step 4: Update index.html to reference assets**

Replace all text placeholder logos with `<img>` tags pointing to SVGs. Add themed logo switching:
```html
<img src="assets/raven-logo-light.svg" alt="Raven" class="h-8 dark:hidden">
<img src="assets/raven-logo-dark.svg" alt="Raven" class="h-8 hidden dark:block">
```

Update screenshot placeholders in the tabbed features section to use the actual image paths.

- [ ] **Step 5: Commit**

```bash
git add landing/assets/ landing/index.html
git commit -m "feat(landing): add placeholder assets for logo, screenshots, and OG image"
```

---

### Task 14: Integration Test & Lighthouse Audit

**Files:** None created — this is a verification task.

- [ ] **Step 1: Verify all internal links**

Check that all `#section-id` links scroll correctly. Check that all external links (`app.ravencloak.org`, GitHub, etc.) are correctly formed.

- [ ] **Step 2: Test theme toggle persistence**

Toggle theme → refresh page → theme should persist via `localStorage`.

- [ ] **Step 3: Test mobile menu**

Open hamburger → all links visible → close button works → clicking a link closes menu and scrolls.

- [ ] **Step 4: Run Lighthouse audit**

```bash
npx lighthouse http://localhost:8080 --output=json --output-path=./lighthouse-report.json
```

(Serve with any static server: `npx serve landing/`)

Target: Performance > 90, Accessibility > 90, Best Practices > 90, SEO > 90.

- [ ] **Step 5: Fix any Lighthouse issues and re-test**

Common fixes: add `alt` text, fix color contrast, add `aria-label`, optimize images.

- [ ] **Step 6: Final commit**

```bash
git add landing/
git commit -m "fix(landing): lighthouse audit fixes and final polish"
```
