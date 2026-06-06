# Marketplace Takedown Runbook

**Audience:** on-call admin handling the Marketplace abuse queue.
**Backing surface:** `/admin/marketplace/reports` (ADR-0008) + `/admin/marketplace/dmca`.
**Backing data:** `marketplace_reports`, `marketplace_takedowns`, `organizations.takedown_strikes`, `dmca_notices`.
**Authority:** ADR-0006 (licence & moderation), ADR-0008 (admin queue surface), plan §6 (`docs/plans/marketplace-mvp.md`).

> Schema is the source of truth. State-machine constants are defined in
> [`internal/marketplace/moderation.go`](../../internal/marketplace/moderation.go). If a value here drifts
> from the code, the code wins — open a PR to fix this runbook.

## 1. SLA

| Step | Target |
|------|--------|
| Initial response (report transitions `open` → `reviewing`) | **48 hours** from `marketplace_reports.created_at` |
| Resolution (terminal `resolved`/`dismissed`) | **7 days** from `created_at` |
| DMCA counter-notice window | **14 days** from notice receipt (enforced by `dmca_notices.counter_notice_window_ends`) |

Monitor SLA adherence on the **Time-to-Resolution** histogram in the
[Marketplace MVP dashboard](../../deploy/observability/dashboards/marketplace.json).

## 2. Workflow

The report state machine — see `ReportStatus` / `CanTransition` in
`internal/marketplace/moderation.go`:

```
open ──► reviewing ──► resolved
                  └──► dismissed
```

Terminal states have no outgoing edges. The DB CHECK constraint in
`migrations/00053_marketplace_moderation.sql` is the backstop.

### 2.1 Initial triage (`open` → `reviewing`)

1. Open `/admin/marketplace/reports`.
2. Pick the oldest `open` report.
3. Verify the report has enough detail to act on:
   - `reason` is specific (cites the offending content, not a generic
     "I don't like this").
   - The reported KB still exists and is `visibility='public'`. If the
     publisher already self-unpublished, dismiss with note
     `"superseded by publisher self-unpublish"`.
4. Click **Start review** — the admin handler transitions the report to
   `reviewing` (the only legal precursor to a terminal state).

### 2.2 Investigation

1. Open the reported KB and read its contents end-to-end.
2. Cross-check the cited material against the report's `reason`.
3. Document findings as draft notes — they become the `notes` on the
   `marketplace_takedowns` row if you approve.
4. If you need clarification from the reporter, ask via out-of-band
   email; do not respond in-app (the report-confirms-takedown loop is a
   known harassment vector per ADR-0008).

### 2.3 Decision

| Action | Effect |
|--------|--------|
| **Approve** | Calls `POST /api/v1/admin/marketplace/reports/{id}/approve` (issue #734). Atomic transaction: inserts `marketplace_takedowns(source='admin')`, transitions the KB to `private`, increments `organizations.takedown_strikes`, transitions the report to `resolved`. Returns `{ takedown_id, target_kb_id, strikes_after }`. |
| **Dismiss** | Transitions the report to `dismissed`. No notification to reporter (by design — see ADR-0008). No takedown row, no strike. |

The approve handler is the **only** path that increments
`takedown_strikes`. Never bump the column manually unless the runbook
in §4 (Org suspension) is being executed.

### 2.4 Publisher communication

#### Approve (takedown) email template

```
Subject: [Raven Marketplace] Your knowledge base has been taken down

Hi {publisher_display_name},

We received and reviewed a report about your public knowledge base
"{kb_title}" (id: {target_kb_id}).

After investigation, we found the content violates the Raven Marketplace
content policy. The knowledge base has been removed from the public
catalog and is now private to your organization.

Reason: {one_line_summary}

This is takedown strike {strikes_after} of 3 for your organization.
At 3 strikes, the organization is suspended from the Marketplace per
ADR-0006.

If you believe this is in error, reply to this email within 14 days.

— Raven Trust & Safety
```

#### Dismiss email template

```
Subject: [Raven Marketplace] Report dismissed

Hi {publisher_display_name},

A report was filed against your public knowledge base
"{kb_title}" (id: {target_kb_id}).

After investigation, we found no policy violation. No action has been
taken; your KB remains public.

— Raven Trust & Safety
```

Do **not** email the reporter in either case.

## 3. Derivative-owner notification

When you approve a takedown, the approve handler (#734) calls
`NotifyDerivativeOwners` from #735 to alert anyone who forked the
taken-down KB. The notifier walks the `kb_fork_links` chain and queues
an email per derivative owner.

You don't take any action here — but if the derivative-notification
queue stalls, derivative owners may complain. Surface in the
`marketplace_takedowns by source` panel in the dashboard; correlate with
ai-worker logs filtered by `notifier=DerivativeOwners`.

## 4. Strike increment monitoring

The approve handler increments `organizations.takedown_strikes`
atomically inside the takedown transaction. You should **never** run
`UPDATE organizations SET takedown_strikes = ...` directly — it would
desynchronise the strike count from the `marketplace_takedowns`
audit log.

To audit a publisher's strikes:

```sql
SELECT
  o.id,
  o.display_name,
  o.takedown_strikes,
  count(t.id) AS confirmed_takedowns
FROM organizations o
LEFT JOIN knowledge_bases k ON k.org_id = o.id
LEFT JOIN marketplace_takedowns t
  ON t.target_kb_id = k.id AND t.source IN ('admin', 'dmca')
WHERE o.id = '{org_id}'
GROUP BY o.id;
```

`takedown_strikes` and `confirmed_takedowns` should agree (both
`admin`-source and `dmca`-source takedowns increment the strike
counter — see §6 below for the DMCA path). A mismatch is a bug — page
the on-call engineer.

## 5. Org suspension (strikes ≥ 3)

When `organizations.takedown_strikes >= 3`, follow the dedicated
runbook: [`marketplace-repeat-infringer.md`](marketplace-repeat-infringer.md).

The short version:

```sql
-- Confirm strike count
SELECT id, display_name, takedown_strikes
FROM organizations
WHERE id = '{org_id}';

-- Suspend (org_status enum: 'active' | 'suspended' | 'deactivated',
-- defined in migrations/00001_extensions_and_types.sql)
UPDATE organizations
SET status = 'suspended'
WHERE id = '{org_id}'
  AND status = 'active'
  AND takedown_strikes >= 3;
```

Then run the SuperTokens admin signin-suspension flow and the
`visibility='public'` → `private` sweep documented in the repeat-infringer
runbook. Notify customer success.

## 6. DMCA notices

Out-of-scope for the report queue — formal DMCA notices arrive via
`dmca@ravencloak.org` and surface at `/admin/marketplace/dmca`.
See [`../legal/dmca.md`](../legal/dmca.md) for the public-facing policy and
the 14-day counter-notice procedure.

Submitting an admin DMCA action creates a `marketplace_takedowns` row
with `source='dmca'` and transitions the KB to `kb_status='dmca_pending'`
(see `migrations/00049_kb_status_marketplace_states.sql`). DMCA
takedowns increment the strike count via the same atomic transaction
once the counter-notice window elapses.

## 7. References

- ADR-0006 — `docs/adr/0006-licence-and-moderation.md`
- ADR-0008 — `docs/adr/0008-marketplace-discovery-and-operations.md`
- Plan §6 — `docs/plans/marketplace-mvp.md`
- Approve handler — issue #734
- Derivative notifier — issue #735
- DMCA inbox — issue #736
- Dashboard — `deploy/observability/dashboards/marketplace.json`
