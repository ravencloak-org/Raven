// API client for the admin marketplace review queue (issue #734).
// Mirrors the per-endpoint shape in docs/plans/marketplace-mvp.md §4.
//
// The admin gate is enforced server-side by RequirePlatformAdmin
// middleware against RAVEN_ADMIN_EMAILS. This client trusts the SPA
// route guard to keep the unauthorised user away, but a tampered client
// hitting these endpoints still gets a clean 403.

export type ReportStatus = 'open' | 'reviewing' | 'resolved' | 'dismissed'

export interface MarketplaceReport {
  id: string
  reported_kb_id: string
  reporter_user_id: string | null
  reason: string
  status: ReportStatus
  created_at: string
}

export interface ListReportsResponse {
  reports: MarketplaceReport[]
  limit: number
  offset: number
}

export interface ApproveResult {
  takedown_id: string
  target_kb_id: string
  strikes_after: number
}

export interface DismissResult {
  report_id: string
  status: 'dismissed'
}

async function authFetch(path: string, init?: RequestInit): Promise<Response> {
  const base = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'
  return fetch(base + path, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })
}

async function jsonOrThrow<T>(res: Response, op: string): Promise<T> {
  if (res.ok) return (await res.json()) as T
  const body = await res.json().catch(() => ({}) as Record<string, unknown>)
  const detail =
    typeof body.detail === 'string'
      ? body.detail
      : typeof body.message === 'string'
        ? body.message
        : res.statusText
  const err = new Error(`${op}: ${res.status} ${detail}`) as Error & { status: number }
  err.status = res.status
  throw err
}

/**
 * Fetches the admin review queue. Server-side default is `status=open`
 * which matches the queue's live-work view; callers can pass any
 * status to inspect history.
 */
export async function listReports(
  status: ReportStatus = 'open',
  limit = 50,
  offset = 0,
): Promise<ListReportsResponse> {
  const qs = new URLSearchParams({
    status,
    limit: String(limit),
    offset: String(offset),
  })
  const res = await authFetch(`/admin/marketplace/reports?${qs.toString()}`)
  return jsonOrThrow<ListReportsResponse>(res, 'listReports')
}

/**
 * Approves a report: unpublishes the KB, increments the publisher Org's
 * `takedown_strikes`, writes a takedown audit row, and resolves the
 * report — all atomically. Returns the strikes counter after the
 * increment so the UI can show the "this is their N-th strike" banner.
 */
export async function approveReport(reportId: string): Promise<ApproveResult> {
  const res = await authFetch(`/admin/marketplace/reports/${reportId}/approve`, {
    method: 'POST',
  })
  return jsonOrThrow<ApproveResult>(res, 'approveReport')
}

/**
 * Dismisses a report: closes the report without any takedown / strike /
 * notification. The reporter is intentionally NOT notified
 * (ADR-0008 §Why — report-confirms-takedown loop is a harassment vector).
 */
export async function dismissReport(reportId: string): Promise<DismissResult> {
  const res = await authFetch(`/admin/marketplace/reports/${reportId}/dismiss`, {
    method: 'POST',
  })
  return jsonOrThrow<DismissResult>(res, 'dismissReport')
}

// ─── DMCA inbox + counter-notice workflow (issue #736) ─────────────────────

export type DMCAStatus =
  | 'pending'
  | 'counter_filed'
  | 'resolved_take_down'
  | 'resolved_keep_up'
  | 'withdrawn'

export interface DMCANotice {
  id: string
  target_kb_id: string
  notice_text: string
  claimant_email: string
  claimant_name: string
  counter_notice_text?: string | null
  counter_notice_submitted_at?: string | null
  counter_notice_window_ends: string
  status: DMCAStatus
  resolved_at?: string | null
  created_at: string
}

export interface ListDMCANoticesResponse {
  notices: DMCANotice[]
  limit: number
  offset: number
}

export interface DMCASubmitInput {
  target_kb_id: string
  notice_text: string
  claimant_email: string
  claimant_name: string
}

/**
 * Lists DMCA notices visible to admins. Status filter is optional;
 * empty means all five statuses. The `pending` filter is the live
 * work-view (notices still inside the 14-day counter-notice window).
 */
export async function listDMCANotices(
  status?: DMCAStatus,
  limit = 50,
  offset = 0,
): Promise<ListDMCANoticesResponse> {
  const qs = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  })
  if (status) qs.set('status', status)
  const res = await authFetch(`/admin/marketplace/dmca?${qs.toString()}`)
  return jsonOrThrow<ListDMCANoticesResponse>(res, 'listDMCANotices')
}

/**
 * Records a fresh DMCA notice arriving via dmca@ravencloak.org. Atomic:
 * the target KB flips to `kb_status='dmca_pending'` and the 14-day
 * counter-notice clock starts. Returns the full notice including
 * `counter_notice_window_ends` so the UI can render the countdown.
 *
 * 409 means the KB already has a pending notice (one-active-notice-per-
 * KB invariant — see ADR-0006).
 */
export async function submitDMCANotice(input: DMCASubmitInput): Promise<DMCANotice> {
  const res = await authFetch(`/admin/marketplace/dmca`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
  return jsonOrThrow<DMCANotice>(res, 'submitDMCANotice')
}

/**
 * Records a publisher counter-notice on the named DMCA notice. MVP
 * simplification: admin acts on behalf of the publisher who replied
 * to dmca@ravencloak.org. The KB stays frozen until an admin issues
 * the final keep-up/take-down call (safe harbour requires the OSP to
 * hold restoration for 10-14 business days per 17 U.S.C. § 512(g)(2)(C)).
 */
export async function submitCounterNotice(
  noticeId: string,
  counterNoticeText: string,
): Promise<{ notice_id: string; status: 'counter_filed' }> {
  const res = await authFetch(`/admin/marketplace/dmca/${noticeId}/counter-notice`, {
    method: 'POST',
    body: JSON.stringify({ counter_notice_text: counterNoticeText }),
  })
  return jsonOrThrow(res, 'submitCounterNotice')
}
