# Marketplace Repeat-Infringer Runbook

**Audience:** on-call admin executing the 3-strike Org suspension flow.
**Authority:** ADR-0006 (3 confirmed strikes → Org suspension via admin runbook), plan §6.
**Related:** [`marketplace-takedown.md`](marketplace-takedown.md) (drives the report → strike pipeline).

> Suspension is **manual for MVP** — there is no automatic trigger. An
> admin reviews the 3rd strike and decides whether to suspend.

## 1. Detection

Run this query (or read the **Top-Strike Orgs** panel in
[`deploy/observability/dashboards/marketplace.json`](../../deploy/observability/dashboards/marketplace.json)):

```sql
SELECT id, display_name, slug, takedown_strikes
FROM organizations
WHERE takedown_strikes >= 3
ORDER BY takedown_strikes DESC, display_name ASC;
```

Schema references:

- `organizations.takedown_strikes` — `migrations/00050_kb_license_and_strikes.sql`.
- `organizations.status` — `org_status` enum `('active','suspended','deactivated')`,
  defined in `migrations/00001_extensions_and_types.sql`.

Any row returned is a candidate. Continue to §2.

## 2. Final-warning email (third strike)

Sent automatically by the approve handler (#734) when the strike count
hits 3. Keep this template here as the canonical wording — change it
here and in the handler in lockstep.

```
Subject: [Raven Marketplace] Final warning — third takedown strike

Hi {org_display_name} admin,

Your organization has accrued 3 confirmed takedown strikes on the
Raven Marketplace. Per our content policy (ADR-0006), this is a final
warning.

Strikes (most recent first):
  - {takedown_3.created_at}: "{kb_title}" — {reason_summary}
  - {takedown_2.created_at}: "{kb_title}" — {reason_summary}
  - {takedown_1.created_at}: "{kb_title}" — {reason_summary}

Within the next 7 days, our Trust & Safety team will review your
organization's status. Possible outcomes:

  - Suspension of all Marketplace publishing privileges.
  - All public knowledge bases set to private.
  - Sign-in disabled for organization owners and members.

If you believe any of these takedowns was issued in error, reply to
this email with details.

— Raven Trust & Safety
```

## 3. Suspension procedure

Execute the four steps below **in order**. Each step is idempotent;
re-running on an already-suspended Org is a no-op.

### Step 3.1 — Snapshot the Org state

Capture before-state for the audit log:

```sql
SELECT
  id,
  display_name,
  slug,
  status,
  takedown_strikes,
  created_at
FROM organizations
WHERE id = '{org_id}';

SELECT id, title, visibility, kb_status
FROM knowledge_bases
WHERE org_id = '{org_id}'
  AND visibility = 'public';
```

Save the output to the takedown ticket / Stash entry.

### Step 3.2 — Suspend the Org

```sql
UPDATE organizations
SET status = 'suspended'
WHERE id = '{org_id}'
  AND takedown_strikes >= 3
  AND status = 'active';
```

The `takedown_strikes >= 3` guard prevents accidental suspension of an
Org that hasn't hit the threshold (e.g. wrong UUID pasted). The
`status = 'active'` guard makes the statement idempotent.

Verify exactly one row was updated. If zero, recheck the strike count
and current status before proceeding.

### Step 3.3 — Flip public KBs to private

```sql
UPDATE knowledge_bases
SET visibility = 'private'
WHERE org_id = '{org_id}'
  AND visibility = 'public';
```

This removes every Public KB from the Marketplace surface. We do **not**
delete content — the Org retains private access for export / appeal.

### Step 3.4 — Suspend SuperTokens sign-in

For each user belonging to the suspended Org, suspend the SuperTokens
session via the admin CLI:

```bash
supertokens-admin user suspend --email <owner_email>
```

(Repeat per Org member. The exact CLI invocation lives with the
SuperTokens deployment docs; if it has drifted, update both that doc
and this runbook in the same PR.)

### Step 3.5 — Notify the Org

```
Subject: [Raven Marketplace] Your organization has been suspended

Hi {org_display_name} admin,

Following 3 confirmed takedown strikes and our 7-day review window,
your Raven organization has been suspended from the Marketplace.

  - Sign-in for all members is disabled.
  - All public knowledge bases have been set to private.
  - Your data remains intact and is available for export on request.

To appeal this decision, reply to this email within 30 days with:
  - The reason you believe the suspension was issued in error, AND
  - Steps you would take to prevent further infringement.

Without an appeal, this suspension becomes permanent after 30 days.

— Raven Trust & Safety
```

### Step 3.6 — Handover to customer success

Open a customer-success ticket with:

- Org ID, display name, slug.
- Strike summary (3+ rows from `marketplace_takedowns` joined to
  `knowledge_bases` on `target_kb_id`).
- Suspension timestamp.
- Owner email(s).

CS owns follow-up calls and the appeal review.

## 4. Reinstatement criteria

An Org may be reinstated if **all** of the following hold:

1. Owner submitted a written appeal within 30 days of suspension.
2. The appeal acknowledges the infringement and explains preventative
   measures (e.g. internal review process, takedown of source
   material).
3. No additional reports against the Org's private KBs in the
   suspension window (queried via `marketplace_reports`
   where the report's `reported_kb_id` rolls up to this Org).
4. Customer success and Trust & Safety both sign off.

Reinstatement query (reverse of §3.2):

```sql
UPDATE organizations
SET status = 'active'
WHERE id = '{org_id}'
  AND status = 'suspended';
```

`takedown_strikes` is **not** reset on reinstatement — a fourth strike
re-triggers this runbook immediately. The strike counter is a permanent
audit trail per ADR-0006.

Re-enable SuperTokens sign-in:

```bash
supertokens-admin user reinstate --email <owner_email>
```

Send a reinstatement email referencing the appeal and the conditions
under which the Org may publish again.

## 5. References

- ADR-0006 — `docs/adr/0006-licence-and-moderation.md`
- Plan §6 — `docs/plans/marketplace-mvp.md`
- Takedown runbook — `marketplace-takedown.md`
- Strikes column — `migrations/00050_kb_license_and_strikes.sql`
- Org status enum — `migrations/00001_extensions_and_types.sql`
- Dashboard — `deploy/observability/dashboards/marketplace.json`
