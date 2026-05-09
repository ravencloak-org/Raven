# Raven Docs Site — Design

| Field      | Value                                            |
| ---------- | ------------------------------------------------ |
| Status     | DRAFT — pending review by Jobin Lawrance         |
| Date       | 2026-05-09                                       |
| Author     | Jobin Lawrance                                   |
| Spec ID    | docs-site-2026-05-09                             |

## Summary

Stand up a public documentation site at **`docs.raven.ravencloak.org`** built
with VitePress, fed by markdown in the existing `docs/` directory and an
OpenAPI 3.1 spec at `contracts/openapi.yaml`. Every Cloudflare-hosted Raven
service consolidates under `*.raven.ravencloak.org` over time; the docs site
launches under that subdomain immediately, with a follow-up ticket migrating
`api`, `app`, `auth`, `livekit`, `logs`, and `monitor` from `*.ravencloak.org`.

The spec deliberately separates **three sub-projects** that together form the
"people can try Raven" goal: this spec covers Sub-project 3 (docs site) only.
Sub-project 1 (hosting/operating Raven) and Sub-project 2 (demo seed +
embedded chat-widget configuration) are tracked separately.

## Goals

- Ship `docs.raven.ravencloak.org` as a static, fast, accessible documentation
  surface for **four audiences**: end users, self-host operators, API
  consumers, contributors.
- Adopt **spec-first** API documentation: a single `contracts/openapi.yaml`
  becomes the source of truth, rendered into the docs as per-operation pages
  and (later) consumed by `oapi-codegen` to generate Go server types and
  handler interfaces.
- Build foundation that can later host an embedded `<raven-chat>` demo widget
  without re-architecture (Sub-project 2 follow-up).
- Stay consistent with existing repo conventions: Cloudflare Pages via the
  `wrangler` CLI in GitHub Actions, SHA-pinned actions, DCO sign-off,
  squash-merge only, no AI attribution in commits.

## Non-goals

- **No embedded chat widget yet.** A "Try the Demo — coming soon" card holds
  the position on `/get-started/`.
- **No SDKs** in any language. Defer until oapi-codegen Phase 3 stabilises
  the spec.
- **No versioned docs.** Single "latest" tracks `main`. Revisit at v1.0.
- **No i18n.** English only.
- **No Algolia.** PageFind handles search.
- **No video tutorials.** Markdown + screenshots only.
- **No domain migration of the other services in this spec.** Tracked as a
  separate follow-up ticket.
- **No internal docs published.** `docs/research/`, `docs/design/`,
  `docs/superpowers/`, `docs/security/incident-response.md`,
  `docs/compliance/dpia-template.md`, `docs/project-status.md` stay in-repo.
- **No `PRIVACY.md` / `DPA.md` publication.** Held back until counsel
  removes their DRAFT banners.
- **No CMS.** Markdown in the repo is the only content source.
- **No analytics or cookies.** Privacy-friendly default; no consent prompt.

## Architecture

```
contracts/openapi.yaml ────┐
                           │  (single spec, source of truth)
                           ▼
              ┌──────────────────────────────┐
              │   docs-site/  (VitePress)     │
              │   ─────────────────────────── │
              │   • scripts/sync-content.ts → reads markdown from /docs/*.md
              │   • .vitepress/config.ts      → reads contracts/openapi.yaml
              │   • vitepress-openapi plugin  → generates per-operation pages
              │   • theme/                    → minor brand overrides
              │   • postbuild: pagefind index │
              └────────────┬──────────────────┘
                           │  npm run build
                           ▼
              ┌──────────────────────────────┐
              │  GitHub Actions: docs.yml      │
              │  ──────────────────────────────│
              │  • spectral lint contracts/openapi.yaml
              │  • npm ci && npm run build      │
              │  • npx wrangler@4 pages deploy │
              │  • parse + comment preview URL │
              └────────────┬──────────────────┘
                           │
                           ▼
              ┌──────────────────────────────┐
              │  Cloudflare Pages              │
              │  project: raven-docs          │
              │  custom domain:                │
              │  docs.raven.ravencloak.org    │
              └──────────────────────────────┘
```

### Stack pin

| Layer         | Choice              | Reason                                                                                  |
| ------------- | ------------------- | --------------------------------------------------------------------------------------- |
| SSG           | **VitePress** (1.x) | Vue stack alignment; markdown-first; PageFind native; small build; decoupled from `landing/` |
| API renderer  | **vitepress-openapi** | Static `.md` per operation → SEO + PageFind indexable; per-operation deep links         |
| Search        | **PageFind**         | Local-first, build-time, ~100 KB index, zero ops, no signup                              |
| Hosting       | **Cloudflare Pages** | Same provider as `landing/` and `frontend/`; global edge cache                          |
| Deploy        | **`wrangler@4` CLI** | Direct upload from GH Actions; no CF-dashboard build config to drift                    |
| Lint          | **Spectral**         | OpenAPI lint gate before each deploy                                                    |
| Index sync    | Custom `sync-content.ts` | Maps `docs/*.md` → `docs-site/.tmp/*.md` per `content-map.json`                          |

### Repo layout

```
docs/                            ← canonical markdown (contributors edit here)
docs-site/
├── package.json                 ← VitePress + vitepress-openapi + pagefind + wrangler
├── package-lock.json
├── content-map.json             ← source-path → site-path mapping
├── scripts/
│   └── sync-content.ts          ← copies docs/ → .tmp/ at build time
├── .vitepress/
│   ├── config.ts                ← VitePress config; reads contracts/openapi.yaml
│   ├── theme/                   ← brand overrides (logo, colours, footer)
│   ├── components/              ← any custom Vue components
│   └── dist/                    ← build output (gitignored)
└── .tmp/                        ← synced markdown (gitignored)
contracts/
├── openapi.yaml                 ← renamed from openapi-stub.yaml in this PR
└── .spectral.yaml               ← lint ruleset
.github/workflows/docs.yml       ← new CI pipeline
```

The existing `docs/` directory keeps its role as the source of truth for
contributor-authored prose. `docs-site/` holds only build tooling and theme
code, no content. The build's `sync-content.ts` step copies `docs/`
markdown into `docs-site/.tmp/` according to `content-map.json`, applying
URL rewrites for cross-links between source files. This keeps the
`docs/` tree readable in the GitHub web UI, in IDE markdown previews, and
to CodeRabbit, while letting VitePress impose its own `srcDir`.

## Information architecture

Top-level navigation, Diátaxis-shaped (Tutorials / How-to / Reference /
Explanation) plus standard OSS sections.

```
docs.raven.ravencloak.org/
├── /                            ← landing: 3-card hero (Get Started, Self-Host, API)
├── /get-started/                ← tutorial-style first-run path
│   ├── installation/            ← Docker Compose quickstart
│   ├── first-knowledge-base/
│   ├── embed-the-chat-widget/
│   └── try-the-voice-agent/
├── /guides/                     ← task-oriented "how do I…"
│   ├── workspaces-and-tenancy/
│   ├── ingestion/               ← sources, chunking, embeddings, BYOK
│   ├── retrieval/               ← hybrid search, RRF, reranking
│   ├── llm-providers/           ← Anthropic, OpenAI, Cohere, BYOK
│   ├── voice/                   ← LiveKit pipeline, STT/TTS
│   ├── webhooks-and-events/
│   ├── self-hosting/            ← operator subsection
│   │   ├── docker-compose/
│   │   ├── edge-and-raspberry-pi/
│   │   ├── traefik-and-tls/
│   │   ├── observability/       ← OpenObserve + Beszel
│   │   ├── backups/             ← pgBackRest
│   │   ├── upgrades/
│   │   └── hardening/
│   └── billing/                 ← Hyperswitch integration
├── /concepts/                   ← "why it's built this way"
│   ├── architecture/            ← from docs/architecture.md
│   ├── data-model/              ← from docs/wiki/Data-Model.md
│   ├── multi-tenancy/           ← RLS, org/workspace/KB
│   ├── hybrid-retrieval/        ← vector + BM25 + RRF
│   ├── deployment-models/       ← cloud / edge / hybrid
│   └── ai-worker-design/        ← gRPC, providers, processors
├── /api/                        ← vitepress-openapi-generated
│   ├── overview/                ← + Download spec button (yaml + json)
│   ├── authentication/
│   ├── pagination-and-errors/
│   └── <one route per operation, grouped by tag>
├── /reference/
│   ├── configuration/           ← env vars, compose service options
│   ├── cli/                     ← planned binary commands
│   ├── security-policy/         ← from SECURITY.md
│   └── changelog/               ← rendered from CHANGELOG.md if present
├── /contributing/
│   ├── overview/                ← from CONTRIBUTING.md
│   ├── dev-setup/
│   ├── architecture-decisions/  ← ADR index
│   ├── dependency-policy/       ← from docs/dependency-policy.md
│   └── release-process/
├── /community/
│   ├── code-of-conduct/         ← from CODE_OF_CONDUCT.md
│   ├── governance/              ← from GOVERNANCE.md
│   ├── maintainers/             ← from MAINTAINERS.md
│   └── support/
└── /trust/                      ← public compliance posture
    ├── openssf-baseline/        ← from docs/compliance/osps-l2-2026-02-19.md
    ├── openssf-best-practices/  ← from docs/compliance/openssf-best-practices-12590.md
    ├── slsa-level-3/            ← from docs/security/slsa-verification.md
    └── security/                ← summary; defers to /reference/security-policy/
```

### Content sourcing rules

| Source file in `docs/` (or repo root)                  | Site path                                  | Notes                                   |
| ------------------------------------------------------ | ------------------------------------------ | --------------------------------------- |
| `docs/quickstart.md`                                   | `/get-started/installation/`               | Migrate verbatim                        |
| `docs/architecture.md`                                 | `/concepts/architecture/`                  | Migrate verbatim                        |
| `docs/wiki/Architecture-Overview.md`                   | `/concepts/system-overview/`               | Migrate verbatim                        |
| `docs/wiki/Data-Model.md`                              | `/concepts/data-model/`                    | Migrate verbatim                        |
| `docs/dependency-policy.md`                            | `/contributing/dependency-policy/`         | Migrate verbatim                        |
| `docs/security/slsa-verification.md`                   | `/trust/slsa-level-3/`                     | Migrate verbatim                        |
| `docs/compliance/osps-l2-2026-02-19.md`                | `/trust/openssf-baseline/`                 | Migrate verbatim                        |
| `docs/compliance/openssf-best-practices-12590.md`      | `/trust/openssf-best-practices/`           | Already public-shaped                   |
| `CONTRIBUTING.md` (root)                               | `/contributing/overview/`                  | Mirror; root file remains source        |
| `SECURITY.md` (root)                                   | `/reference/security-policy/`              | Mirror                                  |
| `CODE_OF_CONDUCT.md` (root)                            | `/community/code-of-conduct/`              | Mirror                                  |
| `GOVERNANCE.md` (root)                                 | `/community/governance/`                   | Mirror                                  |
| `MAINTAINERS.md` (root)                                | `/community/maintainers/`                  | Mirror                                  |
| `CHANGELOG.md` (root, if present at build time)        | `/reference/changelog/`                    | Else link to GitHub Releases            |
| `docs/security/incident-response.md`                   | *(internal — not published)*               | Operational runbook                     |
| `docs/compliance/dpia-template.md`                     | *(internal — not published)*               | Template, not public-facing             |
| `docs/swagger/*`                                       | *(removed)*                                | Replaced by `/api/`                     |
| `docs/research/*`, `docs/design/*`,                    | *(internal — not published)*               | Stay in repo                            |
| `docs/superpowers/specs/*`, `docs/project-status.md`   |                                            |                                         |
| `PRIVACY.md`, `DPA.md` (root)                          | *(blocked — DRAFT)*                        | Publish after counsel removes banners   |

The mapping table is materialised in `docs-site/content-map.json`; the
`sync-content.ts` step rewrites cross-document links between source files
according to this table at build time.

## API reference flow

```
contracts/openapi.yaml ──► spectral lint (CI gate)
                             │
                             ▼
docs-site/.vitepress/config.ts  imports the spec
                             │
                             ▼
vitepress-openapi  generates one .md per operation
                   • route: /api/<tag-slug>/<operationId>/
                   • sidebar: grouped by `tags`
                   • each page: request schema, response schemas, examples
                             │
                             ▼
PageFind indexes every operation page at build time
                             │
                             ▼
Cloudflare Pages serves static HTML
```

### Spec hosting

The build copies the canonical YAML and an auto-converted JSON variant into
the static output:

- `https://docs.raven.ravencloak.org/api/openapi.yaml`
- `https://docs.raven.ravencloak.org/api/openapi.json`

`/api/overview/` exposes a "Download spec" button linking both. Integrators
can pipe the spec into Postman, Insomnia, Bruno, `openapi-generator`, or
their own `oapi-codegen` setup.

### Spec server URLs

The OpenAPI `servers:` block lists:

```yaml
servers:
  - url: https://api.ravencloak.org
    description: Production
  - url: http://localhost:8080
    description: Local development
```

The production URL flips to `https://api.raven.ravencloak.org` in a
**one-line change** when the domain-migration follow-up ticket lands.

### Try-It panel

- **Phase 1:** disabled. The spec is incomplete; users hitting endpoints
  whose schema doesn't match the live handler would get confusing
  failures.
- **Phase 2:** enabled, points at the production server. CORS for
  `https://docs.raven.ravencloak.org` is added to the Go API's CORS
  allowlist (`internal/middleware/cors`). Users supply their own API
  token.

### Beta banner

Until Phase 2 lands a complete spec, every `/api/*` page shows a banner:

> **Beta.** The API surface is documented incrementally. Operations
> marked `x-status: stable` are guaranteed; everything else may change
> without notice.

The banner is rendered by a small Vue component in
`docs-site/.vitepress/theme/`. It reads the `x-status` extension from
each operation; `stable` suppresses the banner on that page.

### Phased migration

| Phase | Scope | Deliverable | Blocks docs site? |
| ----- | ----- | ----------- | ----------------- |
| **1 — Ship docs** | Rename `openapi-stub.yaml` → `openapi.yaml`. Wire vitepress-openapi. Add Spectral lint. Ship docs to Cloudflare Pages. | docs.raven.ravencloak.org live with whatever the stub covers (Beta banner) | — |
| **2 — Complete the spec** | Hand-write OpenAPI 3.1 entries for every existing endpoint. Mark `x-status: stable` per reviewed operation. Enable Try-It. | Full API reference; spec used as canonical contract. | No (docs auto-update) |
| **3 — oapi-codegen** | Add `oapi-codegen` config (`cfg.yaml`). Generate `internal/api/types.gen.go` and `internal/api/server.gen.go`. Retrofit handlers to satisfy the generated `ServerInterface`. Remove `swag` deps + annotations. | Spec-first Go server. Docs unchanged. | No |

## Build & deploy pipeline

### Workflow shape (`.github/workflows/docs.yml`)

```yaml
name: Docs

on:
  push:
    branches: [main]
    paths:
      - 'docs/**'
      - 'docs-site/**'
      - 'contracts/**'
      - '.github/workflows/docs.yml'
  pull_request:
    branches: [main]
    paths:
      - 'docs/**'
      - 'docs-site/**'
      - 'contracts/**'
      - '.github/workflows/docs.yml'

permissions: {}

jobs:
  build-and-deploy:
    name: Build & Deploy
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions:
      contents: read
      deployments: write          # Cloudflare Pages deployments
      pull-requests: write        # comment preview URL
    steps:
      - uses: actions/checkout@<sha>  # v5
      - uses: actions/setup-node@<sha>  # v6
        with:
          node-version: '22'
          cache: npm
          cache-dependency-path: docs-site/package-lock.json

      - name: Lint OpenAPI spec
        run: npx -y @stoplight/spectral-cli@<pinned> lint contracts/openapi.yaml --fail-severity=error

      - name: Install dependencies
        run: npm ci --prefix docs-site

      - name: Build site (sync content + VitePress + PageFind)
        run: npm run build --prefix docs-site

      - name: Deploy to Cloudflare Pages
        id: deploy
        env:
          CLOUDFLARE_API_TOKEN: ${{ secrets.CLOUDFLARE_API_TOKEN }}
          CLOUDFLARE_ACCOUNT_ID: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}
        run: |
          npx wrangler@4 pages deploy docs-site/.vitepress/dist \
            --project-name=raven-docs \
            --branch="${{ github.head_ref || github.ref_name }}" \
            | tee wrangler.out
          url=$(grep -oE 'https://[a-z0-9-]+\.raven-docs\.pages\.dev' wrangler.out | head -1)
          echo "url=$url" >> "$GITHUB_OUTPUT"

      - name: Comment preview URL on PR
        if: github.event_name == 'pull_request'
        uses: marocchino/sticky-pull-request-comment@<sha>
        with:
          header: docs-preview
          message: |
            📚 Docs preview: ${{ steps.deploy.outputs.url }}
```

All `uses:` references SHA-pinned per the Scorecard policy established in
PR #445. The wrangler version pin matches `landing/` (`wrangler@4`).

### Cloudflare Pages project

| Setting          | Value                                        |
| ---------------- | -------------------------------------------- |
| Project name     | `raven-docs`                                 |
| Production branch| `main`                                       |
| Mode             | Direct upload (no CF-side build)             |
| Custom domain    | `docs.raven.ravencloak.org`                  |
| Preview pattern  | `<branch>.raven-docs.pages.dev`              |

### One-time bootstrap (manual)

These steps run once, outside any PR:

1. **Create the Pages project**
   ```bash
   npx wrangler@4 pages project create raven-docs --production-branch main
   ```

2. **Add the custom domain.** Wrangler v4 dropped the
   `pages project domain` subcommand; use either the CF dashboard or the
   API directly. Dashboard is the recommended path.

   - **Dashboard:** *Workers & Pages → raven-docs → Custom domains → Set up a custom domain*. Enter `docs.raven.ravencloak.org`. Activate.
   - **API (alternative):**
     ```bash
     curl -X POST \
       -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
       -H "Content-Type: application/json" \
       "https://api.cloudflare.com/client/4/accounts/$CLOUDFLARE_ACCOUNT_ID/pages/projects/raven-docs/domains" \
       -d '{"name":"docs.raven.ravencloak.org"}'
     ```

3. **DNS.** Cloudflare auto-suggests a CNAME from `docs.raven` →
   `raven-docs.pages.dev` in the `ravencloak.org` zone. Accept it.

4. **Secrets.** `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` already
   exist as repo secrets from the `landing/` deploy. Confirm with `gh
   secret list`. Token scope must include `Pages:Edit` and `Zone:DNS:Edit`
   for `ravencloak.org`.

### Performance budget

| Stage              | Target |
| ------------------ | ------ |
| Spectral lint      | < 5 s  |
| `npm ci` (cached)  | < 15 s |
| Sync + VitePress build | < 30 s |
| PageFind index     | < 5 s  |
| Wrangler upload    | < 30 s |
| Total CI cycle     | < 2 min |

PageFind index size budget: < 200 KB (gzipped).

## Domain & DNS

### This spec's domain

- `docs.raven.ravencloak.org` — Cloudflare Pages custom domain on the
  `raven-docs` project.

### Follow-up ticket: full `*.raven.ravencloak.org` migration

A separate spec (not this one) will migrate the existing services from
`*.ravencloak.org` to `*.raven.ravencloak.org`. The complete target
namespace is:

| Service        | Current host                  | Target host                              |
| -------------- | ----------------------------- | ---------------------------------------- |
| Frontend       | (CF Pages, var-driven)        | `app.raven.ravencloak.org`               |
| Go API         | `api.ravencloak.org`          | `api.raven.ravencloak.org`               |
| Auth           | `auth.ravencloak.org`         | `auth.raven.ravencloak.org`              |
| LiveKit        | `livekit.ravencloak.org`      | `livekit.raven.ravencloak.org`           |
| OpenObserve    | `logs.ravencloak.org`         | `logs.raven.ravencloak.org`              |
| Beszel         | `monitor.ravencloak.org`      | `monitor.raven.ravencloak.org`           |
| Docs           | (new)                         | `docs.raven.ravencloak.org` *(this spec)* |

Files the follow-up touches: `deploy/cloudflare-pages.json`,
`deploy/ansible/files/glance/config/glance.yml`,
`internal/middleware/cors`, `contracts/openapi.yaml` (`servers:` block),
`README.md` cross-references, and `frontend/`'s LiveKit URL constant.

The docs site does **not** depend on this migration and ships first.
When the migration lands, the only docs-site change is updating
`contracts/openapi.yaml`'s `servers:` block — a one-line edit.

## Branding & theme

- **Colours.** Black on white (light) and white on near-black (dark)
  with a single red accent for links, focused elements, and active nav
  items. Specific tokens:
  - `--vp-c-brand-1`: red (final hex chosen during implementation, must
    pass WCAG AA against both backgrounds — start from `#E11D48` /
    `tailwind rose-600` and adjust)
  - `--vp-c-brand-2`, `--vp-c-brand-3`, `--vp-c-brand-soft`: derived
    shades.
- **Logo.** Source from `ravenlogoassets/` (added in PR #408). The
  build embeds:
  - SVG wordmark in nav (light + dark variants)
  - Square favicon
  - PNG fallback for the OG card
- **Typography.** VitePress defaults (Inter for prose, JetBrains Mono
  for code). Don't customise unless required for brand parity with
  `landing/`.
- **Dark mode.** Default to system preference; user toggle persists per
  domain.
- **Theme file.** `docs-site/.vitepress/theme/` extends the default theme
  with the brand tokens above; minimal Vue component overrides only.

## Operational decisions

| Decision                  | Choice                                                                              |
| ------------------------- | ----------------------------------------------------------------------------------- |
| Edit-on-GitHub link       | Points at the **source** file in `docs/` (or repo root for mirrored files), never `.tmp/` |
| 404 page                  | Custom; centred PageFind search box; "Back to home" + "Report a typo" CTAs          |
| Sitemap.xml               | Generated at build time via `vitepress-plugin-sitemap` (or built-in if available)   |
| `robots.txt`              | Generated; allows all crawlers; declares sitemap location                            |
| OG / Twitter meta cards   | Auto-generated per page from `<title>` + first heading; Raven logo as fallback OG   |
| Changelog surface         | Renders `CHANGELOG.md` in-page at `/reference/changelog/` if present at build; else links to GitHub Releases |
| Footer                    | GitHub repo link, license badge, "Edit this page", "Report an issue", copyright     |
| Analytics                 | None. No cookies, no consent banner needed.                                         |

## Tracking items / follow-ups (NOT in this spec)

1. **Sub-project 1 — hosting & ops.** Stand up a public Raven instance.
   Decisions: host (EC2 / Hetzner / Pi / DigitalOcean), DNS, TLS,
   SuperTokens admin, secrets (TMDB, LLM keys, AWS SES, Hyperswitch),
   pgBackRest, observability cutover, monthly LLM-spend budget cap,
   per-IP rate limit on the demo workspace.
2. **Sub-project 2 — public demo.** Run `seed-demo` against the live
   instance. Issue a long-lived demo workspace API key. Configure CORS
   for `https://docs.raven.ravencloak.org` on the Go API. Test the chat
   widget against the seeded TMDB data. Add abuse protections.
3. **Embed `<raven-chat>` widget on docs.** Once SP1 + SP2 land, swap
   the "Try the Demo — coming soon" placeholder for the live web
   component, configured against the demo workspace's public API key.
4. **Domain migration.** Move `app/api/auth/livekit/logs/monitor` from
   `*.ravencloak.org` to `*.raven.ravencloak.org` (separate spec).
5. **OpenAPI Phase 2.** Hand-write entries for every existing endpoint;
   mark `x-status: stable` per review; enable Try-It.
6. **OpenAPI Phase 3.** Adopt `oapi-codegen`; generate types and server
   interface; retrofit handlers; remove `swag`.
7. **Privacy + DPA publication.** When counsel removes the DRAFT
   banners on `PRIVACY.md` and `DPA.md`, link them under `/trust/`.
8. **SDK generation.** Once Phase 2 stabilises the spec, publish a
   canonical Go and TypeScript client.

## Acceptance criteria

The docs site is considered shipped when **all** of these are true:

1. `docs.raven.ravencloak.org` resolves with valid HTTPS and serves the
   VitePress-built homepage.
2. Every section in the IA tree above renders at least one page (use a
   stub page where source content does not yet exist; mark stubs with a
   visible banner so contributors know to fill them in).
3. `/api/` lists every operation present in `contracts/openapi.yaml`,
   each at its own URL, indexable by PageFind.
4. PageFind search returns results for queries against operation names,
   markdown headings, and prose body text.
5. `gh workflow view docs` shows the workflow has run green at least
   once on `main` after the initial bootstrap commit.
6. A PR opened against the docs site triggers a Cloudflare Pages
   preview deploy and posts the preview URL as a sticky comment.
7. Spectral lint on `contracts/openapi.yaml` returns zero errors.
8. Lighthouse run on `/` and a representative content page reports:
   Performance ≥ 95, Accessibility ≥ 95, Best Practices ≥ 90, SEO ≥ 95.
9. The `Edit this page` link on a representative content page resolves
   to the correct source file in `docs/` on GitHub `main`.
10. README.md gains a "Documentation" section linking to
    `docs.raven.ravencloak.org`.

## Risks and mitigations

| Risk | Mitigation |
| ---- | ---------- |
| `wrangler pages deploy` flakes on a transient CF outage | Retry the deploy step up to 2 times before failing; CI does not block other PRs |
| OpenAPI spec is incomplete and `/api/` looks empty | Beta banner; "Coming soon" text on tag groups not yet covered |
| sync-content.ts mapping drifts from reality | `content-map.json` is the single source; CI fails if a mapped source file is missing |
| PageFind index grows past 200 KB | Re-evaluate Algolia DocSearch only when the corpus exceeds ~500 pages |
| Custom domain DNS misconfigured | One-time bootstrap is documented above; CF dashboard validates the CNAME during setup |
| LLM token cost from a future embedded widget | Tracked under Sub-project 2 (rate limit, spend cap, prefilled questions in Phase 1) |
