# Raven Landing Page — Design Spec

**Date**: 2026-04-13
**Domain**: raven.ravencloak.org
**Status**: Approved

---

## Overview

A full-platform marketing landing page for Raven — an open-source, self-hostable, multi-tenant AI knowledge platform with chat, voice, and WhatsApp capabilities. The page follows a demo-led approach with an embedded live `<raven-chat>` widget in the hero section.

> **Note**: This is a **full redesign** replacing the existing `landing/index.html` (which uses an indigo/purple color scheme). The new page uses a black/white + amber palette and will be deployed at `raven.ravencloak.org`.

## Target Audience

- **Primary**: Developers and technical leads evaluating Raven for their team
- **Secondary**: Business decision makers (CTO/VP Eng) looking for enterprise features and compliance
- **Approach**: Developer-first copy with enterprise trust signals layered in

## Design System

### Color Palette (Tailwind)

| Token | Dark Theme | Light Theme | Usage |
|-------|-----------|-------------|-------|
| Background primary | `#000000` / `black` | `#ffffff` / `white` | Page background |
| Background secondary | `#0a0a0a` / `neutral-950` | `#fafafa` / `neutral-50` | Card/section backgrounds |
| Background tertiary | `#1a1a1a` / `neutral-900` | `#f5f5f5` / `neutral-100` | Inset panels, code blocks |
| Border | `#262626` / `neutral-800` | `#e5e5e5` / `neutral-200` | Card borders, dividers |
| Border hover | `#404040` / `neutral-700` | `#d4d4d4` / `neutral-300` | Interactive borders |
| Text primary | `#ffffff` / `white` | `#000000` / `black` | Headlines, body |
| Text secondary | `#a3a3a3` / `neutral-400` | `#525252` / `neutral-600` | Descriptions, subtext |
| Text muted | `#737373` / `neutral-500` | `#737373` / `neutral-500` | Captions, labels |
| Accent | `#fbbf24` / `amber-400` | `#f59e0b` / `amber-500` | CTAs, active indicators, highlights |
| Accent hover | `#f59e0b` / `amber-500` | `#d97706` / `amber-600` | Button hover states |

### Theme Behavior

- Dark/light toggle in nav (sun/moon icon)
- Default: respects `prefers-color-scheme` system preference
- Implemented via Tailwind `dark:` class variant on `<html>` element
- Raven logo: black bird on light theme, white bird on dark theme

### Typography

- **Font family**: Inter (loaded via Google Fonts, same as existing landing page)
- **Hero headline**: `text-5xl sm:text-6xl lg:text-7xl font-extrabold tracking-tight`
- **Section headlines**: `text-3xl sm:text-4xl font-bold`
- **Card headlines**: `text-xl font-semibold`
- **Body copy**: `text-base sm:text-lg` (16-18px), `leading-relaxed`
- **Muted/technical text**: `text-sm text-neutral-500`
- **Badge text**: `text-xs uppercase tracking-widest font-medium`

### Content Tone

- **Section headers**: Bold and opinionated — *"Your team's knowledge is trapped. Free it."*
- **Body copy**: Clear and accessible — *"Ingest any document, ask in natural language, get cited answers."*
- **Feature deep-dives**: Technical specifics — *"Hybrid search combining pgvector cosine similarity with BM25 full-text ranking"*
- **GitHub/README**: Engineering-precise tone (separate from landing page)

## Page Structure

### Section 1: Navigation Bar

**Layout**: Sticky top nav, full-width, blurred background

**Elements**:
- Left: Raven logo (themed — black on light, white on dark)
- Center: Features, Pricing, Docs, GitHub (with stars badge)
- Right: Theme toggle (sun/moon), Sign In (ghost button), Get Started Free (amber accent button)

**Mobile**: Hamburger menu, Get Started Free always visible

### Section 2: Hero (Centered, Demo-Led)

**Layout**: Centered text block → trust badges → live chat widget

**Content**:
- Badge: `Open Source · MIT Licensed` (small uppercase, amber text)
- Headline: **"The AI Brain for Your Entire Team"**
- Subhead: "Ingest your docs. Ask in natural language. Get cited answers via chat, voice, or WhatsApp. Self-host or use our cloud."
- CTAs: `Get Started Free` (amber filled) + `Try Live Demo ↓` (ghost/outline)
- Trust bar: 🔓 MIT Licensed · 🐳 4 Deploy Options · 🔑 Bring Your Own LLM Keys · 🛡️ Full Data Sovereignty

**Live Demo Widget**:
- Embedded `<raven-chat>` web component
- **Phase 1 (launch)**: Runs in mock mode with curated hardcoded Q&A pairs showcasing Raven's capabilities (the chat widget currently uses mock responses — real SSE endpoint is not yet implemented)
- **Phase 2 (post-launch)**: Connect to a live demo knowledge base once the real SSE chat API is implemented
- Header: "💬 Try Raven — Ask anything" + "Interactive Demo" badge
- Pre-populated with a sample Q&A showing the answer format (with citation styling)
- Input field with Send button (amber accent)
- "Try Live Demo ↓" CTA in hero smooth-scrolls to this widget

### Section 3: Problem Statement

**Headline**: "Your team's knowledge is scattered. Your people are stuck."

**Layout**: 3 cards in a row (stacks on mobile)

| Card | Headline | Copy |
|------|----------|------|
| 1 | Docs nobody reads | Knowledge lives in PDFs, wikis, and Confluence pages that go stale. Your team asks the same questions on Slack instead. |
| 2 | Search that doesn't work | Keyword search returns 50 results. None of them answer the actual question. Your team gives up and pings someone senior. |
| 3 | No way to just ask | Your team wants to ask a question and get an answer — with a source. Not browse. Not search. Just ask. |

**Transition line**: "Raven fixes this. Ingest your docs. Ask in natural language. Get cited answers — via chat, voice, or WhatsApp."

### Section 4: Tabbed Features

**Layout**: 3 tabs with active indicator (amber underline). Each tab shows screenshot on one side + description on the other. Alternating layout (screenshot left/right) for visual rhythm.

**Tab 1: AI Chat**
- Headline: "Ask your docs anything"
- Copy: Semantic search powered by hybrid vector + BM25 ranking. Every answer comes with source citations. Embed the chatbot widget on your site with a single `<raven-chat>` tag.
- Visual: Screenshot of chat interface with cited answer

**Tab 2: Voice Agent**
- Headline: "Talk to your knowledge base"
- Copy: Real-time voice calls via WebRTC. Hands-free access to your entire knowledge base — powered by LiveKit with STT, LLM, and TTS pipeline. Perfect for field teams and mobile workers.
- Visual: Screenshot of voice call UI

**Tab 3: WhatsApp**
- Headline: "Meet your team where they already are"
- Copy: Connect via WhatsApp Business API. Zero app installs, zero training. Your team asks questions in the chat they already use every day.
- Visual: Screenshot of WhatsApp conversation with Raven

### Section 5: How It Works

**Layout**: 3 numbered cards in horizontal row, connected by a line/arrow

| Step | Headline | Copy |
|------|----------|------|
| 1 | **Ingest** | Upload PDFs, DOCX, Markdown, HTML, or paste URLs. Raven chunks, embeds, and indexes everything automatically. |
| 2 | **Search** | Hybrid retrieval — pgvector cosine similarity + BM25 full-text ranking fused with Reciprocal Rank Fusion. No keyword guessing. |
| 3 | **Answer** | Your team asks in natural language. Raven responds with accurate, cited answers — via chat, voice, or WhatsApp. |

Technical details in muted text below each card for developer audience.

### Section 6: Deploy Your Way

**Headline**: "Your infrastructure. Your rules."

**Layout**: 4 cards in a grid (2x2 on desktop, stacks on mobile)

| Option | Icon | Copy |
|--------|------|------|
| Docker Compose | 🐳 | One command. Full stack. Perfect for getting started and small teams. |
| Kubernetes | ☸️ | Production-grade, multi-region, auto-scaling. |
| EC2 + Ansible | 🖥️ | Cloud VM with auto-TLS via Traefik. Playbooks ready to go. |
| Edge / Raspberry Pi | 📡 | 25MB ARM64 binary. Run the API on a Pi, connect a remote AI worker via gRPC. |

### Section 7: Enterprise Ready

**Headline**: "Built for teams that don't compromise."

**Layout**: 6 cards in a 3x2 grid (stacks on mobile)

| Card | Copy |
|------|------|
| Multi-Tenant Isolation | Full Row-Level Security per tenant in PostgreSQL. Your data never crosses boundaries. |
| SSO / SCIM | Keycloak-powered OIDC and SAML out of the box. SCIM provisioning coming soon. |
| Audit Logs | Timestamped audit trail for every action. *(Coming soon — in roadmap)* |
| BYOK | Bring your own LLM keys — Anthropic, OpenAI, Cohere, or self-hosted models. Zero vendor lock-in. |
| eBPF WAF | Kernel-level XDP pre-filtering and observability at the edge. |
| GDPR / SOC2 | Built for compliance. Self-host for full data sovereignty. |

### Section 8: Pricing

**Headline**: "Simple pricing. Generous free tier."

**Layout**: 3 tier cards side-by-side. Pro card has amber "Recommended" badge and slightly elevated/bordered treatment.

| | Free | Pro (Recommended) | Enterprise |
|---|---|---|---|
| Price | $0 forever | $29/month | Contact Sales *(backend has $99/mo placeholder — landing page shows "Contact Sales" intentionally)* |
| Users | 5 | 25 | Unlimited |
| Workspaces | 2 | 10 | Unlimited |
| Knowledge Bases | 3 | 50 | Unlimited |
| Storage | 500 MB | 10 GB | Unlimited |
| Voice Sessions | 1 concurrent | 5 concurrent | Unlimited |
| Voice Minutes | 60/mo | 1,200/mo | Unlimited |
| Key Features | Full MIT source, AI chat/voice/WhatsApp, BYOK, self-host | Everything in Free + higher limits, advanced rate limiting | Everything in Pro + SAML/SSO/SCIM, audit logs, eBPF WAF, dedicated support, custom SLA |
| CTA | Get Started Free | Start Pro Trial | Contact Sales |

### Section 9: Bottom CTA

**Headline**: "Start building with Raven in under 5 minutes."

**Code snippet**: `docker compose up -d`

**CTA**: `Get Started Free` (amber accent, large)

### Section 10: Footer

**Layout**: 4-column grid (stacks on mobile)

| Product | Resources | Community | Legal |
|---------|-----------|-----------|-------|
| Features | Quick Start | GitHub | Privacy Policy |
| Pricing | API Reference | Discord | Terms of Service |
| Docs | Architecture | Contributing | MIT License |
| Changelog | Blog | Roadmap | |

**Bottom row**: © 2026 Ravencloak · Built with ❤️ for teams that own their data

## Technical Implementation

### Stack
- **Framework**: Static HTML + Tailwind CSS v4 (paid license — Tailwind Plus/UI components)
- **Build**: Tailwind CLI with purging for production (NOT the CDN play script — must meet <500KB target)
- **Theme**: Tailwind `dark:` class variant, toggled via JavaScript on `<html>` element, default from `prefers-color-scheme`
- **Chat Widget**: Embedded `<raven-chat>` web component (mock mode at launch, live API post-launch)
- **Hosting**: Static site at raven.ravencloak.org (Cloudflare Pages)
- **Responsive**: Mobile-first, breakpoints at `sm`, `md`, `lg`, `xl`
- **Font loading**: Inter via Google Fonts with `font-display: swap`

### SEO & Meta Tags

```html
<title>Raven — The AI Brain for Your Entire Team</title>
<meta name="description" content="Open-source AI knowledge platform. Ingest your docs, ask in natural language, get cited answers via chat, voice, or WhatsApp. Self-host or use our cloud.">
<meta property="og:title" content="Raven — Open Source AI Knowledge Platform">
<meta property="og:description" content="Ingest your docs. Ask in natural language. Get cited answers via chat, voice, or WhatsApp.">
<meta property="og:url" content="https://raven.ravencloak.org">
<meta property="og:image" content="https://raven.ravencloak.org/og-image.png">
<meta property="og:type" content="website">
<meta name="twitter:card" content="summary_large_image">
<link rel="canonical" href="https://raven.ravencloak.org">
```

OG image: 1200x630px, Raven logo + tagline on dark background. Created as a static asset.

### Domain Map

| Purpose | Domain |
|---------|--------|
| Marketing/landing page | `raven.ravencloak.org` |
| Application (dashboard) | `app.ravencloak.org` |
| API server | `api.ravencloak.org` |
| Auth (Keycloak) | `auth.ravencloak.org` |
| Documentation | `docs.ravencloak.org` (future) |

### Accessibility

- Amber accent (`#fbbf24`) on black background: contrast ratio ~11.6:1 (passes WCAG AA and AAA)
- Muted text (`#737373`) on black: contrast ratio ~4.6:1 (passes AA for normal text)
- All interactive elements: visible focus ring (`ring-2 ring-amber-400 ring-offset-2 ring-offset-black`)
- `prefers-reduced-motion`: disable scroll animations
- Semantic HTML: `<nav>`, `<main>`, `<section>`, `<footer>` with `aria-label` on each section

### Interactions & Animations

- **Scroll animations**: Subtle fade-in-up on section entry (via `IntersectionObserver`), respects `prefers-reduced-motion`
- **Tab switching** (Section 4): Click to switch, amber underline slides to active tab, screenshot crossfades. On mobile: tabs become a vertical accordion
- **Hover states**: Cards lift with `hover:-translate-y-1 hover:border-amber-400/50` transition
- **Theme toggle**: Smooth color transition via `transition-colors duration-300` on `<html>`
- **GitHub stars**: Static count at build time (hardcoded, updated periodically) — avoids GitHub API rate limits

### Assets Required
- Raven logo (SVG, themed — black and white variants)
- Product screenshots for Chat, Voice, WhatsApp features (from existing admin dashboard)
- Icons for deployment options and enterprise features (Tailwind Heroicons or similar)

### Demo Knowledge Base
- A curated knowledge base containing Raven's own documentation
- Hosted on the production API or a dedicated demo instance
- The `<raven-chat>` widget connects via a domain-scoped API key
- Rate-limited for public access (anonymous users, read-only)

### Performance Targets
- First Contentful Paint < 1.5s
- Total page weight < 500KB (excluding chat widget)
- Lighthouse score > 90 across all categories

## Prerequisites (must resolve before implementation)

1. **Domain setup**: `raven.ravencloak.org` DNS must be configured and pointing to Cloudflare Pages (or chosen hosting)
2. **Raven logo**: Finalized SVG logo in black and white variants — if none exists, create a minimal text/icon logo as placeholder

## Open Questions (non-blocking, can resolve during implementation)

1. **Product screenshots**: Can we capture from the existing admin dashboard, or do we mock them? (Fallback: styled placeholder illustrations)
2. **OG image**: Design and generate the 1200x630 social sharing image
3. **GitHub stars count**: Current count to hardcode at build time

## Out of Scope

- User authentication / sign-up flow (handled by app.ravencloak.org)
- Blog / changelog pages (future addition)
- Documentation site (separate docs.ravencloak.org)
- A/B testing / analytics integration (PostHog — future addition)
