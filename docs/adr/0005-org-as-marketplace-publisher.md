# 0005 — Org is the Marketplace publisher

Status: accepted

## Decision

The public-facing publisher of a KB on the Marketplace is the **Org** that owns it, not the User who pressed Publish. Marketplace cards, URLs, and Importer-side lineage all display the Org's `display_name` and `slug`. The User who triggered the publish event is recorded in `published_by_user_id` for audit only and is never shown publicly.

Marketplace KB URL: `raven.ravencloak.org/marketplace/{org_slug}/{kb_slug}`. This requires `org_slug` to become **globally unique and URL-safe** — a schema change from today's free-form `display_name`.

Republish and unpublish rights are gated by Org membership + a `kb:publish` permission, not by the original publishing User. A KB outlives any individual User in the Org.

For Derivative Public KBs (forks that were themselves published), the card shows the Importer Org as publisher and adds a "Forked from {original Org display_name}" line linking back. Only one level of lineage is rendered by default; deeper chains are reachable by following the link.

## Why

The data ownership model already names the Org as the legal owner of every KB row: `org_id` is the RLS axis, Subscriptions are per-Org, and `ON DELETE CASCADE` flows from Org to KB. Making the User the public-facing publisher would be a stack of small lies against this model:

- Multi-user Orgs (the team-collaboration case) would publish with one engineer's name on collective work.
- User offboarding would create orphan-attribution rows — KBs whose stated publisher no longer exists.
- Republish/unpublish rights coupled to the original publishing User would lock out Org admins from managing their own data.
- A future User-credit feature (an optional "Maintainer" field, User-controlled) can be added without changing the publisher-is-Org foundation. The reverse migration is much harder.

For free-tier single-user Orgs, this degrades gracefully: the Org's `display_name` defaults to the User's name at signup, so the card reads "by Jobin Lawrance" naturally. No corporate cosplay.

## Trade-offs accepted

- **Org rename = Marketplace card rename.** No frozen historical attribution. If Acme rebrands, every Marketplace card under that Org renames live. Acceptable: same legal entity, same liability. If demand emerges, we can snapshot `published_org_display_name` per publish event.
- **User credit is hidden in MVP.** A creator who wants personal recognition has no surface for it today. Mitigated by future optional "Maintainer" field; not in MVP.
- **`org_slug` must become globally unique and URL-safe.** Today it's effectively free-form `display_name`. Migration: add `slug VARCHAR(64) UNIQUE` to `organizations`, backfill via slugified `display_name` with collision suffixes (`-2`, `-3`), enforce at the API.
- **Derivative lineage is one-hop in UI.** A KB forked through three Orgs shows only its immediate predecessor by default. Deeper chains exist in data (`source_public_kb_id` traversal) but aren't UX-rendered to avoid noise. Power users can follow.

## Consequences

- New `organizations.slug` column, `UNIQUE` constraint, backfilled at migration.
- `knowledge_bases.published_by_user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL` (already in ADR-0002 plan) — kept for audit, never read by Marketplace endpoints.
- `kb:publish` becomes a discrete permission on the role model; default for Org admins, optional for members per Org policy.
- The Marketplace listing function (a `SECURITY DEFINER` view per ADR-0001) returns `(kb_id, org_slug, org_display_name, kb_slug, kb_name, description, last_modified_at, source_public_kb_id, source_org_slug, source_org_display_name)` and nothing tying back to individual Users.
- URL routing: `/marketplace/{org_slug}/{kb_slug}` is the canonical public URL. 404s if either slug is unknown or the target is private.
