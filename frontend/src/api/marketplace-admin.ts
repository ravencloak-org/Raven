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
