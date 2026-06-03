# 0003 — Always-fresh publish lifecycle for MVP

Status: accepted

## Decision

A Published KB is **always-fresh**: the Marketplace listing and the Import operation both read from the live KB state at the moment of request. There are no immutable publication snapshots, no version history, and no explicit "Publish update" action separate from "Publish."

The contract Raven offers Importers is timestamp-based, not version-based:

- The Marketplace listing displays `last_modified_at` ("Updated 3 days ago").
- An Imported KB stamps `imported_from_revision_at` so the Importer can see exactly which moment they captured.
- **Re-import** is a destructive overwrite of the Importer's local KB, with confirmation, available from the Importer's local KB detail page.

## Why

Two publisher personas, one tier today:

1. **Free-tier auto-publisher** (the MVP majority): uses Raven as their primary chatbot, KB is public because the plan requires it. Wants zero friction — upload doc, it's discoverable. Versioning UI is overhead they didn't ask for.
2. **OSS curator** (deferred persona): wants stable releases with version pinning so importers can rely on a known revision. Real need — but not represented in the MVP user base.

Always-fresh serves persona 1 at zero cost, and persona 2 is protected from silent breakage by the fork-on-import contract from [ADR-0001](0001-marketplace-fork-on-import.md) — their existing imports never change. Only *future* importers see the publisher's edits, and they see a timestamp telling them so.

A versioned model (`marketplace_publications` table, "Publish update" button, v1/v2/v3 UI) was considered and rejected for MVP. It can be added later as an additive migration when a real curator persona materialises and asks for it.

## Trade-offs accepted

- **No version pinning for Importers in MVP.** The "what did I get?" question is answered by `imported_from_revision_at`, not a version number. If the publisher made breaking changes between two import events, Importers must reconcile manually.
- **No rollback for publishers.** If a publisher accidentally deletes content from their KB, an Importer arriving in that window gets the broken state. The Importer's existing fork is fine (per ADR-0001); only new imports during the bad window are affected. Mitigated by standard backup/restore on the publisher's own KB, not a Marketplace feature.
- **OSS curator persona is deferred.** When a serious open-knowledge publisher signs up and asks for versioned releases, we add the `marketplace_publications` table, backfill all existing public KBs as v1, and surface a "Publish update" action. The migration is additive; no Importer breaks.

## Consequences

- `knowledge_bases` gains a `last_modified_at TIMESTAMPTZ` column (updated by trigger on any Document/Source/Chunk write that affects the KB's content surface). Distinct from `updated_at`, which already exists and tracks any row mutation.
- Importer-side `knowledge_bases` row carries `imported_from_revision_at TIMESTAMPTZ` set at Import time.
- No `marketplace_publications` table is added in MVP. When added later it will join to `knowledge_bases` via `(org_id, kb_id, version_seq)` and Import will start reading from publication rows instead of live KB state — Importer schema does not need to change.
