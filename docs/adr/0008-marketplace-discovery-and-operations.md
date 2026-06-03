# 0008 — Marketplace discovery and operations scope

Status: accepted

## Decision

**Discovery (Q8).** `/marketplace` ships with:

- A single text search box backed by Postgres full-text search over `name + description + org_display_name` (one `tsvector` column on `knowledge_bases`, one GIN index).
- Four sort options: **Newest published**, **Most imported (lifetime)**, **Recently updated**, **Alphabetic**.
- One filter: **License** (multi-select against the 7-item allow-list from ADR-0006).
- Two denormalised counters on `knowledge_bases`: `import_count` (exact, trigger-maintained on Importer-side KB insert when `source_public_kb_id IS NOT NULL`) and `preview_count` (eventually consistent, incremented by the preview endpoint handler).
- No categories, no tags, no semantic search over published chunks, no trending algorithm. Each is independently addable later.

**Per-plan quotas (Q9).** Out of scope for the Marketplace architecture. The publish, import, KB-creation, and document-upload paths read the Org's `effective_plan` and call a single `enforceQuota(plan, action)` hook — config-driven, no schema dependency. The actual quota numbers are a billing decision tracked separately.

**Admin review queue (Q10).** A minimal Vue page at `/admin/marketplace/reports`, gated by the existing `raven_admin` Postgres role used for RLS bypass elsewhere. Two binary actions per report: **Approve** (unpublishes the KB, increments `takedown_strikes` on the publisher Org, emails the publisher) and **Dismiss** (closes the report, no notification to reporter). A parallel `/admin/marketplace/dmca` page handles formal DMCA notices with the two-stage 14-day counter-notice window before takedown becomes effective.

## Why

Each sub-decision is the minimum viable surface that doesn't paint us into a corner:

- **FTS on metadata only** keeps the marketplace search using the same infrastructure as in-org KB search — no new dependency, no Elasticsearch. Content search is layerable later as a separate SECURITY DEFINER function over published chunks.
- **Four hard sorts, no ranking algorithm** because trending requires time-window counters, decay functions, and recurring jobs we don't need before measuring actual user behaviour.
- **No categories, no tags** because taxonomy is a tar pit at MVP scale; we should let publisher behaviour reveal the natural axes before encoding them in schema.
- **Quotas as a hook, not a schema** because we want to tune limits in config when product/billing decides, without redeploying.
- **Binary admin actions** because intermediate states (warn, probation, etc.) compound runbook complexity without measurable benefit at MVP scale.
- **Reporter not notified by default** because the report-confirms-takedown loop is a known harassment vector on other platforms.

## Trade-offs accepted

- **No content search at launch.** Users searching for "any KB containing pgvector tutorials" must rely on metadata or browse. Layerable later.
- **`Most imported` is gameable.** Bot-spam imports can boost a KB's ranking. Acceptable at MVP volume; revisit when abuse appears.
- **Counters are denormalised.** `import_count` and `preview_count` live on `knowledge_bases` to avoid expensive aggregations in the listing function. Trigger maintenance is small but real ongoing surface.
- **Admin UI is internal-tooling-grade.** No accessibility polish, no admin-role granularity beyond `raven_admin`. Sufficient for the volumes we expect; revisit if we onboard non-engineer admins.
- **Repeat-infringer suspension is manual.** Three strikes triggers a runbook task, not an automatic suspension. Acceptable: low-volume MVP; bad actors creating fresh Orgs are a known issue we won't solve mechanically yet.

## Consequences

- `knowledge_bases` gains `search_tsv tsvector` (GENERATED stored, GIN-indexed), `import_count INTEGER NOT NULL DEFAULT 0`, `preview_count INTEGER NOT NULL DEFAULT 0`.
- Trigger on Importer-side KB insert increments `import_count` on the source Public KB via `source_public_kb_id`.
- Listing endpoint surfaces: `q`, `sort`, `license[]` query params. SECURITY DEFINER `marketplace_list_public_kbs(q TEXT, sort TEXT, licenses TEXT[], limit INT, offset INT)`.
- `enforceQuota(plan, action)` hook lives in `internal/billing/quota.go`; called from publish, import, KB create, doc upload paths. Implementation in MVP can be `func enforceQuota(...) error { return nil }` — wired into the call sites so future changes are config-only.
- Admin pages live under `/admin/marketplace/*`, served by the same SPA but gated server-side by a session check that the User's Postgres role is `raven_admin`.
- DMCA workflow: notice received → KB transitions to `dmca_pending`, publisher emailed with 14-day counter-notice window → if no counter-notice, transition to `private` + admin records takedown. New `kb_status` value: `dmca_pending`.
