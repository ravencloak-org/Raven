# Raven DMCA Policy

Raven respects the intellectual property rights of others and expects Marketplace publishers and importers to do the same. This document is the operational reference for the DMCA workflow; the public-facing equivalent lives at [raven.ravencloak.org/legal/dmca](https://raven.ravencloak.org/legal/dmca).

## Designated Agent

| Field           | Value                                                |
|-----------------|------------------------------------------------------|
| Email           | [dmca@ravencloak.org](mailto:dmca@ravencloak.org)    |
| Hosting Org     | Raven (ravencloak-org)                               |
| Statute         | 17 U.S.C. § 512(c)(3) — DMCA designated-agent inbox  |

`dmca@ravencloak.org` is provisioned as a Google Workspace alias that fans out to the platform-admin distribution list. The inbox MUST stay reachable before the public Marketplace ships — see ADR-0006 §Consequences.

## Submitting a DMCA Notice

A valid notice under 17 U.S.C. § 512(c)(3) MUST include:

1. A physical or electronic signature of the copyright owner or authorised agent.
2. Identification of the copyrighted work.
3. Identification of the allegedly infringing material with a URL or other sufficient location data.
4. Contact information (mailing address, phone, email).
5. A good-faith statement that the use is not authorised.
6. A statement under penalty of perjury that the notifier is the copyright owner or authorised agent.

Notices that materially fail the statutory requirements are rejected without action; the recipient operator replies with a short rejection email pointing back at this page.

## Operational Workflow

```
notice arrives at dmca@ravencloak.org
        │
        ▼
admin records via /admin/marketplace/dmca           ─── (1) Submit
        │
        ▼ atomic
INSERT dmca_notices (status='pending', window=14d)
UPDATE knowledge_bases SET status='dmca_pending'
        │
        ▼ outside tx
claimant receipt email (best-effort)
        │
        ▼ wait up to 14 days
        │
        ├── publisher counter-notices via dmca@      ─── (2) Counter
        │       admin records via /admin/marketplace/
        │           dmca/:id/counter-notice
        │       status='counter_filed'; KB stays frozen
        │
        └── window expires with no counter-notice    ─── (3) Sweep
                daily cron `scheduled:marketplace_dmca_sweep` (4am UTC)
                per row:
                  UPDATE dmca_notices SET status='resolved_take_down'
                  UPDATE knowledge_bases SET visibility='private', status='active'
                  INSERT marketplace_takedowns (source='dmca', notes=notice_id)
                  DispatchOnTakedownCreated → derivative_notifier emails fork owners
```

The two-stage workflow lives in `internal/marketplace/dmca.go`; the sweeper lives in `internal/jobs/marketplace_dmca_sweeper.go`.

### Counter-notice handling

The MVP records the publisher's counter-notice through the admin (the publisher emails `dmca@ravencloak.org` with their counter-notice text; the operator pastes it into the admin UI). There is no public counter-notice form in MVP.

When a counter-notice is recorded, the KB stays in `dmca_pending`. The DMCA safe harbour (17 U.S.C. § 512(g)(2)(C)) requires the OSP to hold restoration for 10 to 14 business days to give the original claimant time to file suit. The final keep-up/take-down decision is a manual admin action in MVP.

## Repeat-Infringer Policy

Per 17 U.S.C. § 512(i) and ADR-0006, Raven tracks confirmed takedown strikes against the publishing Organisation via `organizations.takedown_strikes`. Three strikes triggers the Organisation-suspension runbook (separate operational document, out of scope for this file). DMCA-sourced takedowns also increment the strike counter; the counter is incremented by the admin approve path (#734) and the same mechanism applies to DMCA auto-resolutions.

> NOTE: The DMCA sweeper currently writes a `marketplace_takedowns` row with `source='dmca'` but does NOT increment `takedown_strikes` automatically. Whether DMCA auto-resolutions count toward strikes is a policy decision tracked separately; the implementation will follow once that decision lands.

## Misrepresentation Liability

17 U.S.C. § 512(f) provides a damages remedy against parties who knowingly misrepresent that material is infringing (or that material was removed by mistake). Operators rejecting bad-faith notices should keep the rejection email on file.

## Implementation References

| Component              | Path                                                                |
|------------------------|---------------------------------------------------------------------|
| Migration              | `migrations/00055_marketplace_dmca.sql`                             |
| Service                | `internal/marketplace/dmca.go`                                      |
| Sweeper handler        | `internal/jobs/marketplace_dmca_sweeper.go`                         |
| Sweeper cron           | `internal/jobs/scheduler.go` — `CronMarketplaceDMCASweep`           |
| HTTP handler           | `internal/handler/admin_dmca.go`                                    |
| Admin UI               | `frontend/src/pages/admin/AdminDMCAView.vue`                        |
| Public-facing page     | `frontend/src/pages/legal/DMCAPage.vue` (`/legal/dmca`)             |
| API client             | `frontend/src/api/marketplace-admin.ts`                             |

## See Also

- [ADR-0006 — Mandatory licence + reactive moderation](../adr/0006-licence-and-moderation.md)
- [ADR-0008 — Marketplace discovery + operations](../adr/0008-marketplace-discovery-and-operations.md)
- [Marketplace MVP plan §4 (admin endpoints), §6 (DMCA inbox)](../plans/marketplace-mvp.md)
