# 0006 — Mandatory license + reactive moderation

Status: accepted

## Decision

**License.** Every Public KB declares an SPDX license at first publish, chosen from a curated allow-list. No default, no override, no free-form text — publishers actively pick. Allow-list: `CC0-1.0`, `CC-BY-4.0`, `CC-BY-SA-4.0`, `CC-BY-NC-4.0`, `MIT`, `Apache-2.0`, `GPL-3.0`. The license badge is rendered on every Marketplace card and on Importer-side KB metadata. License compatibility across forks is not enforced — Raven displays badges, users reason about compatibility.

**Moderation.** Reactive only: a "Report" button on every Public KB, an internal admin review queue, and a documented DMCA process at the designated agent address `dmca@ravencloak.org`. Repeat infringers tracked per Org via `organizations.takedown_strikes` (3 confirmed strikes → Org suspension via admin runbook).

**Takedown cascade.** Takedowns (publisher, admin, or DMCA) **do not auto-cascade to derivatives**. Owners of Derivative Public KBs receive an email explaining the root was taken down and inviting them to review their fork; they decide whether to unpublish.

## Why

`NC-ND` and proprietary licenses are excluded because no-derivatives contradicts fork-on-import — the entire Import model produces derivative works. Defaulting (rather than requiring) a license invites publishers to ship under a default they never read; mandatory selection forces a conscious act. Reactive moderation is the only honest MVP — "none" is irresponsible the moment content goes public, and proactive ML moderation has false-positive UX costs we can't afford at MVP volume.

Notify-don't-cascade protects legitimate forks from DMCA weaponisation. The OSS norm is that downstream users keep their freedoms when upstream retracts; a single bad-faith DMCA shouldn't kill hundreds of derivatives. Repeat-infringer suspension is at the Org axis (matches our Subscription axis), not the KB axis.

## Trade-offs accepted

- DMCA designated agent inbox must be staffed within reasonable response windows or Raven loses safe-harbor footing. Real ops commitment.
- Bad actors can spin up fresh Orgs faster than admins can suspend them. Acceptable at MVP scale; revisit if abuse signal climbs.
- License-compatibility across import chains is the user's problem. Documented in CONTEXT.md, not enforced.
- No public-domain auto-detection: a publisher uploading Wikipedia content under the wrong license is on the publisher.

## Consequences

- `knowledge_bases.license_spdx_id TEXT NOT NULL` for `visibility='public'` rows. Enforced via partial CHECK constraint.
- License allow-list is a Go const slice in `internal/marketplace/license.go`, not config — code review gates the set.
- `organizations.takedown_strikes INTEGER NOT NULL DEFAULT 0`.
- New tables (MVP): `marketplace_reports (id, reported_kb_id, reporter_user_id, reason, status, created_at)`, `marketplace_takedowns (id, target_kb_id, source enum('publisher','admin','dmca'), notes, created_at)`.
- DMCA inbox `dmca@ravencloak.org` provisioned + runbook before public launch.
