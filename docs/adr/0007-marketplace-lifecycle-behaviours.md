# 0007 — Marketplace lifecycle behaviours

Status: accepted

## Decision

The Marketplace lifecycle is governed by five rules covering preview, re-import, unpublish, edit propagation, and Org slug migration.

**7a Try-before-fork is not offered.** Logged-in Users cannot run a chat against a Public KB without Importing it first. Chat always executes against a workspace-local KB so that retrieval-path RLS stays single-tenant. A narrow preview surface — `GET /api/v1/marketplace/{org_slug}/{kb_slug}/preview` — returns at most 3 sample Chunks for content vetting. The endpoint is backed by a `SECURITY DEFINER` function `marketplace_preview_kb(public_kb_id UUID)` that is the only sanctioned cross-tenant read of Chunk rows in the system.

**7b Re-import is a destructive overwrite, scoped to content.** A Re-import drops every Source, Document, Chunk, and Embedding row attached to the Importer's local KB and replaces them with the publisher's current projection (per [ADR-0002](0002-content-grade-publish-boundary.md)). The Importer's local `knowledge_bases.id` is preserved, and along with it: Chat sessions, Widgets, API keys, Routing rules, Webhook configs, Response cache — every artefact that FKs to the KB id, not its content. On success the Importer's `imported_from_revision_at` is updated to the publisher's current `last_modified_at`. UI requires explicit confirmation.

**7c Unpublish flips visibility and holds the slug for 90 days.** When a Publisher unpublishes, `visibility` is set back to `private` and the KB stays in the Publisher's Workspace untouched. Already-Imported forks are unaffected (per [ADR-0001](0001-marketplace-fork-on-import.md)). The Marketplace URL `/marketplace/{org_slug}/{kb_slug}` returns **HTTP 410 Gone**, not 404, so Importers and link-checkers can distinguish "removed" from "never existed." The `(org_id, kb_slug)` pair is registered in `kb_slug_holds` with `held_until = now() + interval '90 days'`. During the hold window the same Org cannot publish a new KB under that slug.

**7d Edits after publish need no new mechanism.** Already covered by [ADR-0003](0003-always-fresh-publish-lifecycle.md): `last_modified_at` is updated by trigger on any Document, Source, or Chunk write, and the Marketplace card renders it as "Updated N days ago." No publish-update action, no version row, no notification fan-out.

**7e Org slug becomes a first-class column.** `organizations.slug VARCHAR(64) UNIQUE NOT NULL` is added. Backfill: `slugify(display_name)` with `-2`, `-3`, … collision suffixes. Org owners can rename their slug post-launch; the previous slug is held in `org_slug_holds` for 30 days as a soft-redirect so Marketplace URLs do not break. The existing `workspaces.slug` unique-within-org logic is untouched — workspace slugs live below the org axis and are not URL-public.

## Why

- **Cross-tenant Chunk reads at chat time would punch the RLS hole [ADR-0001](0001-marketplace-fork-on-import.md) explicitly avoided.** Try-before-fork sounds friendly but turns every retrieval query into "either my Org or a public KB" — a coupling we refused once and decline to re-introduce. A 3-chunk preview, behind a single `SECURITY DEFINER` function with no Embedding or Source-blob exposure, is the bounded escape valve.
- **Re-import has to be destructive to be honest.** A merge model ("keep my edits, add their new docs") would be a lie — Importers don't edit Chunks, the publisher's projection is canonical. Destructive-with-confirm is the only model that matches what the user is actually doing.
- **410 Gone is the correct semantic for unpublished URLs.** 404 ("never existed") would mislead Importers whose Chat panels link back to the publisher; 410 is the documented "this was here, it isn't anymore" signal.
- **90-day slug hold blocks the squat-and-replace attack.** Without it, a Publisher could delete and re-create `acme/legal-handbook` with adversarial content, riding the old URL's trust and inbound links. 90 days is long enough that re-creation requires a deliberate wait; short enough that an honest Org wanting to reuse a slug is only mildly inconvenienced.
- **Org slug needs to be public-URL-safe and globally unique** per [ADR-0005](0005-org-as-marketplace-publisher.md). `display_name` is neither. The 30-day soft-redirect on rename preserves Importer links across re-branding.

## Trade-offs accepted

- **Preview is the one cross-tenant Chunk read.** `marketplace_preview_kb` is a `SECURITY DEFINER` function — a deliberate, audited hole, capped at 3 chunks per call, callable only by an authenticated User against a KB whose `visibility='public'`. Bounded blast radius, single review surface.
- **90-day slug hold creates friction for legitimate delete-and-recreate.** An Org that wants to retire `acme/legal-handbook` and ship a new one under the same slug must wait. Acceptable: famous-slug squatting is the worse failure.
- **No try-before-buy UX as good as competitors'.** Other RAG marketplaces offer "ask a sample question" against the live publisher KB. Raven offers chunk previews instead. Friction cost is real but bounded; the alternative is a permanent cross-tenant retrieval coupling.
- **Re-import wipes Importer-side annotations on Source/Document rows.** None exist today. If we add Importer-private metadata on Sources later, Re-import must explicitly carry it across the wipe-and-replace step.

## Consequences

- New endpoints:
  - `POST /api/v1/knowledge_bases/{id}/publish`
  - `POST /api/v1/knowledge_bases/{id}/unpublish`
  - `POST /api/v1/knowledge_bases/{id}/re-import`
  - `POST /api/v1/marketplace/import/{public_kb_id}`
  - `GET /api/v1/marketplace`
  - `GET /api/v1/marketplace/{org_slug}/{kb_slug}`
  - `GET /api/v1/marketplace/{org_slug}/{kb_slug}/preview`
- New SQL functions (`SECURITY DEFINER`):
  - `marketplace_list_public_kbs()` returns `(kb_id, org_slug, org_display_name, kb_slug, kb_name, description, license_spdx_id, last_modified_at, source_public_kb_id, source_org_slug, source_org_display_name)`.
  - `marketplace_preview_kb(public_kb_id UUID)` returns up to 3 Chunk rows (`chunk_id`, `text`, `ordinal`) for a `visibility='public'` KB; raises otherwise.
- New tables:
  - `kb_slug_holds (org_id UUID, slug VARCHAR(100), held_until TIMESTAMPTZ, PRIMARY KEY (org_id, slug))`.
  - `org_slug_holds (slug VARCHAR(64) PRIMARY KEY, org_id UUID, held_until TIMESTAMPTZ)`.
- HTTP handler for the public Marketplace URL returns `410 Gone` for any `(org_slug, kb_slug)` present in `kb_slug_holds` whose `held_until > now()`, `404` otherwise.
- The Import handler stamps `imported_from_revision_at = source_kb.last_modified_at` on the new row; the Re-import handler updates the same column on the existing row.
