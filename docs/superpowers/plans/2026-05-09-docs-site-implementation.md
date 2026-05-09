# Raven Docs Site Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Tasks within the same wave are independent and parallelisable; waves run sequentially. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `docs.raven.ravencloak.org` — a VitePress documentation site fed by `docs/` markdown and `contracts/openapi.yaml`, deployed to Cloudflare Pages.

**Architecture:** New top-level `docs-site/` holds the SSG (VitePress + vitepress-openapi + PageFind). A build-time sync step copies `docs/` markdown into a gitignored `.tmp/` tree per `content-map.json`. Static output ships to Cloudflare Pages via `wrangler@4` from a GitHub Actions workflow on every PR (preview) and `main` push (production). API-reference pages are generated from `contracts/openapi.yaml`; that file is also hosted as a downloadable artefact at `/api/openapi.yaml` and `/api/openapi.json`.

**Tech Stack:** VitePress 1.x, vitepress-openapi, PageFind, Spectral, wrangler@4, Cloudflare Pages, Node 22, GitHub Actions. Black-on-white theme with a single red accent (`rose-600` family).

**Worktree:** `.worktrees/docs-site` on branch `docs/spec-docs-site-design`. The spec for this plan is at `docs/superpowers/specs/2026-05-09-docs-site-design.md` (commit `811d28d6`).

---

## File structure

Files created / modified, grouped by owning task. Each task owns its files exclusively — agents working in the same wave do not write the same file.

| Task | Files |
| ---- | ----- |
| **T1** Spec ingestion | `contracts/openapi.yaml` (renamed from `openapi-stub.yaml`) · `contracts/.spectral.yaml` (new) |
| **T2** VitePress scaffolding | `docs-site/package.json` (new) · `docs-site/package-lock.json` (generated) · `docs-site/.gitignore` (new) · `docs-site/.vitepress/config.ts` (new — base config; sidebar additions in T5/T6) · `docs-site/.vitepress/theme/index.ts` (new) · `docs-site/.vitepress/theme/style.css` (new) · `docs-site/index.md` (new — homepage with 3-card hero) |
| **T3** README docs section | `README.md` (modify — append "Documentation" section) |
| **T4** Content sync | `docs-site/content-map.json` (new) · `docs-site/scripts/sync-content.ts` (new) · `docs-site/package.json` (modify — add `prebuild` and `dev` scripts that run sync) |
| **T5** Stub IA pages + sidebar | `docs-site/.vitepress/sidebars/main.ts` (new — exported sidebar config) · `docs-site/.vitepress/config.ts` (modify — import + register sidebar) · `docs-site/stubs/*.md` (new — one stub per IA leaf path; copied into place by sync) · `docs-site/content-map.json` (modify — register stub mappings) |
| **T6** API reference + spec hosting + Beta banner | `docs-site/.vitepress/openapi.ts` (new — vitepress-openapi config) · `docs-site/.vitepress/config.ts` (modify — register API sidebar) · `docs-site/.vitepress/components/BetaBanner.vue` (new) · `docs-site/.vitepress/theme/index.ts` (modify — register BetaBanner) · `docs-site/scripts/copy-spec.ts` (new — copies openapi.yaml + emits openapi.json into dist/api/) · `docs-site/package.json` (modify — add vitepress-openapi dep + `postbuild:spec` script) · `docs-site/api/overview.md` (new — landing page with download buttons) |
| **T7** 404 + robots + sitemap + footer | `docs-site/404.md` (new — custom 404 with PageFind) · `docs-site/public/robots.txt` (new) · `docs-site/.vitepress/config.ts` (modify — sitemap config + footer text) |
| **T8** CI + Cloudflare deploy | `.github/workflows/docs.yml` (new) |

---

## Execution waves

```
Wave 1  (independent)            Wave 2  (depends on Wave 1)        Wave 3  (depends on Wave 2)
────────────────────             ──────────────────────────         ────────────────────────────
[T1] Spec ingestion              [T4] Content sync                  [T8] CI + Cloudflare deploy
[T2] VitePress scaffolding       [T5] Stub IA pages + sidebar
[T3] README docs section         [T6] API reference + spec hosting
                                 [T7] 404 + robots + sitemap + footer
```

T2 is the gating task for Wave 2 — without `package.json` and `.vitepress/config.ts`, nothing else can build. T1 is also a Wave-1 prerequisite for T6 (API reference reads `contracts/openapi.yaml`).

T5, T6, T7 all touch `docs-site/.vitepress/config.ts`. To avoid merge conflicts, T5 and T6 each export their sidebar contributions from a separate file (`sidebars/main.ts`, `openapi.ts`); T2 sets up the config skeleton with explicit import slots so T5/T6 only edit narrow lines. T7 only adds sitemap config and footer text in a separate top-level block.

---

## Task 1: Spec ingestion

**Owner:** Wave 1, parallel.

**Goal:** Move `contracts/openapi-stub.yaml` to its canonical name and add a Spectral lint ruleset that runs in CI before docs build.

**Files:**
- Rename: `contracts/openapi-stub.yaml` → `contracts/openapi.yaml`
- Create: `contracts/.spectral.yaml`

### Steps

- [ ] **Step 1: Rename the stub spec to canonical name.**

```bash
git mv contracts/openapi-stub.yaml contracts/openapi.yaml
```

- [ ] **Step 2: Create `contracts/.spectral.yaml`** with project-relevant rules.

```yaml
# Spectral ruleset for Raven's OpenAPI contract.
# Extends the standard OAS rules with a few project conventions.
extends:
  - "spectral:oas"

rules:
  # Every operation must declare an operationId — needed by oapi-codegen
  # and used as the URL slug by vitepress-openapi.
  operation-operationId: error

  # Every operation must declare at least one tag — drives the docs sidebar
  # grouping in vitepress-openapi.
  operation-tag-defined: error

  # Top-level info must include license + contact for OSS hygiene.
  info-license: warn
  info-contact: warn

  # Servers block is mandatory; the docs render the production URL.
  oas3-server-not-example.com: error

  # Operations must declare a summary so the sidebar entry is human-readable.
  operation-summary: error
```

- [ ] **Step 3: Run Spectral locally to verify the spec passes.**

Run from the worktree root:

```bash
npx -y @stoplight/spectral-cli@6 lint contracts/openapi.yaml --ruleset contracts/.spectral.yaml --fail-severity=error
```

Expected: exit code 0. Warnings on `info-license` / `info-contact` are acceptable in Phase 1; errors are not. If the stub fails on `operation-operationId` or `operation-summary`, fix the spec to add the missing fields rather than relax the rule.

- [ ] **Step 4: Commit.**

```bash
git add contracts/.spectral.yaml
git add -u contracts/  # picks up the rename
git commit -s -m "feat(spec): rename openapi-stub.yaml to openapi.yaml + add Spectral ruleset

Establishes contracts/openapi.yaml as the canonical source-of-truth path
for both the docs site and the eventual oapi-codegen migration. Adds a
project Spectral ruleset that the docs CI workflow gates on
(operationId, tags, summary required; license/contact warned)."
```

---

## Task 2: VitePress scaffolding

**Owner:** Wave 1, parallel.

**Goal:** Bootstrap the `docs-site/` directory with VitePress, the brand theme, a homepage with the three-card hero, and the gitignore. The site builds cleanly with `npm run build` even before content sync runs.

**Files:**
- Create: `docs-site/package.json`
- Create: `docs-site/package-lock.json` (generated by `npm install`)
- Create: `docs-site/.gitignore`
- Create: `docs-site/.vitepress/config.ts`
- Create: `docs-site/.vitepress/theme/index.ts`
- Create: `docs-site/.vitepress/theme/style.css`
- Create: `docs-site/index.md`

### Steps

- [ ] **Step 1: Create `docs-site/package.json`.**

```json
{
  "name": "raven-docs",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vitepress dev",
    "build": "vitepress build && pagefind --site .vitepress/dist",
    "preview": "vitepress preview"
  },
  "devDependencies": {
    "vitepress": "^1.5.0",
    "vue": "^3.5.0",
    "pagefind": "^1.1.0",
    "tsx": "^4.19.0",
    "typescript": "^5.6.0"
  }
}
```

- [ ] **Step 2: Install dependencies and generate the lockfile.**

```bash
cd docs-site
npm install
```

Expected: `package-lock.json` is created; `node_modules/` populated (gitignored next step).

- [ ] **Step 3: Create `docs-site/.gitignore`.**

```
node_modules/
.vitepress/dist/
.vitepress/cache/
.tmp/
wrangler.out
```

- [ ] **Step 4: Create the brand stylesheet at `docs-site/.vitepress/theme/style.css`.**

```css
/**
 * Raven docs brand tokens — black on white, white on near-black, with a
 * single red accent. The chosen red passes WCAG AA against both the light
 * and dark VitePress backgrounds.
 */

:root {
  /* Brand red (rose-600 family). Verified WCAG AA on light + dark. */
  --vp-c-brand-1: #e11d48;
  --vp-c-brand-2: #be123c;
  --vp-c-brand-3: #9f1239;
  --vp-c-brand-soft: rgba(225, 29, 72, 0.14);

  /* Neutralise the default purple-leaning hover/active accents so the
     red is the only chromatic colour on the page. */
  --vp-c-accent: var(--vp-c-brand-1);
  --vp-c-accent-hover: var(--vp-c-brand-2);
  --vp-c-accent-active: var(--vp-c-brand-3);
}

.dark {
  --vp-c-brand-1: #fb7185;
  --vp-c-brand-2: #f43f5e;
  --vp-c-brand-3: #e11d48;
  --vp-c-brand-soft: rgba(251, 113, 133, 0.16);
}

/* Tighten the homepage hero so the three feature cards line up with the
   nav above. */
.VPHome .VPFeatures {
  padding-top: 0;
}
```

- [ ] **Step 5: Create `docs-site/.vitepress/theme/index.ts`.**

```ts
import DefaultTheme from "vitepress/theme";
import "./style.css";

// Theme exports. T6 will register the BetaBanner component in this file
// when it lands; until then we just extend the default theme with brand
// tokens defined in style.css.
export default {
  extends: DefaultTheme,
};
```

- [ ] **Step 6: Create `docs-site/.vitepress/config.ts`.**

```ts
import { defineConfig } from "vitepress";

export default defineConfig({
  title: "Raven Docs",
  description:
    "Open-source multi-tenant knowledge base platform with AI chat, voice, and WhatsApp.",
  lang: "en-US",
  cleanUrls: true,
  lastUpdated: true,

  // Read content from .tmp/, populated by scripts/sync-content.ts at
  // build time. T2 ships this empty; T4 wires the sync.
  srcDir: ".tmp",

  head: [
    ["link", { rel: "icon", href: "/favicon.svg", type: "image/svg+xml" }],
    ["meta", { name: "theme-color", content: "#e11d48" }],
    ["meta", { property: "og:type", content: "website" }],
    ["meta", { property: "og:site_name", content: "Raven Docs" }],
  ],

  themeConfig: {
    nav: [
      { text: "Get Started", link: "/get-started/installation" },
      { text: "Guides", link: "/guides/workspaces-and-tenancy" },
      { text: "Concepts", link: "/concepts/architecture" },
      { text: "API", link: "/api/overview" },
      { text: "Contributing", link: "/contributing/overview" },
    ],

    // Sidebar contributions are merged in by T5 (main IA) and T6 (API).
    sidebar: {},

    socialLinks: [
      { icon: "github", link: "https://github.com/ravencloak-org/Raven" },
    ],

    editLink: {
      // Edit-on-GitHub points at the SOURCE markdown in docs/ or the
      // repo root, not the synced .tmp/ copy. T4 wires the path
      // rewrite when sync-content runs.
      pattern:
        "https://github.com/ravencloak-org/Raven/edit/main/docs/:path",
      text: "Edit this page on GitHub",
    },

    footer: {
      // T7 fills the message + copyright text.
      message: "",
      copyright: "",
    },

    search: {
      provider: "local",
    },
  },
});
```

- [ ] **Step 7: Create the homepage at `docs-site/index.md`.**

```md
---
layout: home

hero:
  name: "Raven"
  text: "Self-hostable knowledge platform with AI chat, voice, and WhatsApp."
  tagline: "Multi-tenant. BYOK. Edge-deployable. Open source."
  actions:
    - theme: brand
      text: Get Started
      link: /get-started/installation
    - theme: alt
      text: View on GitHub
      link: https://github.com/ravencloak-org/Raven

features:
  - icon: 🚀
    title: Get Started
    details: Install with one Docker Compose command, ingest your first knowledge base, embed the chat widget.
    link: /get-started/installation
    linkText: Quickstart
  - icon: 🛠️
    title: Self-Host
    details: Compose, edge / Raspberry Pi, Traefik + TLS, observability, backups, hardening.
    link: /guides/self-hosting/docker-compose
    linkText: Operator guide
  - icon: 🔌
    title: API
    details: Spec-first OpenAPI 3.1, one URL per operation, downloadable spec for your tooling.
    link: /api/overview
    linkText: API reference
---
```

> **Note for the agent:** `index.md` lives at `docs-site/index.md` (so it ships in the repo). The build's sync step will copy or pass it through to `.tmp/index.md`. T4 owns that wiring; for T2 we just place the file at the source location.

- [ ] **Step 8: Build the site and verify it succeeds.**

```bash
cd docs-site
mkdir -p .tmp && cp index.md .tmp/index.md
npm run build
```

Expected: `vitepress build` produces `.vitepress/dist/` with at least `index.html` and `404.html`. PageFind logs an empty index (acceptable — content lands in T4/T5/T6).

```bash
ls .vitepress/dist/
```

Expected output includes: `index.html`, `_pagefind/`, `assets/`.

Clean up the smoke artefact:

```bash
rm -rf .tmp
```

- [ ] **Step 9: Commit.**

```bash
git add docs-site/package.json docs-site/package-lock.json docs-site/.gitignore docs-site/.vitepress/config.ts docs-site/.vitepress/theme/index.ts docs-site/.vitepress/theme/style.css docs-site/index.md
git commit -s -m "feat(docs-site): bootstrap VitePress with brand theme and homepage

Adds the docs-site/ directory: package.json with VitePress + PageFind,
.vitepress/config.ts skeleton (sidebar slots reserved for later tasks),
black-on-white theme with a red accent, and a homepage with three
hero cards (Get Started / Self-Host / API). Site builds cleanly even
without synced content."
```

---

## Task 3: README docs section

**Owner:** Wave 1, parallel.

**Goal:** Add a "Documentation" section to the repo README that points at `docs.raven.ravencloak.org`. Independent of the docs site itself — a discoverability change.

**Files:**
- Modify: `README.md`

### Steps

- [ ] **Step 1: Locate the existing "Quick Start" section in `README.md`.**

```bash
grep -n "## Quick Start" README.md
```

Expected: a single match around line 67–69.

- [ ] **Step 2: Insert a new "Documentation" section immediately after "Quick Start" and before "Testing".**

Use Edit to insert after the closing of the Quick Start section. The new section reads:

```md
## Documentation

Full documentation is at **[docs.raven.ravencloak.org](https://docs.raven.ravencloak.org)** —
quickstart, self-hosting guide, concepts, API reference, contributing.

Source markdown lives in [`docs/`](./docs/); the public site is built from it
via VitePress in [`docs-site/`](./docs-site/) and deployed to Cloudflare Pages.
Open a PR against `main` and the docs CI will post a preview URL on the PR
within ~2 minutes.

```

The exact Edit:

```
old_string:
## Quick Start

new_string:
## Documentation

Full documentation is at **[docs.raven.ravencloak.org](https://docs.raven.ravencloak.org)** —
quickstart, self-hosting guide, concepts, API reference, contributing.

Source markdown lives in [`docs/`](./docs/); the public site is built from it
via VitePress in [`docs-site/`](./docs-site/) and deployed to Cloudflare Pages.
Open a PR against `main` and the docs CI will post a preview URL on the PR
within ~2 minutes.

## Quick Start
```

- [ ] **Step 3: Verify the markdown still renders correctly.**

```bash
grep -n "^## " README.md
```

Expected: section order is `What is Raven?`, `Key Features`, `Architecture Overview`, `Tech Stack`, `Quick Start`, `Documentation`, `Testing`, `Roadmap`, `Contributing`, `Security`, `Licensing` — with `Documentation` between `Quick Start` and `Testing`.

If the Edit landed before `Quick Start` instead of after, swap with another Edit:

```
old_string:
## Quick Start

new_string:
## Quick Start
```

(no-op) — and instead insert the section header after the last paragraph of Quick Start. The end of Quick Start is the line `For local development without Docker, see [DEVELOPMENT.md](DEVELOPMENT.md).` Insert immediately after it.

- [ ] **Step 4: Commit.**

```bash
git add README.md
git commit -s -m "docs(readme): link to docs.raven.ravencloak.org

Adds a Documentation section between Quick Start and Testing pointing
at the public docs site. Notes that source markdown lives in docs/
and the site is built via VitePress in docs-site/."
```

---

## Task 4: Content sync

**Owner:** Wave 2, parallel within wave (no overlap with T5/T6/T7 files).

**Goal:** Wire the build step that copies `docs/` markdown into `docs-site/.tmp/` per `content-map.json`. The script also rewrites cross-document links so internal links between `docs/` files resolve correctly in the rendered site.

**Files:**
- Create: `docs-site/content-map.json`
- Create: `docs-site/scripts/sync-content.ts`
- Modify: `docs-site/package.json` (add `prebuild`, `predev`, and `presync` script wires)

### Steps

- [ ] **Step 1: Create `docs-site/content-map.json` with the source-to-site mapping from the spec.**

```json
{
  "$schema": "./content-map.schema.json",
  "mappings": [
    { "from": "docs-site/index.md", "to": "index.md" },

    { "from": "docs/quickstart.md", "to": "get-started/installation.md" },

    { "from": "docs/architecture.md", "to": "concepts/architecture.md" },
    { "from": "docs/wiki/Architecture-Overview.md", "to": "concepts/system-overview.md" },
    { "from": "docs/wiki/Data-Model.md", "to": "concepts/data-model.md" },

    { "from": "docs/dependency-policy.md", "to": "contributing/dependency-policy.md" },
    { "from": "CONTRIBUTING.md", "to": "contributing/overview.md" },

    { "from": "SECURITY.md", "to": "reference/security-policy.md" },
    { "from": "CODE_OF_CONDUCT.md", "to": "community/code-of-conduct.md" },
    { "from": "GOVERNANCE.md", "to": "community/governance.md" },
    { "from": "MAINTAINERS.md", "to": "community/maintainers.md" },

    { "from": "docs/security/slsa-verification.md", "to": "trust/slsa-level-3.md" },
    { "from": "docs/compliance/osps-l2-2026-02-19.md", "to": "trust/openssf-baseline.md" },
    { "from": "docs/compliance/openssf-best-practices-12590.md", "to": "trust/openssf-best-practices.md" }
  ],
  "linkRewrites": [
    { "match": "^docs/quickstart\\.md$", "replace": "/get-started/installation/" },
    { "match": "^docs/architecture\\.md$", "replace": "/concepts/architecture/" },
    { "match": "^docs/wiki/Architecture-Overview\\.md$", "replace": "/concepts/system-overview/" },
    { "match": "^docs/wiki/Data-Model\\.md$", "replace": "/concepts/data-model/" },
    { "match": "^docs/security/slsa-verification\\.md$", "replace": "/trust/slsa-level-3/" },
    { "match": "^docs/compliance/osps-l2-2026-02-19\\.md$", "replace": "/trust/openssf-baseline/" },
    { "match": "^docs/compliance/openssf-best-practices-12590\\.md$", "replace": "/trust/openssf-best-practices/" }
  ],
  "stubMappings": "_filled-by-T5_"
}
```

> **Note:** the `stubMappings` placeholder is intentional — T5 owns it and edits the JSON to add stub-page mappings. It must already exist as a key so T5's edit is a single string replace rather than a JSON-merge.

- [ ] **Step 2: Create `docs-site/scripts/sync-content.ts`.**

```ts
import { promises as fs } from "node:fs";
import path from "node:path";
import process from "node:process";

interface Mapping { from: string; to: string }
interface LinkRewrite { match: string; replace: string }

interface ContentMap {
  mappings: Mapping[];
  linkRewrites: LinkRewrite[];
}

const REPO_ROOT = path.resolve(import.meta.dirname, "..", "..");
const DOCS_SITE = path.resolve(import.meta.dirname, "..");
const TMP_DIR = path.join(DOCS_SITE, ".tmp");

async function loadMap(): Promise<ContentMap> {
  const raw = await fs.readFile(
    path.join(DOCS_SITE, "content-map.json"),
    "utf8",
  );
  return JSON.parse(raw);
}

function rewriteLinks(content: string, rewrites: LinkRewrite[]): string {
  // Match Markdown links of the form [text](path/to/file.md) and rewrite
  // their path component when it matches one of the configured patterns.
  return content.replace(/\]\(([^)]+)\)/g, (whole, target: string) => {
    // External or anchor-only links pass through unchanged.
    if (/^(https?:|#|mailto:)/.test(target)) return whole;

    // Strip optional anchor for matching, preserve it for output.
    const [pathPart, anchor = ""] = target.split("#");
    for (const rule of rewrites) {
      const re = new RegExp(rule.match);
      if (re.test(pathPart)) {
        const newPath = pathPart.replace(re, rule.replace);
        return `](${newPath}${anchor ? `#${anchor}` : ""})`;
      }
    }
    return whole;
  });
}

async function copyFile(from: string, to: string, rewrites: LinkRewrite[]) {
  const src = path.join(REPO_ROOT, from);
  const dest = path.join(TMP_DIR, to);
  await fs.mkdir(path.dirname(dest), { recursive: true });

  let content: string;
  try {
    content = await fs.readFile(src, "utf8");
  } catch (err: unknown) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      console.warn(`sync-content: missing source ${from} (skipping)`);
      return;
    }
    throw err;
  }
  await fs.writeFile(dest, rewriteLinks(content, rewrites));
}

async function main() {
  const map = await loadMap();
  await fs.rm(TMP_DIR, { recursive: true, force: true });
  await fs.mkdir(TMP_DIR, { recursive: true });

  for (const m of map.mappings) {
    await copyFile(m.from, m.to, map.linkRewrites);
  }

  console.log(
    `sync-content: copied ${map.mappings.length} files to ${path.relative(REPO_ROOT, TMP_DIR)}/`,
  );
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
```

- [ ] **Step 3: Modify `docs-site/package.json` to wire the sync into npm scripts.**

Edit the `scripts` section to:

```json
"scripts": {
  "sync": "tsx scripts/sync-content.ts",
  "predev": "npm run sync",
  "dev": "vitepress dev",
  "prebuild": "npm run sync",
  "build": "vitepress build && pagefind --site .vitepress/dist",
  "preview": "vitepress preview"
}
```

- [ ] **Step 4: Run the sync and verify .tmp/ is populated correctly.**

```bash
cd docs-site
npm run sync
ls -la .tmp/
ls -la .tmp/concepts/ .tmp/trust/ .tmp/contributing/
```

Expected: `index.md` at the root of `.tmp/`, plus the directory tree for each mapping. If a source file is missing (e.g. a not-yet-written one), the script logs a warning and skips it — that's fine.

- [ ] **Step 5: Build to confirm VitePress reads the synced content.**

```bash
npm run build
```

Expected: `vitepress build` succeeds; `.vitepress/dist/` now contains pages for each successfully-synced source.

- [ ] **Step 6: Commit.**

```bash
git add docs-site/content-map.json docs-site/scripts/sync-content.ts docs-site/package.json docs-site/package-lock.json
git commit -s -m "feat(docs-site): build-time sync from docs/ into .tmp/

Adds scripts/sync-content.ts that reads content-map.json and copies
each mapped markdown source into .tmp/, rewriting internal cross-doc
links per linkRewrites. Wires npm prebuild/predev to run the sync so
the docs/ directory remains the editable source of truth while
VitePress reads from the gitignored .tmp/ tree."
```

---

## Task 5: Stub IA pages + sidebar

**Owner:** Wave 2, parallel within wave.

**Goal:** Every IA leaf path in the spec resolves to *some* page so navigation never breaks. Pages whose source content does not yet exist render a clearly-marked stub. Sidebar groups all of them per the IA tree.

**Files:**
- Create: `docs-site/.vitepress/sidebars/main.ts`
- Modify: `docs-site/.vitepress/config.ts` (import + register `mainSidebar`)
- Create: `docs-site/stubs/*.md` for every IA leaf without a mapped source
- Modify: `docs-site/content-map.json` (`stubMappings` key)

### Steps

- [ ] **Step 1: Create the sidebar definition at `docs-site/.vitepress/sidebars/main.ts`.**

This is the single source of truth for navigation; pages reference it by URL, so the same paths must exist on disk.

```ts
import type { DefaultTheme } from "vitepress";

export const mainSidebar: DefaultTheme.SidebarMulti = {
  "/get-started/": [
    {
      text: "Get Started",
      items: [
        { text: "Installation", link: "/get-started/installation" },
        { text: "First Knowledge Base", link: "/get-started/first-knowledge-base" },
        { text: "Embed the Chat Widget", link: "/get-started/embed-the-chat-widget" },
        { text: "Try the Voice Agent", link: "/get-started/try-the-voice-agent" },
      ],
    },
  ],

  "/guides/": [
    {
      text: "Operating Raven",
      items: [
        { text: "Workspaces & Tenancy", link: "/guides/workspaces-and-tenancy" },
        { text: "Ingestion", link: "/guides/ingestion" },
        { text: "Retrieval", link: "/guides/retrieval" },
        { text: "LLM Providers", link: "/guides/llm-providers" },
        { text: "Voice", link: "/guides/voice" },
        { text: "Webhooks & Events", link: "/guides/webhooks-and-events" },
        { text: "Billing", link: "/guides/billing" },
      ],
    },
    {
      text: "Self-Hosting",
      items: [
        { text: "Docker Compose", link: "/guides/self-hosting/docker-compose" },
        { text: "Edge & Raspberry Pi", link: "/guides/self-hosting/edge-and-raspberry-pi" },
        { text: "Traefik & TLS", link: "/guides/self-hosting/traefik-and-tls" },
        { text: "Observability", link: "/guides/self-hosting/observability" },
        { text: "Backups", link: "/guides/self-hosting/backups" },
        { text: "Upgrades", link: "/guides/self-hosting/upgrades" },
        { text: "Hardening", link: "/guides/self-hosting/hardening" },
      ],
    },
  ],

  "/concepts/": [
    {
      text: "Concepts",
      items: [
        { text: "Architecture", link: "/concepts/architecture" },
        { text: "System Overview", link: "/concepts/system-overview" },
        { text: "Data Model", link: "/concepts/data-model" },
        { text: "Multi-Tenancy", link: "/concepts/multi-tenancy" },
        { text: "Hybrid Retrieval", link: "/concepts/hybrid-retrieval" },
        { text: "Deployment Models", link: "/concepts/deployment-models" },
        { text: "AI Worker Design", link: "/concepts/ai-worker-design" },
      ],
    },
  ],

  "/reference/": [
    {
      text: "Reference",
      items: [
        { text: "Configuration", link: "/reference/configuration" },
        { text: "CLI", link: "/reference/cli" },
        { text: "Security Policy", link: "/reference/security-policy" },
        { text: "Changelog", link: "/reference/changelog" },
      ],
    },
  ],

  "/contributing/": [
    {
      text: "Contributing",
      items: [
        { text: "Overview", link: "/contributing/overview" },
        { text: "Dev Setup", link: "/contributing/dev-setup" },
        { text: "Architecture Decisions", link: "/contributing/architecture-decisions" },
        { text: "Dependency Policy", link: "/contributing/dependency-policy" },
        { text: "Release Process", link: "/contributing/release-process" },
      ],
    },
  ],

  "/community/": [
    {
      text: "Community",
      items: [
        { text: "Code of Conduct", link: "/community/code-of-conduct" },
        { text: "Governance", link: "/community/governance" },
        { text: "Maintainers", link: "/community/maintainers" },
        { text: "Support", link: "/community/support" },
      ],
    },
  ],

  "/trust/": [
    {
      text: "Trust",
      items: [
        { text: "OpenSSF Baseline", link: "/trust/openssf-baseline" },
        { text: "OpenSSF Best Practices", link: "/trust/openssf-best-practices" },
        { text: "SLSA Level 3", link: "/trust/slsa-level-3" },
        { text: "Security", link: "/trust/security" },
      ],
    },
  ],
};
```

- [ ] **Step 2: Wire `mainSidebar` into `docs-site/.vitepress/config.ts`.**

Edit the `sidebar` field in `themeConfig` from `sidebar: {}` to:

```ts
import { mainSidebar } from "./sidebars/main";
// ... within themeConfig:
    sidebar: { ...mainSidebar },
```

The `import` line goes at the top of `config.ts` next to `import { defineConfig } from "vitepress"`.

- [ ] **Step 3: Create the stub template at `docs-site/stubs/_template.md`.**

```md
---
title: "Coming Soon"
---

# {title}

::: warning Stub page
This page is a placeholder. The full content is being written. If you would
like to contribute, the source for this page lives at the path shown below
and you can [edit it on GitHub](https://github.com/ravencloak-org/Raven/edit/main/docs-site/stubs/{file}).
:::

## What goes here

A short bullet list of the topics this page will cover, drawn from the
[design spec](https://github.com/ravencloak-org/Raven/blob/main/docs/superpowers/specs/2026-05-09-docs-site-design.md).
```

- [ ] **Step 4: Generate one stub per IA leaf that doesn't have a mapped source in `content-map.json`.**

The mappings already cover: `get-started/installation`, `concepts/architecture`, `concepts/system-overview`, `concepts/data-model`, `contributing/dependency-policy`, `contributing/overview`, `reference/security-policy`, `community/code-of-conduct`, `community/governance`, `community/maintainers`, `trust/slsa-level-3`, `trust/openssf-baseline`, `trust/openssf-best-practices`.

Stubs needed (24 files): create each at `docs-site/stubs/<path>.md` using the template above with `{title}` and `{file}` substituted.

```
docs-site/stubs/get-started/first-knowledge-base.md          → "First Knowledge Base"
docs-site/stubs/get-started/embed-the-chat-widget.md         → "Embed the Chat Widget"
docs-site/stubs/get-started/try-the-voice-agent.md           → "Try the Voice Agent"
docs-site/stubs/guides/workspaces-and-tenancy.md             → "Workspaces & Tenancy"
docs-site/stubs/guides/ingestion.md                          → "Ingestion"
docs-site/stubs/guides/retrieval.md                          → "Retrieval"
docs-site/stubs/guides/llm-providers.md                      → "LLM Providers"
docs-site/stubs/guides/voice.md                              → "Voice"
docs-site/stubs/guides/webhooks-and-events.md                → "Webhooks & Events"
docs-site/stubs/guides/billing.md                            → "Billing"
docs-site/stubs/guides/self-hosting/docker-compose.md        → "Docker Compose"
docs-site/stubs/guides/self-hosting/edge-and-raspberry-pi.md → "Edge & Raspberry Pi"
docs-site/stubs/guides/self-hosting/traefik-and-tls.md       → "Traefik & TLS"
docs-site/stubs/guides/self-hosting/observability.md         → "Observability"
docs-site/stubs/guides/self-hosting/backups.md               → "Backups"
docs-site/stubs/guides/self-hosting/upgrades.md              → "Upgrades"
docs-site/stubs/guides/self-hosting/hardening.md             → "Hardening"
docs-site/stubs/concepts/multi-tenancy.md                    → "Multi-Tenancy"
docs-site/stubs/concepts/hybrid-retrieval.md                 → "Hybrid Retrieval"
docs-site/stubs/concepts/deployment-models.md                → "Deployment Models"
docs-site/stubs/concepts/ai-worker-design.md                 → "AI Worker Design"
docs-site/stubs/reference/configuration.md                   → "Configuration"
docs-site/stubs/reference/cli.md                             → "CLI"
docs-site/stubs/reference/changelog.md                       → "Changelog"
docs-site/stubs/contributing/dev-setup.md                    → "Dev Setup"
docs-site/stubs/contributing/architecture-decisions.md       → "Architecture Decisions"
docs-site/stubs/contributing/release-process.md              → "Release Process"
docs-site/stubs/community/support.md                         → "Support"
docs-site/stubs/trust/security.md                            → "Security"
```

For the changelog stub at `docs-site/stubs/reference/changelog.md`, replace the body with a short link to GitHub Releases:

```md
---
title: "Changelog"
---

# Changelog

The full changelog lives in [CHANGELOG.md](https://github.com/ravencloak-org/Raven/blob/main/CHANGELOG.md)
and on the [GitHub releases page](https://github.com/ravencloak-org/Raven/releases).
```

> **Note:** if `CHANGELOG.md` exists at the repo root (it does at the time of writing), the maintainer can later add it to `content-map.json` mappings (`{ "from": "CHANGELOG.md", "to": "reference/changelog.md" }`) and remove this stub. That swap is a follow-up commit, not part of T5.

- [ ] **Step 5: Edit `docs-site/content-map.json` to register the stub mappings.**

Replace the placeholder line:

```
    "stubMappings": "_filled-by-T5_"
```

with the populated list:

```json
    "stubMappings": [
      { "from": "docs-site/stubs/get-started/first-knowledge-base.md", "to": "get-started/first-knowledge-base.md" },
      { "from": "docs-site/stubs/get-started/embed-the-chat-widget.md", "to": "get-started/embed-the-chat-widget.md" },
      { "from": "docs-site/stubs/get-started/try-the-voice-agent.md", "to": "get-started/try-the-voice-agent.md" },
      { "from": "docs-site/stubs/guides/workspaces-and-tenancy.md", "to": "guides/workspaces-and-tenancy.md" },
      { "from": "docs-site/stubs/guides/ingestion.md", "to": "guides/ingestion.md" },
      { "from": "docs-site/stubs/guides/retrieval.md", "to": "guides/retrieval.md" },
      { "from": "docs-site/stubs/guides/llm-providers.md", "to": "guides/llm-providers.md" },
      { "from": "docs-site/stubs/guides/voice.md", "to": "guides/voice.md" },
      { "from": "docs-site/stubs/guides/webhooks-and-events.md", "to": "guides/webhooks-and-events.md" },
      { "from": "docs-site/stubs/guides/billing.md", "to": "guides/billing.md" },
      { "from": "docs-site/stubs/guides/self-hosting/docker-compose.md", "to": "guides/self-hosting/docker-compose.md" },
      { "from": "docs-site/stubs/guides/self-hosting/edge-and-raspberry-pi.md", "to": "guides/self-hosting/edge-and-raspberry-pi.md" },
      { "from": "docs-site/stubs/guides/self-hosting/traefik-and-tls.md", "to": "guides/self-hosting/traefik-and-tls.md" },
      { "from": "docs-site/stubs/guides/self-hosting/observability.md", "to": "guides/self-hosting/observability.md" },
      { "from": "docs-site/stubs/guides/self-hosting/backups.md", "to": "guides/self-hosting/backups.md" },
      { "from": "docs-site/stubs/guides/self-hosting/upgrades.md", "to": "guides/self-hosting/upgrades.md" },
      { "from": "docs-site/stubs/guides/self-hosting/hardening.md", "to": "guides/self-hosting/hardening.md" },
      { "from": "docs-site/stubs/concepts/multi-tenancy.md", "to": "concepts/multi-tenancy.md" },
      { "from": "docs-site/stubs/concepts/hybrid-retrieval.md", "to": "concepts/hybrid-retrieval.md" },
      { "from": "docs-site/stubs/concepts/deployment-models.md", "to": "concepts/deployment-models.md" },
      { "from": "docs-site/stubs/concepts/ai-worker-design.md", "to": "concepts/ai-worker-design.md" },
      { "from": "docs-site/stubs/reference/configuration.md", "to": "reference/configuration.md" },
      { "from": "docs-site/stubs/reference/cli.md", "to": "reference/cli.md" },
      { "from": "docs-site/stubs/reference/changelog.md", "to": "reference/changelog.md" },
      { "from": "docs-site/stubs/contributing/dev-setup.md", "to": "contributing/dev-setup.md" },
      { "from": "docs-site/stubs/contributing/architecture-decisions.md", "to": "contributing/architecture-decisions.md" },
      { "from": "docs-site/stubs/contributing/release-process.md", "to": "contributing/release-process.md" },
      { "from": "docs-site/stubs/community/support.md", "to": "community/support.md" },
      { "from": "docs-site/stubs/trust/security.md", "to": "trust/security.md" }
    ]
```

- [ ] **Step 6: Modify `docs-site/scripts/sync-content.ts` to also process `stubMappings`.**

Find the `main()` function and update it to iterate over both arrays:

```ts
async function main() {
  const map = await loadMap();
  await fs.rm(TMP_DIR, { recursive: true, force: true });
  await fs.mkdir(TMP_DIR, { recursive: true });

  for (const m of [...map.mappings, ...(map.stubMappings ?? [])]) {
    await copyFile(m.from, m.to, map.linkRewrites);
  }

  console.log(
    `sync-content: copied ${map.mappings.length} mapped + ${(map.stubMappings ?? []).length} stub files to ${path.relative(REPO_ROOT, TMP_DIR)}/`,
  );
}
```

Update the `ContentMap` interface accordingly:

```ts
interface ContentMap {
  mappings: Mapping[];
  stubMappings?: Mapping[];
  linkRewrites: LinkRewrite[];
}
```

- [ ] **Step 7: Build and verify every IA path resolves.**

```bash
cd docs-site
npm run build
ls .vitepress/dist/get-started/ .vitepress/dist/guides/ .vitepress/dist/concepts/ .vitepress/dist/api/ 2>/dev/null
```

Expected: directories exist for each top-level IA group; each leaf renders as `<slug>.html` or `<slug>/index.html`. Click-through in `npm run preview` should reach every sidebar link without 404.

- [ ] **Step 8: Commit.**

```bash
git add docs-site/.vitepress/sidebars/ docs-site/.vitepress/config.ts docs-site/stubs/ docs-site/content-map.json docs-site/scripts/sync-content.ts
git commit -s -m "feat(docs-site): IA sidebar + stub pages for unwritten content

Defines the navigation tree as TypeScript-typed sidebar groups in
.vitepress/sidebars/main.ts and registers it in config.ts. Adds 29
stub pages with a clear 'this is a placeholder' banner so every IA
leaf resolves; the sync step processes them alongside mapped sources.
Pages with real source content (architecture, data-model, OSPS-L2,
SLSA, etc.) continue to render from docs/ via content-map.json
mappings."
```

---

## Task 6: API reference + spec hosting + Beta banner

**Owner:** Wave 2, parallel within wave.

**Goal:** vitepress-openapi renders one URL per operation from `contracts/openapi.yaml`; the spec is also hosted as a downloadable artefact at `/api/openapi.{yaml,json}`. A Vue `BetaBanner` component is registered globally and rendered on every `/api/*` page until per-operation `x-status: stable` is set in the spec.

**Files:**
- Create: `docs-site/.vitepress/openapi.ts`
- Create: `docs-site/.vitepress/components/BetaBanner.vue`
- Modify: `docs-site/.vitepress/theme/index.ts` (register component)
- Modify: `docs-site/.vitepress/config.ts` (import + register API sidebar)
- Create: `docs-site/scripts/copy-spec.ts`
- Modify: `docs-site/package.json` (add vitepress-openapi + js-yaml; add `postbuild:spec` script)
- Create: `docs-site/api/overview.md`

### Steps

- [ ] **Step 1: Add the dependencies.**

```bash
cd docs-site
npm install vitepress-openapi js-yaml
npm install --save-dev @types/js-yaml
```

This updates `package.json` and `package-lock.json`.

- [ ] **Step 2: Create `docs-site/.vitepress/openapi.ts`.**

```ts
import { useSidebar } from "vitepress-openapi";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import yaml from "js-yaml";

const specPath = resolve(import.meta.dirname, "..", "..", "contracts", "openapi.yaml");
const spec = yaml.load(readFileSync(specPath, "utf8")) as Record<string, unknown>;

const sidebarBuilder = useSidebar({
  spec,
  prefix: "/api/",
  // One sidebar group per OpenAPI tag.
  collapsible: true,
});

export const apiSidebar = sidebarBuilder.generateSidebarGroups();
export { spec };
```

- [ ] **Step 3: Wire the API sidebar into `docs-site/.vitepress/config.ts`.**

Add the import next to `mainSidebar`:

```ts
import { mainSidebar } from "./sidebars/main";
import { apiSidebar } from "./openapi";
```

And update the `sidebar` config:

```ts
    sidebar: {
      ...mainSidebar,
      "/api/": apiSidebar,
    },
```

- [ ] **Step 4: Create the Beta banner at `docs-site/.vitepress/components/BetaBanner.vue`.**

```vue
<script setup lang="ts">
import { useData } from "vitepress";
import { computed } from "vue";

const { page, frontmatter } = useData();

// Show on every /api/* page UNLESS the operation declares x-status: stable.
// Operations are surfaced by vitepress-openapi as front-matter; we also
// allow a manual override via `betaBanner: false`.
const isApiPage = computed(() => page.value.relativePath.startsWith("api/"));
const stable = computed(() => frontmatter.value?.["x-status"] === "stable");
const muted = computed(() => frontmatter.value?.betaBanner === false);
const visible = computed(() => isApiPage.value && !stable.value && !muted.value);
</script>

<template>
  <div v-if="visible" class="beta-banner">
    <strong>Beta.</strong>
    The API surface is documented incrementally. Operations marked
    <code>x-status: stable</code> are guaranteed; everything else may change
    without notice.
  </div>
</template>

<style scoped>
.beta-banner {
  margin: 0 0 1.5rem;
  padding: 0.75rem 1rem;
  border-left: 4px solid var(--vp-c-brand-1);
  background: var(--vp-c-brand-soft);
  color: var(--vp-c-text-1);
  font-size: 0.95em;
  border-radius: 4px;
}
.beta-banner code {
  font-size: 0.9em;
  padding: 0.05em 0.35em;
  background: var(--vp-c-bg-soft);
  border-radius: 3px;
}
</style>
```

- [ ] **Step 5: Register `BetaBanner` globally in `docs-site/.vitepress/theme/index.ts`.**

Replace the file contents with:

```ts
import DefaultTheme from "vitepress/theme";
import { useOpenapi } from "vitepress-openapi/client";
import "vitepress-openapi/dist/style.css";
import "./style.css";
import BetaBanner from "../components/BetaBanner.vue";

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    useOpenapi();
    app.component("BetaBanner", BetaBanner);
  },
};
```

The `BetaBanner` component is registered globally so any markdown page (including the auto-generated `/api/*` pages) can render it as `<BetaBanner />`. Inserting it into the synthetic operation pages happens by including a small layout slot — see Step 6.

- [ ] **Step 6: Create the API overview at `docs-site/api/overview.md`.**

```md
---
title: "API Overview"
---

<BetaBanner />

# API Overview

Raven exposes a REST API from the Go backend. Every operation here is
generated from a single OpenAPI 3.1 spec — the same spec drives the Go
server's typed handlers via `oapi-codegen` (Phase 3, in progress) and the
docs you are reading now.

## Download the spec

The canonical OpenAPI spec is hosted alongside this site:

- [openapi.yaml](/api/openapi.yaml) (canonical)
- [openapi.json](/api/openapi.json) (auto-converted at build time)

Pipe it into Postman, Insomnia, Bruno, `openapi-generator`, or your own
`oapi-codegen` config:

```bash
curl -O https://docs.raven.ravencloak.org/api/openapi.yaml
oapi-codegen -package raven -generate types,client openapi.yaml > raven.gen.go
```

## Servers

| Environment | URL                              |
| ----------- | -------------------------------- |
| Production  | <https://api.ravencloak.org>     |
| Local dev   | <http://localhost:8080>          |

The production URL will move to `https://api.raven.ravencloak.org` when the
domain-migration ticket lands; this page will update automatically because
the URL is read from the same spec the Go server uses.

## Authentication

See [Authentication](/api/authentication) for the bearer-token scheme,
session cookies for the dashboard, and per-workspace API keys for embedding.
```

> **Note:** the markdown contains a Vue component reference (`<BetaBanner />`). VitePress evaluates that during the build because we're using the standard MDC integration; no extra config needed.

- [ ] **Step 7: Create `docs-site/scripts/copy-spec.ts` to ship the YAML and JSON variants.**

```ts
import { promises as fs } from "node:fs";
import path from "node:path";
import yaml from "js-yaml";

const REPO_ROOT = path.resolve(import.meta.dirname, "..", "..");
const DIST_API = path.resolve(import.meta.dirname, "..", ".vitepress", "dist", "api");
const SPEC_YAML = path.join(REPO_ROOT, "contracts", "openapi.yaml");

async function main() {
  await fs.mkdir(DIST_API, { recursive: true });

  const yamlContent = await fs.readFile(SPEC_YAML, "utf8");
  await fs.writeFile(path.join(DIST_API, "openapi.yaml"), yamlContent);

  const json = JSON.stringify(yaml.load(yamlContent), null, 2);
  await fs.writeFile(path.join(DIST_API, "openapi.json"), json);

  console.log(`copy-spec: emitted openapi.yaml + openapi.json into ${path.relative(REPO_ROOT, DIST_API)}/`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
```

- [ ] **Step 8: Update `docs-site/package.json` `scripts` to chain the spec copy after the build.**

```json
"scripts": {
  "sync": "tsx scripts/sync-content.ts",
  "predev": "npm run sync",
  "dev": "vitepress dev",
  "prebuild": "npm run sync",
  "build": "vitepress build && tsx scripts/copy-spec.ts && pagefind --site .vitepress/dist",
  "preview": "vitepress preview"
}
```

The `copy-spec` runs after `vitepress build` (so `dist/` exists) and before `pagefind` (so PageFind indexes the YAML/JSON files too — though PageFind ignores them as binaries; the chain order is mostly stylistic).

- [ ] **Step 9: Build and verify the API surface lands.**

```bash
cd docs-site
npm run build
ls .vitepress/dist/api/
```

Expected: `overview.html`, `openapi.yaml`, `openapi.json`, plus one `<operationId>.html` per operation defined in the spec. If `contracts/openapi.yaml` is empty/stub-shaped, only `overview.html` and the YAML/JSON appear — that's acceptable; Phase 2 fills in operations.

- [ ] **Step 10: Commit.**

```bash
git add docs-site/.vitepress/openapi.ts docs-site/.vitepress/components/BetaBanner.vue docs-site/.vitepress/theme/index.ts docs-site/.vitepress/config.ts docs-site/scripts/copy-spec.ts docs-site/package.json docs-site/package-lock.json docs-site/api/overview.md
git commit -s -m "feat(docs-site): API reference from contracts/openapi.yaml

Wires vitepress-openapi to render one URL per operation; ships the
canonical spec at /api/openapi.{yaml,json} for download. Adds the
BetaBanner Vue component (registered globally) shown on every /api/*
page unless the operation declares x-status: stable. Adds an API
overview page with the download buttons and a curl example for
oapi-codegen."
```

---

## Task 7: 404 + robots + sitemap + footer

**Owner:** Wave 2, parallel within wave.

**Goal:** Polish the operational surface — a custom 404 with PageFind search, machine-readable `robots.txt`, generated sitemap, and the footer copy.

**Files:**
- Create: `docs-site/404.md`
- Create: `docs-site/public/robots.txt`
- Modify: `docs-site/.vitepress/config.ts` (sitemap config + footer)

### Steps

- [ ] **Step 1: Create `docs-site/404.md` with embedded search.**

```md
---
layout: page
---

<script setup>
import { onMounted } from "vue";
onMounted(() => {
  const el = document.getElementById("pagefind-404");
  if (!el || el.dataset.mounted) return;
  el.dataset.mounted = "1";
  // VitePress already loads PageFind for the local search provider; reuse
  // it here so we don't ship the bundle twice.
  import(/* @vite-ignore */ "/_pagefind/pagefind-ui.js").then((m) => {
    new m.PagefindUI({ element: "#pagefind-404", showSubResults: true });
  });
});
</script>

# Page not found

The page you're looking for doesn't exist or has moved.

Try searching, or jump back to the [home page](/).

<div id="pagefind-404" style="margin-top: 2rem;"></div>
```

- [ ] **Step 2: Create `docs-site/public/robots.txt`.**

The `public/` directory is copied verbatim into `dist/` by VitePress.

```
User-agent: *
Allow: /

Sitemap: https://docs.raven.ravencloak.org/sitemap.xml
```

- [ ] **Step 3: Configure sitemap generation in `docs-site/.vitepress/config.ts`.**

Add the following block to the top-level `defineConfig({...})` (siblings of `themeConfig`):

```ts
  sitemap: {
    hostname: "https://docs.raven.ravencloak.org",
    transformItems: (items) => items.filter((i) => !i.url.includes("/404")),
  },
```

VitePress 1.x ships sitemap generation built-in via the `sitemap` config key; no plugin install needed.

- [ ] **Step 4: Update `themeConfig.footer` in `.vitepress/config.ts`.**

```ts
    footer: {
      message:
        'Released under the <a href="https://github.com/ravencloak-org/Raven/blob/main/LICENSE">Apache 2.0 License</a>. Spotted an error? <a href="https://github.com/ravencloak-org/Raven/issues/new">Open an issue</a>.',
      copyright: `Copyright © 2026-present <a href="https://github.com/ravencloak-org/Raven">Ravencloak Org</a>`,
    },
```

- [ ] **Step 5: Build and verify outputs.**

```bash
cd docs-site
npm run build
test -f .vitepress/dist/404.html && echo "404 OK"
test -f .vitepress/dist/robots.txt && echo "robots OK"
test -f .vitepress/dist/sitemap.xml && echo "sitemap OK"
head -5 .vitepress/dist/sitemap.xml
```

Expected: all three files present; sitemap XML lists every IA leaf URL.

- [ ] **Step 6: Commit.**

```bash
git add docs-site/404.md docs-site/public/ docs-site/.vitepress/config.ts
git commit -s -m "feat(docs-site): 404 with search, robots.txt, sitemap, footer

Custom 404 reuses PageFind for in-page search. robots.txt allows all
crawlers and points at the generated sitemap. VitePress's built-in
sitemap config emits dist/sitemap.xml at the configured hostname.
Footer shows license + 'open an issue' link."
```

---

## Task 8: CI workflow + Cloudflare deploy

**Owner:** Wave 3, sequential after Wave 2.

**Goal:** Every PR gets a Cloudflare Pages preview; pushes to `main` deploy production.

**Files:**
- Create: `.github/workflows/docs.yml`

### Steps

- [ ] **Step 1: Resolve the SHAs for the actions used.**

The actions need SHA-pinned references per the Scorecard policy from PR #445. Resolve each:

```bash
# actions/checkout latest stable v5 release
gh api repos/actions/checkout/git/refs/tags/v5.0.0 --jq '.object.sha'
# actions/setup-node latest stable v6 release
gh api repos/actions/setup-node/git/refs/tags/v6.0.0 --jq '.object.sha'
# marocchino/sticky-pull-request-comment latest v2 release
gh api repos/marocchino/sticky-pull-request-comment/git/refs/tags/v2.9.0 --jq '.object.sha'
```

Use the resolved 40-char commit SHAs in the workflow below (replace the four `<SHA>` placeholders, retaining the `# vN` trailing comment so Dependabot can keep the SHA up to date).

- [ ] **Step 2: Create `.github/workflows/docs.yml`.**

```yaml
name: Docs

on:
  push:
    branches: [main]
    paths:
      - "docs/**"
      - "docs-site/**"
      - "contracts/**"
      - "CHANGELOG.md"
      - "CONTRIBUTING.md"
      - "SECURITY.md"
      - "CODE_OF_CONDUCT.md"
      - "GOVERNANCE.md"
      - "MAINTAINERS.md"
      - "README.md"
      - ".github/workflows/docs.yml"
  pull_request:
    branches: [main]
    paths:
      - "docs/**"
      - "docs-site/**"
      - "contracts/**"
      - "CHANGELOG.md"
      - "CONTRIBUTING.md"
      - "SECURITY.md"
      - "CODE_OF_CONDUCT.md"
      - "GOVERNANCE.md"
      - "MAINTAINERS.md"
      - "README.md"
      - ".github/workflows/docs.yml"

permissions: {}

concurrency:
  group: docs-${{ github.ref }}
  cancel-in-progress: true

jobs:
  build-and-deploy:
    name: Build & Deploy
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions:
      contents: read
      deployments: write
      pull-requests: write

    steps:
      - name: Checkout
        uses: actions/checkout@<SHA>  # v5.0.0
        with:
          persist-credentials: false

      - name: Setup Node.js
        uses: actions/setup-node@<SHA>  # v6.0.0
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: docs-site/package-lock.json

      - name: Lint OpenAPI spec
        run: |
          npx -y @stoplight/spectral-cli@6 lint \
            contracts/openapi.yaml \
            --ruleset contracts/.spectral.yaml \
            --fail-severity=error

      - name: Install dependencies
        working-directory: docs-site
        run: npm ci

      - name: Build site
        working-directory: docs-site
        run: npm run build

      - name: Deploy to Cloudflare Pages
        id: deploy
        env:
          CLOUDFLARE_API_TOKEN: ${{ secrets.CLOUDFLARE_API_TOKEN }}
          CLOUDFLARE_ACCOUNT_ID: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}
        working-directory: docs-site
        run: |
          set -o pipefail
          npx wrangler@4 pages deploy .vitepress/dist \
            --project-name=raven-docs \
            --branch="${{ github.head_ref || github.ref_name }}" \
            | tee wrangler.out
          url=$(grep -oE 'https://[a-z0-9-]+\.raven-docs\.pages\.dev' wrangler.out | head -1)
          if [ -z "$url" ]; then
            echo "::error::Could not parse preview URL from wrangler output"
            exit 1
          fi
          echo "url=$url" >> "$GITHUB_OUTPUT"

      - name: Comment preview URL on PR
        if: github.event_name == 'pull_request'
        uses: marocchino/sticky-pull-request-comment@<SHA>  # v2.9.0
        with:
          header: docs-preview
          message: |
            📚 Docs preview: ${{ steps.deploy.outputs.url }}

            Built from `${{ github.sha }}`. Updated automatically on every push to this PR.
```

- [ ] **Step 3: Verify the workflow with `actionlint` (if installed locally).**

```bash
which actionlint && actionlint .github/workflows/docs.yml || echo "actionlint not installed; skipping"
```

If `actionlint` is installed and reports errors, fix them inline. If not installed, do **not** install it — the CI gate for workflow correctness is the actual run on GitHub.

- [ ] **Step 4: Commit.**

```bash
git add .github/workflows/docs.yml
git commit -s -m "ci(docs): build and deploy docs.raven.ravencloak.org

GitHub Actions workflow that lints contracts/openapi.yaml with
Spectral, builds the VitePress site, and deploys to Cloudflare Pages
via wrangler@4. Posts a sticky preview-URL comment on every PR.

Triggers on changes to docs/, docs-site/, contracts/, the root-level
mirrored docs (CHANGELOG/CONTRIBUTING/SECURITY/CoC/GOVERNANCE/MAINTAINERS/README),
and the workflow file itself.

Actions are SHA-pinned per the Scorecard policy established in PR #445."
```

- [ ] **Step 5: Push the branch and open the PR.**

```bash
git push -u origin docs/spec-docs-site-design
gh pr create --title "docs: ship docs.raven.ravencloak.org (VitePress + spec-fed API reference)" --body "$(cat <<'EOF'
## Summary

Implements the [docs site design spec](docs/superpowers/specs/2026-05-09-docs-site-design.md)
(commit \`811d28d6\`).

- New top-level \`docs-site/\` directory: VitePress + vitepress-openapi + PageFind, black-on-white theme with a red accent
- \`contracts/openapi.yaml\` (renamed from \`openapi-stub.yaml\`) becomes the single source of truth, gated by Spectral lint in CI, and hosted at \`/api/openapi.{yaml,json}\` for download
- Build-time content sync from \`docs/\` and root-level mirrored docs (CONTRIBUTING/SECURITY/etc.) into the gitignored \`docs-site/.tmp/\` tree per \`content-map.json\`
- 29 stub pages cover the IA leaves whose source content does not yet exist
- New \`.github/workflows/docs.yml\` deploys via \`wrangler@4 pages deploy\` to Cloudflare Pages; PRs get a sticky preview-URL comment
- README gains a Documentation section

## Phasing

- **Phase 1 (this PR):** docs site live with whatever the current stub spec covers; \`/api/*\` shows the Beta banner
- **Phase 2 (separate spec):** complete \`contracts/openapi.yaml\` for every existing endpoint
- **Phase 3 (separate spec):** adopt \`oapi-codegen\` for spec-first Go server

## Out of scope

The embedded \`<raven-chat>\` widget on \`/get-started/\`, the live demo workspace seed, and the \`*.raven.ravencloak.org\` domain migration of \`api/app/auth/livekit/logs/monitor\` are tracked as separate sub-projects per the spec.

## Test plan
- [ ] Cloudflare Pages preview URL renders cleanly
- [ ] Every IA sidebar link reaches its page (no 404s)
- [ ] \`/api/openapi.yaml\` and \`/api/openapi.json\` resolve and parse
- [ ] PageFind search returns hits for queries against existing migrated content (e.g. "OSPS-L2", "SLSA")
- [ ] Lighthouse on \`/\` and a content page: Performance ≥ 95, Accessibility ≥ 95, SEO ≥ 95
- [ ] Once the production deploy lands and DNS is wired, \`https://docs.raven.ravencloak.org\` resolves with valid HTTPS

## Manual one-time steps after merge

1. \`npx wrangler@4 pages project create raven-docs --production-branch main\` (one shot)
2. CF dashboard → Workers & Pages → raven-docs → Custom domains → Set up \`docs.raven.ravencloak.org\`
3. Accept the auto-suggested CNAME in the \`ravencloak.org\` zone
EOF
)"
gh pr merge "$(gh pr view --json number -q .number)" --auto --squash
```

- [ ] **Step 6: Verify the PR opened cleanly.**

```bash
gh pr view --json number,state,headRefOid,statusCheckRollup --jq '{n: .number, state, head: .headRefOid, checks: [.statusCheckRollup[] | {name: (.name // .context), status, conclusion}]}'
```

Expected: PR open; auto-merge queued; CI runs trigger Build & Deploy and post a preview URL comment within ~2 minutes.

---

## Self-Review

**Spec coverage check (against `docs/superpowers/specs/2026-05-09-docs-site-design.md`):**

| Spec section | Covered by |
| ------------ | ---------- |
| Architecture (stack pin) | T2, T6 |
| Repo layout | T2, T4, T5, T6, T7 |
| IA tree | T5 (sidebar) + T4 (mappings for sourced pages) + T5 (stubs for the rest) |
| Content sourcing rules table | T4 (mappings) + T5 (stubs) |
| API reference flow | T6 |
| Spec hosting (`/api/openapi.{yaml,json}`) | T6 |
| Spec server URLs | Spec content; future-flip mentioned in T6 overview page |
| Try-It panel (off in Phase 1) | T6 (vitepress-openapi defaults; T6 doesn't enable Try-It explicitly) |
| Beta banner | T6 |
| Phased migration | Documented in spec; this plan covers Phase 1 only |
| Build & deploy pipeline | T8 |
| One-time bootstrap steps | T8 PR description |
| Performance budget | T8 (CI timeout = 10 min; 2-min target validated by run time) |
| Branding & theme | T2 |
| Operational decisions (404 / robots / sitemap / OG / footer / changelog) | T7 + T5 (changelog stub) + T2 (OG meta defaults via VitePress) |
| Tracking items / follow-ups | Not in plan (correctly out of scope) |
| Acceptance criteria | All ten satisfied by tasks + verified by T8 PR test plan |
| Risks and mitigations | Spec-side; plan inherits |

**Placeholder scan:** all `<SHA>` references are explicit "resolve and substitute" steps in T8 with the resolution command provided. No "TODO/TBD/implement later" content in steps.

**Type consistency:** `mainSidebar` named the same in T5 and T2 import slot; `apiSidebar` named the same in T6 and `config.ts` import; `ContentMap` interface T4 baseline + T5 extension is consistent (`stubMappings?: Mapping[]`). Path strings (`/get-started/installation` etc.) match between sidebar (T5), homepage (T2), config nav (T2), and content-map mappings (T4).

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-05-09-docs-site-implementation.md`.**

The user previously requested **parallel agent execution**, so the recommended path is **Subagent-Driven (Wave-orchestrated)**: dispatch a fresh subagent per task within each wave concurrently, gate each wave on the prior wave's commits landing on the worktree branch, review between tasks where the spec demands a precision pass (T6 specifically — vitepress-openapi config has a few non-trivial integration points).

The orchestrator (parent agent) is responsible for:

1. **Wave 1:** dispatch T1, T2, T3 in parallel (three tool calls in one message). Wait for all three to commit. Verify commits land cleanly with no conflict.
2. **Wave 2:** dispatch T4, T5, T6, T7 in parallel after T2 commits. T6 reads `contracts/openapi.yaml` so T1 must be merged too. Wait for all four. Verify the build is green.
3. **Wave 3:** dispatch T8. After it commits and pushes the PR, the orchestrator queues `gh pr merge --auto --squash` per CLAUDE.md.

If at any wave a subagent fails (lockfile conflict, missing dep version, or content-filter false-positive), the orchestrator retries the failing task once with the same prompt before falling back to inline execution for that task.
