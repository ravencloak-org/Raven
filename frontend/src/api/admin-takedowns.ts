// Admin Marketplace takedown audit-log API client (issue #735).
//
// Read-only paginated list. The backend gates this with the global
// `raven_admin` allowlist (env `RAVEN_ADMIN_EMAILS`); a non-admin session
// gets HTTP 403 and the view surfaces a "forbidden" empty state.

export type AdminTakedownSource = 'publisher' | 'admin' | 'dmca'

export interface AdminTakedownRow {
  takedown_id: string
  target_kb_id: string
  target_kb_name: string
  target_org_slug: string
  target_org_display_name: string
  source: AdminTakedownSource
  notes: string
  created_at: string
  strikes_after_org_total: number
}

export interface AdminTakedownsPage {
  items: AdminTakedownRow[]
  next_cursor?: string
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

export interface ListAdminTakedownsParams {
  limit?: number
  cursor?: string
}

// HTTPError carries the status so the view can branch on 401/403 without
// re-parsing the message.
export class AdminTakedownsHTTPError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
    this.name = 'AdminTakedownsHTTPError'
  }
}

export async function listAdminTakedowns(
  params: ListAdminTakedownsParams = {},
): Promise<AdminTakedownsPage> {
  const qs = new URLSearchParams()
  if (params.limit) qs.set('limit', String(params.limit))
  if (params.cursor) qs.set('cursor', params.cursor)
  const suffix = qs.toString() ? `?${qs.toString()}` : ''
  const res = await authFetch(`/admin/marketplace/takedowns${suffix}`)
  if (!res.ok) {
    let detail = res.statusText
    try {
      const body = (await res.json()) as { detail?: string }
      if (body?.detail) detail = body.detail
    } catch {
      // body wasn't JSON — fall through with the status text.
    }
    throw new AdminTakedownsHTTPError(res.status, detail)
  }
  return res.json() as Promise<AdminTakedownsPage>
}
