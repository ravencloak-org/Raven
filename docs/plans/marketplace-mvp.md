# Marketplace MVP — implementation plan

Tracking doc for the walled Marketplace feature. Source decisions: [ADR-0001](../adr/0001-marketplace-fork-on-import.md) … [ADR-0007](../adr/0007-marketplace-lifecycle-behaviours.md). Branch: `feat/marketplace-mvp`.

## 1. Goal

The Marketplace is the cross-tenant discovery surface that lists Public KBs available for Import. It is hosted at `raven.ravencloak.org/marketplace`, accessible only to authenticated Users, and serves as the flywheel that turns Free Plan Orgs into a content supply for Importers without leaking private content from Paid Plan Orgs. Every Marketplace operation reads live KB state (no snapshots); every Import is a content-grade fork; every Public KB carries an SPDX license badge.

## 2. Schema delta

Seven sequential, independently reversible Goose migrations starting at `00047`.

### `00047_kb_marketplace_columns.sql`

Extend `knowledge_bases` with publish-state columns.

- `visibility kb_visibility NOT NULL DEFAULT 'private'` — new enum `kb_visibility AS ENUM ('private','public')`.
- `published_at TIMESTAMPTZ NULL`.
- `published_by_user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL`.
- `source_public_kb_id UUID NULL REFERENCES knowledge_bases(id) ON DELETE SET NULL` — Importer-side lineage pointer.
- `imported_from_revision_at TIMESTAMPTZ NULL` — set at Import time, updated by Re-import.
- `last_modified_at TIMESTAMPTZ NOT NULL DEFAULT now()` — distinct from `updated_at`.
- Trigger `trg_kb_last_modified_on_content` on `INSERT/UPDATE/DELETE` of `documents`, `sources`, `chunks` — bumps the parent KB's `last_modified_at`.
- Partial unique index on `(org_id, slug)` where `visibility='public'` to allow Marketplace URL routing by `(org_slug, kb_slug)` once `org_slug` lands in 00048.
- Indexes: `idx_kb_visibility_public` (partial, `WHERE visibility='public'`), `idx_kb_source_public_kb_id`.

### `00048_org_slug.sql`

Promote Org slug to a first-class column.

- `organizations.slug VARCHAR(64) NOT NULL` with `UNIQUE` constraint and CHECK `slug ~ '^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$'`.
- Backfill in the same migration: `slug = slugify(display_name)` with `-2`, `-3`, … collision suffixes.
- `org_slug_holds (slug VARCHAR(64) PRIMARY KEY, org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, held_until TIMESTAMPTZ NOT NULL)` — 30-day soft-redirect on rename.

### `00049_kb_status_read_only_private.sql`

Extend the existing `kb_status` enum.

- `ALTER TYPE kb_status ADD VALUE 'read_only_private';` (per ADR-0004).
- Reverse migration: documented as no-op — Postgres cannot remove enum values; rollback is by reverting application reads.

### `00050_kb_license_and_strikes.sql`

License and moderation columns (ADR-0006).

- `knowledge_bases.license_spdx_id TEXT NULL`.
- Partial CHECK constraint: `CHECK (visibility = 'private' OR license_spdx_id IS NOT NULL)` so Public KBs must carry a license.
- `organizations.takedown_strikes INTEGER NOT NULL DEFAULT 0`.

### `00051_kb_slug_holds.sql`

90-day slug holds on unpublish (ADR-0007).

- `kb_slug_holds (org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, slug VARCHAR(100) NOT NULL, held_until TIMESTAMPTZ NOT NULL, PRIMARY KEY (org_id, slug))`.
- Trigger `trg_kb_release_slug_on_unpublish` on `knowledge_bases` `AFTER UPDATE` when `visibility` flips `public→private` — inserts `(org_id, slug, now() + interval '90 days')` ON CONFLICT update `held_until`.
- Publish handler reads this table to block re-use during the hold window.

### `00052_marketplace_functions.sql`

The two cross-tenant read functions.

- `marketplace_list_public_kbs() RETURNS TABLE (...) LANGUAGE sql SECURITY DEFINER STABLE` — returns the row shape declared in ADR-0005 plus `license_spdx_id`. Filters `visibility='public'` and joins to `organizations` for slugs and display names.
- `marketplace_preview_kb(public_kb_id UUID) RETURNS TABLE (chunk_id UUID, ordinal INT, text TEXT) LANGUAGE plpgsql SECURITY DEFINER` — raises `insufficient_privilege` if the target is not `visibility='public'`; otherwise returns at most 3 chunks ordered by `ordinal`.
- `REVOKE ALL ... FROM PUBLIC` + `GRANT EXECUTE ... TO raven_app` on both; ownership set to `raven_admin`.

### `00053_marketplace_moderation.sql`

Report and takedown queues (ADR-0006).

- `marketplace_reports (id UUID PK, reported_kb_id UUID REFERENCES knowledge_bases(id) ON DELETE CASCADE, reporter_user_id UUID REFERENCES users(id) ON DELETE SET NULL, reason TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','reviewing','resolved','dismissed')), created_at TIMESTAMPTZ NOT NULL DEFAULT now())`.
- `marketplace_takedowns (id UUID PK, target_kb_id UUID REFERENCES knowledge_bases(id) ON DELETE CASCADE, source TEXT NOT NULL CHECK (source IN ('publisher','admin','dmca')), notes TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT now())`.
- Indexes: `idx_marketplace_reports_status`, `idx_marketplace_takedowns_target_kb_id`.
- RLS: `marketplace_reports` exposes own-reports only to the reporter; `marketplace_takedowns` admin-only.

## 3. Backend services

New Go packages under `internal/`.

| Path | Purpose |
|------|---------|
| `internal/marketplace/license.go` | SPDX allow-list const slice (7 entries per ADR-0006) and validation helper. |
| `internal/marketplace/projection.go` | `KB → PublishedKBProjection` projector. Implements the allow-list scrub of `settings`. Single source of truth for what crosses the publish boundary (ADR-0002). |
| `internal/marketplace/projection_settings_allowlist.go` | The settings allow-list constant; every key is either listed here or in the explicit-deny slice. |
| `internal/marketplace/publish.go` | Service: publish, unpublish, slug-hold registration. Transactional. Validates license, plan rules, slug-hold window. |
| `internal/marketplace/import.go` | Service: Import + Re-import. Wraps projection apply, sets `imported_from_revision_at`, applies Free Plan public-only override (ADR-0004). |
| `internal/marketplace/preview.go` | Wraps the `marketplace_preview_kb` SQL function; 3-chunk cap enforced at the DB layer, defence-in-depth check here. |
| `internal/marketplace/listing.go` | Wraps `marketplace_list_public_kbs` and applies any per-User filtering (e.g. hide reported KBs). |
| `internal/marketplace/moderation.go` | Report submission, admin review queue ops, takedown writer, strike incrementer. |
| `internal/marketplace/org_slug.go` | Slug generation + collision-suffix logic; soft-redirect lookup for `org_slug_holds`. |
| `internal/api/handlers/marketplace_handler.go` | HTTP handlers for every endpoint in §4. |
| `internal/api/handlers/kb_handler.go` (extend) | Add `publish`, `unpublish`, `re-import` handlers; enforce Free Plan public-only override on `POST /knowledge_bases`. |
| `internal/jobs/marketplace_takedown_notifier.go` | Asynq task that emails owners of Derivative Public KBs when their root is taken down (ADR-0006). |
| `internal/jobs/marketplace_slug_hold_sweeper.go` | Asynq cron task that deletes rows from `kb_slug_holds` and `org_slug_holds` where `held_until < now()`. Daily. |
| `internal/billing/downgrade_freeze.go` (extend) | On Hyperswitch webhook → Free Plan transition, run the `UPDATE knowledge_bases SET status='read_only_private' …` from ADR-0004 inside the existing downgrade transaction. |

### CI test (per ADR-0002)

- `internal/marketplace/projection_test.go` includes `TestSettingsAllowListExhaustive`: enumerates every key seen in `knowledge_bases.settings` across the live test schema and asserts each is in either the allow-list or the explicit-deny list. Test fails the build when a new setting key is added without a deliberate decision.

## 4. API endpoints

All endpoints require an authenticated session (`Authorization: Bearer …` via SuperTokens). `org_id` comes from session, never the URL.

| Method | Path | Auth | Request | Response | Errors |
|--------|------|------|---------|----------|--------|
| `GET` | `/api/v1/marketplace` | User session | `?q=&license=&limit=&cursor=` | `{ items: [{kb_id, org_slug, org_display_name, kb_slug, kb_name, description, license_spdx_id, last_modified_at, source_public_kb_id, source_org_slug, source_org_display_name}], next_cursor }` | 401 unauth |
| `GET` | `/api/v1/marketplace/{org_slug}/{kb_slug}` | User session | — | Full Public KB detail row + counts (sources, documents, chunks) | 401, 404 (unknown slugs), 410 (in `kb_slug_holds`) |
| `GET` | `/api/v1/marketplace/{org_slug}/{kb_slug}/preview` | User session | — | `{ chunks: [{chunk_id, ordinal, text}] }` (≤3) | 401, 403 (target private), 404, 410 |
| `POST` | `/api/v1/marketplace/import/{public_kb_id}` | User session + `kb:create` | `{ workspace_id: UUID }` | `{ kb_id, imported_from_revision_at }` | 401, 403 (not member of target workspace; or KB not public; or plan-rule rejection) |
| `POST` | `/api/v1/knowledge_bases/{id}/publish` | User session + `kb:publish` | `{ license_spdx_id }` | `{ visibility: 'public', published_at, marketplace_url }` | 401, 403, 404, 409 (slug held by `kb_slug_holds`), 422 (license not in allow-list; KB has no documents) |
| `POST` | `/api/v1/knowledge_bases/{id}/unpublish` | User session + `kb:publish` | — | `{ visibility: 'private', slug_held_until }` | 401, 403, 404 |
| `POST` | `/api/v1/knowledge_bases/{id}/re-import` | User session + `kb:write` | — | `{ kb_id, imported_from_revision_at }` | 401, 403, 404, 409 (KB is not an import — `source_public_kb_id IS NULL`), 410 (source KB unpublished) |
| `POST` | `/api/v1/marketplace/reports` | User session | `{ kb_id, reason }` | `{ report_id, status: 'open' }` | 401, 404, 429 (per-User rate limit) |
| `GET` | `/api/v1/admin/marketplace/reports` | Admin | `?status=open&limit=&cursor=` | Paginated reports | 401, 403 |
| `POST` | `/api/v1/admin/marketplace/reports/{id}/approve` | Admin | — | `{ takedown_id, target_kb_id, strikes_after }` — unpublishes target KB, increments publisher Org `takedown_strikes`, emails publisher | 401, 403, 404 |
| `POST` | `/api/v1/admin/marketplace/reports/{id}/dismiss` | Admin | — | `{ report_id, status: 'dismissed' }` | 401, 403, 404 |
| `POST` | `/api/v1/admin/marketplace/dmca` | Admin | `{ target_kb_id, notice_text, claimant_email }` | `{ takedown_id, counter_notice_window_ends }` — KB transitions to `dmca_pending`, 14-day counter-notice clock starts | 401, 403, 404 |

## 5. Frontend pages (Vue.js)

New routes under `frontend/src/views/marketplace/`. KB detail page extended in `frontend/src/views/knowledge_bases/`.

| Route | Component | Purpose |
|-------|-----------|---------|
| `/marketplace` | `MarketplaceListView.vue` | Search, filter by SPDX license, infinite-scroll grid of cards. Each card: KB name, Org display_name + slug link, license badge, "Updated N days ago", import count, Report button. |
| `/marketplace/:orgSlug/:kbSlug` | `MarketplaceKbDetailView.vue` | Public KB detail. Description, license badge, source/document/chunk counts, "Forked from {parent Org}" line (one hop). Buttons: Preview, Import, Report. Handles 410 with a "This KB was unpublished" state. |
| `/marketplace/:orgSlug/:kbSlug/preview` (modal, not full route) | `MarketplacePreviewDialog.vue` | Renders the ≤3 chunks returned by the preview endpoint, with a CTA to Import. |
| `/marketplace/import/:publicKbId` (modal) | `MarketplaceImportDialog.vue` | Pick destination workspace; on confirm calls Import endpoint; redirects to the new local KB. |
| Existing KB-detail page extensions | `KnowledgeBaseDetailView.vue` | Add: Publish/Unpublish toggle (with license picker on first publish), Re-import button (only when `source_public_kb_id IS NOT NULL`), license badge, lineage link to parent Public KB, frozen-state banner when `status='read_only_private'`. |
| `/admin/marketplace/reports` | `AdminReportsView.vue` | Admin-only review queue: list reports, mark reviewing/resolved/dismissed, escalate to takedown. |
| `/admin/marketplace/takedowns` | `AdminTakedownsView.vue` | Admin-only: takedown audit log + strike counter per Org. |

Playwright coverage: publish → list → preview → import → re-import → unpublish; Free Plan create-KB forces public; downgrade freezes private KBs.

## 6. Operational requirements

- **DMCA inbox.** Provision `dmca@ravencloak.org` (Google Workspace alias → admin distribution list). Publish the designated agent details in `docs/legal/dmca.md` and at `raven.ravencloak.org/legal/dmca`. Must be in place before the Marketplace ships publicly.
- **Admin takedown runbook.** Document in `docs/runbooks/marketplace-takedown.md`: triage SLA (48h initial response, 7d resolution), workflow for `marketplace_reports` → `marketplace_takedowns`, derivative-owner email template, strike increment procedure, and Org suspension steps when `takedown_strikes >= 3`.
- **Repeat-infringer workflow.** Manual for MVP: admin reviews the 3rd strike, runs `supertokens-admin` flow to suspend the Org's signin, sets all of the Org's `visibility='public'` KBs to `private`, and emails the Org. No automatic suspension trigger in MVP.
- **Observability.** OpenObserve dashboards for: publish rate, import rate, preview-to-import conversion, report submission rate, takedown response time. Beszel covers host metrics; no Marketplace-specific host work.
- **Slug-hold sweepers.** Asynq cron tasks (§3) must be registered in `internal/jobs/scheduler.go` with the existing daily cadence.

## 7. Deferred (explicitly NOT in MVP)

- **Versioned publication snapshots** — deferred per [ADR-0003](../adr/0003-always-fresh-publish-lifecycle.md). No `marketplace_publications` table, no v1/v2/v3 UI, no "Publish update" button. Re-import reads live KB state.
- **Original file blob inclusion** — deferred per [ADR-0002](../adr/0002-content-grade-publish-boundary.md). Importers never receive SeaweedFS blobs. No "include blobs" toggle on Publish.
- **Proactive moderation** — deferred per [ADR-0006](../adr/0006-licence-and-moderation.md). No ML-based content scanning at publish time; reactive Report-button flow only.
- **User credit / Maintainer field** — deferred per [ADR-0005](../adr/0005-org-as-marketplace-publisher.md). No `published_by_user_display_name`, no per-publisher User avatar on Marketplace cards. Org is the only public attribution.
- **License compatibility enforcement** — deferred per [ADR-0006](../adr/0006-licence-and-moderation.md). Cross-fork SPDX compatibility (e.g. CC-BY-SA → CC-BY-NC mismatch) is documented but not enforced; users reason about it from badges.
- **Per-plan quotas on publish / import / storage.** Free Plan public-only and read-only-private freeze land in MVP; numeric per-plan quotas (max public KBs, max imports per day, storage caps) are deferred to a follow-up milestone.
- **Search ranking & categories.** Marketplace listing is timestamp-ordered (`last_modified_at DESC`) plus free-text search on `name` and `description`. Faceted browse, semantic search over Marketplace, category taxonomy: deferred.
- **Marketplace-specific embedding dedupe.** ADR-0001 mentions a future `(chunk_hash, embedding_model)` shared pool. Not in MVP.
- **Anonymous Marketplace browsing.** Login-required for all routes; SEO surface is deferred.

## 8. Milestones

Three squash-mergeable PRs onto `feat/marketplace-mvp`.

### M1 — schema + Org slug + publish lifecycle

- Migrations `00047`–`00051`.
- `internal/marketplace/license.go`, `projection.go`, `projection_settings_allowlist.go`, `publish.go`, `org_slug.go`.
- Endpoints: `POST /knowledge_bases/{id}/publish`, `POST /knowledge_bases/{id}/unpublish`.
- Downgrade hook in `internal/billing/downgrade_freeze.go`.
- Free Plan public-only override in `kb_handler.go`.
- Slug-hold sweeper job.
- Vue: KB detail Publish/Unpublish + license picker; frozen-state banner; license badge.
- Tests: projection allow-list CI test, Playwright publish/unpublish, downgrade freeze.

### M2 — import + re-import + preview + Marketplace listing

- Migration `00052` (functions).
- `internal/marketplace/import.go`, `preview.go`, `listing.go`.
- Endpoints: `GET /marketplace`, `GET /marketplace/{org_slug}/{kb_slug}`, `GET …/preview`, `POST /marketplace/import/{public_kb_id}`, `POST /knowledge_bases/{id}/re-import`.
- HTTP handler 410-Gone path against `kb_slug_holds`.
- Vue: `MarketplaceListView`, `MarketplaceKbDetailView`, `MarketplacePreviewDialog`, `MarketplaceImportDialog`, Re-import button on imported KBs.
- Playwright: list → preview → import → re-import → unpublish 410 round-trip.

### M3 — license, reports, DMCA, admin queue

- Migration `00053`.
- `internal/marketplace/moderation.go`, takedown-notifier Asynq job.
- Endpoints: `POST /marketplace/reports`, admin reports + takedowns endpoints.
- Vue: `AdminReportsView`, `AdminTakedownsView`, Report button on Marketplace card + KB detail.
- Ops: DMCA inbox provisioned, `docs/legal/dmca.md`, `docs/runbooks/marketplace-takedown.md`.
- OpenObserve dashboards.
- Playwright: report submission rate-limit, admin takedown flow, derivative-owner notification email asserted via Mailhog fixture.
