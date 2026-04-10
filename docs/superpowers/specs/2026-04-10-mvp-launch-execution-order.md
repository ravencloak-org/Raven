# MVP Launch Execution Order

**Date:** 2026-04-10
**Milestone:** MVP Launch (4 open issues: #193, #194, #197, #200) + 7 non-milestone fixes/chores
**Strategy:** Fix the foundation, then build features — three sequential waves with parallelism within each wave.

---

## Summary

11 open issues prioritized into 3 waves. Wave 1 clears remaining quick bug fixes for a clean baseline. Wave 2 tackles the billing backend (which has an existing implementation plan) and one EE test gap. Wave 3 delivers the remaining MVP frontend features and the onboarding wizard. Two P2 chores are deferred.

**Already closed (no action needed):** #230, #231, #234, #235 — resolved before this plan was written.

---

## Pre-flight: Close #200

**Issue:** #200 — Mobile-first responsive redesign (parent issue)
**Action:** Close now. All 5 sub-issues (#221, #222, #223, #224, #225) are merged.
**Effort:** Zero — just close the issue.

---

## Wave 1 — Quick Fixes (parallel, no dependencies)

Three remaining issues, all independent and small-scope. Can be executed as parallel worktree agents. Each produces one commit and one PR.

| # | Title | Fix | Priority | Effort |
|---|-------|-----|----------|--------|
| #233 | Harden `testutil.NewTestDB` migrations path | Add `os.Stat(migrationsDir)` guard before `goose.Up` in `internal/testutil/db.go` | P1 | ~10 lines |
| #237 | Clean up `TestPackageCompiles` and `t.Skip` in EE stubs | Remove `t.Skip` from compile tests, remove or implement WAF concept tests | P2 (included for trivial effort + test hygiene) | Small |
| #232 | Add REVOKE cleanup to RLS test fixtures | Add `t.Cleanup` with `REVOKE` in `internal/repository/rls_test.go` | P2 (included for trivial effort + test hygiene) | ~5 lines |

**Entry criteria:** None — can start immediately.
**Exit criteria:** All 3 PRs merged, CI green.

---

## Wave 2 — EE Test Fix + Billing Backend (parallel, independent tracks)

Two independent tracks. Both can run concurrently.

### Track A: Webhook Retry/Dead-Letter Tests (#236)

**Issue:** #236 — Replace permanent `t.Skip` on webhook retry/dead-letter tests
**Priority:** P1

**Scope:**
- Convert skipped tests in `internal/ee/webhooks/webhooks_test.go` (lines 85-111) into integration tests
- Use testcontainers for Valkey + Asynq test server
- Verify retry behavior and dead-letter queue semantics

### Track B: Billing Subscription Enforcement (#193)

**Issue:** #193 — Subscription enforcement: plan limit checks and feature gates
**Priority:** P1 (MVP blocker)
**Existing plan:** `docs/superpowers/plans/2026-04-10-billing-subscription-enforcement.md`

**Scope (from existing plan):**
- `QuotaChecker` service with Valkey-cached subscription lookups (TTL 5 min)
- Limit-check methods injected into KB, workspace, and voice services
- `GET /billing/usage` endpoint exposing current-period usage vs limits
- 402 Payment Required responses with `{"upgrade_required": true, "limit": N}`
- New billing repo queries: `CountKBsByOrg`, `CountMembersByOrg`, `GetVoiceUsageForPeriod`

**Entry criteria:** Wave 1 merged (clean test baseline).
**Exit criteria:** Both tracks merged, CI green.

---

## Wave 3 — MVP Frontend Features

Two features, one with a dependency. #197 can start during Wave 2 (design spec phase) since its only real entry criterion is having a design spec written — not Wave 2 completion.

### Billing UI (#194)

**Issue:** #194 — Billing and subscription management UI
**Blocked by:** #193 (needs backend `GET /billing/usage` endpoint and quota enforcement)

**Scope:**
- Plan selection page at `/settings/billing` with Free/Pro/Enterprise cards
- Payment flow integration via Hyperswitch
- Usage dashboard showing current-period consumption vs plan limits
- Upgrade prompt when 402 responses are received

**Entry criterion:** #193 merged.
**Needs:** Its own design spec before implementation (frontend feature with payment integration).

### Keycloak Onboarding Wizard (#197)

**Issue:** #197 — Keycloak realm auto-provisioning and tenant onboarding wizard
**Blocked by:** Nothing — design spec work can begin during Wave 2.

**Scope:**
- Backend: `POST /internal/provision-realm` endpoint (internal-only)
- Keycloak Admin API integration: create realm, configure client, set redirect URIs
- Frontend: first-run onboarding wizard for new tenants

**Entry criterion:** Design spec written and approved.
**Needs:** Its own design spec before implementation (involves Keycloak Admin API, security considerations).

**Exit criteria (Wave 3):** All MVP Launch issues closed, milestone complete.

---

## Deferred (P2, no milestone)

| # | Title | Reason |
|---|-------|--------|
| #238 | Document required secrets for Playwright E2E tests | P2, CI docs — no user impact |
| #239 | Document rationale for dependency downgrades in go.mod | P2, documentation only |
| #240 | Restore deleted dashboard JSON files | P2, observability config — #235 (the code-level fix) is already closed; these are standalone dashboard configs |

---

## Execution Summary

```
Pre-flight:         close #200                            →  0 PRs, 1 issue closed
Wave 1 (parallel):  #233, #237, #232                      →  3 PRs
Wave 2 (parallel):  #236, #193                            →  2 PRs
Wave 3:             #194 (after #193), #197 (independent) →  2 PRs
Deferred:           #238, #239, #240                      →  backlog
                                                          ─────────
                                                          7 PRs, 8 issues resolved
                                                          (+ 4 already closed: #230, #231, #234, #235)
```
