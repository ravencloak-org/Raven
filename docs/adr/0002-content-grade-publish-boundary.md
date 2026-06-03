# 0002 — Content-grade publish boundary

Status: accepted

## Decision

When a KB is Published to the Marketplace (and on every subsequent Import), Raven copies a **content-grade** projection of the KB:

- **Crosses the boundary**: KB metadata (`name`, `description`, `slug`), Source rows (without file-blob references), Documents, Chunks, Embeddings (if model matches; otherwise re-embed from Chunks), an allow-listed subset of `settings` (`chunker_config`, `embedding_model_id`, and explicitly-marked public keys), and publish metadata (`published_by_user_id`, `published_at`, license declaration).
- **Never crosses the boundary**: original file blobs in SeaweedFS, `api_keys`, `chat_sessions`, `routing_rules`, `webhook_configs`, `response_cache`, `airbyte_connectors`, and any `settings` key not on the allow-list.

The default for the `settings` JSONB scrub is **deny**: a key is copied only if it appears in an explicit allow-list maintained alongside the Publish code.

## Why

Three alternatives were considered:

1. **Wide (source-grade)** — copy original file blobs to importers as well. Rejected: turns Raven into a piracy mirror for any copyrighted PDF a user uploads, complicates takedown response, and bloats Importer storage with files they will almost never re-read (chunks are what powers RAG).
2. **Narrow (index-only)** — copy only KB metadata; Importer re-runs the chunker and embedder from scratch. Rejected: makes Import slow (minutes of worker time) and forces a Python AI-worker round-trip for every Import. The "package-manager UX" the marketplace promises is broken if `npm install` always recompiles.
3. **Medium (content-grade, chosen)** — copy the *derived* text and vectors but not the source artefact or any runtime state.

## Trade-offs accepted

- **Importer cannot see original files.** They get the chunks (the text used for retrieval) but not the PDF/Word/etc. that produced them. UX cost is low: chunks are what RAG uses, and the source file is rarely consulted by an Importer.
- **`settings` allow-list is a maintenance burden.** Every new setting requires a deliberate decision about whether it's public-safe. Mitigation: a CI test that asserts every key in the schema is either explicitly allow-listed or explicitly denied — no implicit silence.
- **No "include blobs" escape hatch in MVP.** Some legitimate use cases (open citation archives, public-domain primary sources) would benefit from blob inclusion. Out of scope until a real publisher asks; default-deny is safer.

## Consequences

- The Publish code path becomes a projection function: `KB → PublishedKBProjection`. Not a `pg_dump`-style copy.
- The Import code path is the same projection applied in reverse: `PublishedKBProjection → new KB in Importer Org`.
- `airbyte_connectors`, `api_keys`, `routing_rules`, `webhook_configs`, `response_cache`, `chat_sessions` are **publisher-private by construction** — there is no path by which they reach an Importer. This is a structural guarantee, not a runtime check.
- Schema needs: `knowledge_bases.visibility ENUM('private','public')`, `knowledge_bases.published_at TIMESTAMPTZ NULL`, `knowledge_bases.published_by_user_id UUID NULL`, `knowledge_bases.license TEXT NULL` (SPDX id), and `knowledge_bases.source_public_kb_id UUID NULL` for Importer-side lineage.
