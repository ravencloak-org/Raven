# 0004 — Free Plan Orgs hold only Public KBs

Status: accepted

## Decision

For any Org without an active paid Subscription (a **Free Plan Org**):

1. **Creating a KB** — `visibility` is forced to `public`. Private creation is not selectable.
2. **Importing a Public KB** — the resulting Imported KB is also forced `public`. Free Plan Orgs cannot hold private forks.
3. **Plan downgrade (Paid → Free)** — existing private KBs transition to status `read_only_private`. They remain private (not auto-published), but cannot accept new Sources, Documents, or Chats. The User must explicitly Publish, Delete, or upgrade back to restore the KB.
4. **Plan upgrade (Free → Paid)** — no automatic state change. Existing Public KBs stay public until the User manually converts them.

Enforcement is at write-time on `knowledge_bases` (and the Import code path), not via batch sweep — bad rows must be impossible to write, not eventually-corrected.

## Why

The user-stated rule "Free Plan can only create Public KBs" leaves three edge cases that determine whether the Marketplace is a flywheel or a faucet:

1. **Import without forced-public** would create a loophole: a Free Plan user imports 50 Public KBs and now enjoys 50 effectively-private KBs in their workspace. The free tier becomes "private as long as someone else seeded the content." Marketplace stays one-way: stuff flows out, nothing flows back as derivatives.

2. **Downgrade auto-publishing** would silently leak private content the User never consented to share — a hard-no.

3. **Upgrade auto-privating** would retroactively yank Public KBs that Importers may be tracking; breaks the "Updated 3 days ago" contract on the Marketplace listing.

Strict-flywheel + frozen-private + manual-upgrade is the only combination that closes the loophole, respects consent, and preserves Importer expectations.

## Trade-offs accepted

- **Derivative Public KBs create attribution tangle.** Acme Corp publishes → 200 Free Plan users import → 200 derivative Public KBs exist. Each is a separate row, each independently editable. UI must show lineage clearly (`source_public_kb_id` already on the row from ADR-0002). Solved in attribution design, not here.
- **Takedown does not cascade.** Acme retracts → derivatives still exist. Standard OSS-attribution practice: the publisher's act of making something public is irrevocable for downstream users. Mitigated by mandatory licence declaration (separate decision) which gives downstream Orgs a legal basis to keep their forks.
- **`read_only_private` is a new KB status that touches every read path.** Chat sessions must check it and surface a graceful "frozen" message; widgets must return a maintenance response; the worker must skip embedding new docs into it. Real implementation cost, but bounded — the status check piggybacks on the existing `status` column on `knowledge_bases`.
- **No grace period on downgrade.** A Paid Org that cancels lands instantly in the frozen state; their dashboards stop letting them add to private KBs immediately. We've decided friction > silent privacy decisions. A future grace-period feature can be additive.

## Consequences

- `kb_status` enum gains `read_only_private` alongside the existing `active`, `archived`.
- Downgrade flow (Hyperswitch webhook → subscription status change → org effective plan recompute) must run a transactional update: `UPDATE knowledge_bases SET status='read_only_private' WHERE org_id = $1 AND visibility='private' AND status='active'`. Idempotent.
- Upgrade flow does nothing automatic.
- KB creation API (`POST /api/v1/workspaces/:id/knowledge_bases`) reads the Org's effective plan and overrides `visibility=public` for Free Plan Orgs, regardless of request payload.
- Import API (`POST /api/v1/marketplace/import/:public_kb_id`) applies the same override on the destination KB's row.
- Plan transition observability: every downgrade emits an event listing affected KB ids — debuggable, auditable.
