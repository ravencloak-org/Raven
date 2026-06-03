# Raven

Multi-tenant RAG platform: orgs upload documents into knowledge bases, query them via chat or widget, and embed grounded answers into third-party apps.

## Language

**Organization** (Org):
The top-level tenant. Owns billing, subscription, members, and all data created beneath it. Identified by `org_id` in RLS and throughout the codebase.
_Avoid_: Account, tenant, customer.

**Workspace**:
A sub-grouping inside an Org that owns one or more Knowledge Bases. Used to scope membership, API keys, and (eventually) per-team plan limits.
_Avoid_: Project, team.

**Knowledge Base** (KB):
A collection of Sources, Documents, Chunks, and their Embeddings against which Chats and Widgets are run. Belongs to exactly one Workspace.
_Avoid_: Corpus, dataset, library.

**Source**:
An input to a KB — uploaded file, URL, connector. Materializes into one or more Documents on processing.

**Chunk**:
The atomic embeddable unit of text extracted from a Document. Sized by the chunker to fit the embedding model's token window.

**Embedding**:
A vector for a Chunk produced by an embedding model. Currently `vector(768)` from `nomic-embed-text`.

**Subscription**:
The billing relationship between an Org and a paid plan. One active subscription per Org. The **Free Plan** is the implicit state when no active paid subscription exists.

**Free Plan Org**:
An Org without an active paid Subscription. The phrase "free user" is shorthand for "a User belonging to a Free Plan Org" — the plan is a property of the Org, never the User. Free Plan Orgs are governed by the strict-flywheel rule in [ADR-0004](docs/adr/0004-free-tier-public-only-rule.md): every KB they hold — whether created or imported — is forced `visibility=public`.
_Avoid_: Free user, free account (when used to imply a per-user concept).

**Read-only-private KB**:
A KB in the transitional state produced when a Paid Plan Org downgrades to Free Plan. Stays private but cannot accept new Sources, Documents, or Chats until the User Publishes it, Deletes it, or upgrades back. Represented by `kb_status = 'read_only_private'`.

**Derivative Public KB**:
A Public KB whose `source_public_kb_id` points at another Public KB — i.e. an Imported KB that was itself made public (either because the Importer chose to publish their fork, or because the Importer is a Free Plan Org and the strict-flywheel rule forced publication). Lineage is preserved indefinitely; takedowns of the original do not cascade.

### Marketplace

**Marketplace**:
The cross-tenant discovery surface that lists Public KBs available for Import. Hosted at `raven.ravencloak.org/marketplace`. Accessible only to authenticated Users; no anonymous browsing.

**Content-grade Publish**:
The data boundary applied when a KB is Published or Imported. Chunks, Embeddings, and Source metadata cross the boundary; original file blobs, runtime artefacts (api_keys, chats, routing_rules, webhooks, response_cache, connectors), and any non-allow-listed `settings` keys do not. See [ADR-0002](docs/adr/0002-content-grade-publish-boundary.md).

**Public KB**:
A KB whose publishing Org has flipped its `visibility` to `public`, making it discoverable on the Marketplace. The KB itself still lives in its publishing Workspace; visibility is the only thing that changes.

**Private KB**:
A KB visible only inside its owning Org. The default state for every KB.

**Publish**:
The act of flipping a KB from `private` to `public`. Reversible (the publisher can unpublish), but unpublish does not affect already-Imported copies. The publisher of record is the **Org**, not the User who pressed the button. See [ADR-0005](docs/adr/0005-org-as-marketplace-publisher.md).

**Org slug**:
A globally-unique, URL-safe identifier for an Org. Forms the first segment of every Marketplace URL: `raven.ravencloak.org/marketplace/{org_slug}/{kb_slug}`. Backfilled from `display_name` at migration; subsequently editable but constrained for uniqueness.

**Marketplace URL**:
The canonical public address of a Public KB: `raven.ravencloak.org/marketplace/{org_slug}/{kb_slug}`. 404s if either slug is unknown or the target is private.

**Import**:
The act of an Org cloning a Public KB into one of its own Workspaces. Produces an independent KB row with its own Sources, Chunks, and Embeddings — a fork, not a live link. See [ADR-0001](docs/adr/0001-marketplace-fork-on-import.md).

**Re-import**:
A destructive overwrite of an existing Imported KB with the publisher's current state. Replaces the Importer's local Chunks and Embeddings; preserves the Importer's KB id, Chats, Widgets, and other runtime artefacts (these FK to the Imported KB id, not its content). Always confirmed.

**`last_modified_at`**:
Timestamp on a Public KB reflecting the most recent content-affecting change in the publisher's workspace. Shown as "Updated 3 days ago" on Marketplace listings; the Marketplace and Import operations both read live KB state per [ADR-0003](docs/adr/0003-always-fresh-publish-lifecycle.md) — there is no separate publication snapshot in MVP.

**`imported_from_revision_at`**:
Timestamp stamped on the Importer's local KB at the moment of Import, recording the publisher's `last_modified_at` value captured. Lets the Importer reason about what they have vs what's now available upstream.

**Importer Org**:
An Org that has Imported one or more Public KBs. Distinct from the **Publishing Org** that owns the original.

**License (SPDX)**:
A required declaration on every Public KB, chosen from a 7-item allow-list (`CC0-1.0`, `CC-BY-4.0`, `CC-BY-SA-4.0`, `CC-BY-NC-4.0`, `MIT`, `Apache-2.0`, `GPL-3.0`). Surfaced as a badge on every Marketplace card and Importer-side KB. See [ADR-0006](docs/adr/0006-licence-and-moderation.md).

**Takedown**:
The forced or voluntary removal of a Public KB from the Marketplace. Sources: publisher (self-unpublish), DMCA (third-party copyright claim via `dmca@ravencloak.org`), or admin (TOS violation). Does not cascade to Derivative Public KBs; derivative owners are notified instead.

**Strike**:
A confirmed Takedown attributable to an Org. Three strikes triggers Org-level suspension via admin runbook. Tracked on `organizations.takedown_strikes`.

**Preview**:
A narrow public surface returning ≤3 sample Chunks from a Public KB, served by a `SECURITY DEFINER` Postgres function so logged-in Users can assess content quality before deciding to Import. The only place Marketplace traffic reads published Chunks cross-tenant — bounded by the function's signature, not a general RLS hole. See [ADR-0007](docs/adr/0007-marketplace-lifecycle-behaviours.md).

**Discovery**:
The browse / search surface at `raven.ravencloak.org/marketplace`. Postgres full-text search over KB metadata (name, description, publishing Org's display name), with sort options (Newest, Most imported, Recently updated, Alphabetic) and a license multi-select filter. No categories, tags, or content search in MVP. See [ADR-0008](docs/adr/0008-marketplace-discovery-and-operations.md).

## Relationships

- An **Org** has one active **Subscription** (or none, in which case it is a **Free Plan Org**).
- An **Org** has many **Workspaces**; each **Workspace** has many **KBs**.
- A **KB** has many **Sources**; each **Source** produces many **Documents**; each **Document** produces many **Chunks**; each **Chunk** has one **Embedding**.
- A **Public KB** can be **Imported** by many other **Orgs**, producing one **Imported KB** per Importer Org. Imported KBs do not track publisher edits — they are forks at the moment of import.
- **Chats** and **Widgets** always belong to the **Importer Org** and FK to the **Imported KB**, never to the publisher's KB.

## Example dialogue

> **Dev:** "If a user on the **Free Plan** uploads a doc, where does it land?"
> **Domain expert:** "Their **Org** is a Free Plan Org, so any **KB** they create has to be **Public**. The doc still uploads into that KB in their **Workspace** like normal — it's just discoverable from the **Marketplace**."

> **Dev:** "If the publisher edits the **KB** after I've **Imported** it, do I see the changes?"
> **Domain expert:** "No. **Import** is a fork. You'd have to re-import to get the new version."

## Flagged ambiguities

- "Account" was used to mean both **User** and **Org**. Resolved: **Org** is the billing and data-ownership unit; **User** is a person who is a member of one or more Orgs. "Free account" specifically means **Free Plan Org**.
