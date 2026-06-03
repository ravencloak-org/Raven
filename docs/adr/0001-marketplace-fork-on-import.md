# 0001 — Marketplace KBs use fork-on-import

Status: accepted

## Decision

When an Importer Org imports a Public KB from the Marketplace, Raven **copies the KB row, its Sources, Chunks, and Embeddings into the Importer Org**. The Imported KB is an independent fork — it does not track publisher edits, and unpublishing or deleting the original has no effect on the Importer. Chats and Widgets created by the Importer FK to their local Imported KB, never to the publisher's KB.

## Why

Three rejected alternatives drove this:

1. **Cross-tenant read (live link).** Importer's KB is a thin pointer at the publisher's chunks. Lower storage, automatic updates. Rejected: forces an RLS hole in `knowledge_bases`, `chunks`, and `embeddings` for every retrieval-path query (a hostile coupling — easy to leak), and silently changes Importers' chatbot answers when publishers edit. A publisher takedown breaks every Importer's product. The "Marketplace" stops being a marketplace and becomes a shared dependency graph.

2. **Separate `marketplace_kbs` table.** Two domain concepts — a regular KB, and a marketplace KB you fork from. Rejected: doubles the storage path (writers must publish to both tables; retrieval code branches), and duplicates the KB concept in the domain language. A `visibility` column on `knowledge_bases` does the same work without forking the type.

3. **Fork-on-import (chosen).** Importer's Org gets a real copy. RLS stays intact for all hot-path queries; the only cross-tenant read is the Marketplace listing query, which runs through a `SECURITY DEFINER` function over public-KB metadata only.

## Trade-offs accepted

- **Storage cost.** Each import re-stores Chunks and Embeddings in the Importer Org. With `vector(768)` and typical KB sizes this is tens of MB per import — manageable. A future optimisation can dedupe Embeddings by `(chunk_hash, embedding_model)` across orgs using a shared content-addressed pool, without breaking the fork contract.
- **No auto-sync.** Importers stay on the version they imported. Pulling a new version is an explicit user action ("import again to get updates"). This is the package-manager model — predictable, but some users will expect Substack-style auto-update and need to be told otherwise.
- **Re-embedding cost on import.** Avoided if and only if the Importer Org uses the same embedding model as the Publisher; otherwise Chunks must be re-embedded against the Importer's model. Acceptable: the alternative is non-portable embeddings.

## Consequences

- `knowledge_bases` gains a `visibility ENUM('private','public')` column and a `source_public_kb_id UUID NULL` lineage pointer.
- Marketplace listing is a single `SECURITY DEFINER` function returning public-KB metadata (`id`, `org_id`, `name`, `description`, counts) across all Orgs. No cross-tenant read of Chunks or Embeddings is ever needed at query time.
- Unpublish is non-destructive to Importers. The original KB flips back to `private`; existing forks continue to work.
- Quota accounting must treat Imported KBs as owned by the Importer Org (they consume Importer storage and embedding budget), not by the Publisher.
