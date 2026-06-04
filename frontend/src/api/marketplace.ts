// Marketplace API client (#732).
//
// Wraps the M2 Marketplace endpoints declared in contracts/openapi.yaml. The
// repo has no OpenAPI -> TS codegen pipeline today (verified at PR time: no
// openapi-typescript or @hey-api in package.json), so types are hand-mirrored
// from the spec. When codegen lands, swap these for generated types — the
// shapes were lifted verbatim from `MarketplaceListItem`, `MarketplaceListing`,
// `MarketplacePreview`, `ImportResult`, and `MarketplaceReport` in openapi.yaml.
//
// `MarketplaceApiError` exposes the raw HTTP status so callers can branch on
// 410 (slug-hold "Gone") and 403/404 without string-matching error messages.

const API_BASE = () => import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

export class MarketplaceApiError extends Error {
  status: number
  body: unknown
  constructor(status: number, message: string, body?: unknown) {
    super(message)
    this.name = 'MarketplaceApiError'
    this.status = status
    this.body = body
  }
}

async function marketplaceFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(API_BASE() + path, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })
  if (res.status === 204) return undefined as T
  const body = await res.json().catch(() => ({}))
  if (!res.ok) {
    const msg =
      (body && typeof body === 'object' && 'message' in body && typeof body.message === 'string'
        ? body.message
        : undefined) ||
      (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
        ? body.error
        : undefined) ||
      `Marketplace request failed (${res.status})`
    throw new MarketplaceApiError(res.status, msg, body)
  }
  return body as T
}

// --- Types (mirrors of openapi.yaml; flag in PR body) ---

export type MarketplaceSort = 'newest' | 'most_imported' | 'recently_updated' | 'alphabetic'

/** Curated SPDX allow-list (ADR-0006). */
export const SPDX_LICENSES = [
  'CC0-1.0',
  'CC-BY-4.0',
  'CC-BY-SA-4.0',
  'CC-BY-NC-4.0',
  'MIT',
  'Apache-2.0',
  'GPL-3.0',
] as const
export type SpdxLicense = (typeof SPDX_LICENSES)[number]

export interface MarketplaceListItem {
  kb_id: string
  org_slug: string
  org_display_name: string
  kb_slug: string
  name: string
  description?: string
  last_modified_at: string
  license_spdx_id: string
  import_count: number
  source_public_kb_id?: string | null
  source_org_slug?: string | null
  source_org_display_name?: string | null
}

export interface MarketplaceListing {
  items: MarketplaceListItem[]
  next_cursor?: string | null
  total?: number
}

export interface MarketplaceListQuery {
  q?: string
  sort?: MarketplaceSort
  license?: string[]
  limit?: number
  offset?: number
}

/** Detail row — shape matches `GET /marketplace/{org_slug}/{kb_slug}`, which
 * the spec types as `KnowledgeBase` plus the marketplace lineage + counts.
 * The card fields are included so the detail view can render the same chrome
 * without a second request. */
export interface MarketplaceKbDetail extends MarketplaceListItem {
  source_count?: number
  document_count?: number
  chunk_count?: number
}

export interface PreviewChunk {
  chunk_id: string
  ordinal: number
  text: string
}

export interface MarketplacePreview {
  chunks: PreviewChunk[]
}

export interface ImportResult {
  kb_id: string
  imported_from_revision_at: string
}

export interface MarketplaceReport {
  report_id: string
  reported_kb_id: string
  reporter_user_id?: string | null
  reason: string
  status: 'open' | 'reviewing' | 'resolved' | 'dismissed'
  created_at: string
}

// --- Endpoints ---

function buildQuery(params: MarketplaceListQuery): string {
  const sp = new URLSearchParams()
  if (params.q) sp.set('q', params.q)
  if (params.sort) sp.set('sort', params.sort)
  if (params.limit !== undefined) sp.set('limit', String(params.limit))
  if (params.offset !== undefined) sp.set('offset', String(params.offset))
  if (params.license) {
    for (const lic of params.license) sp.append('license', lic)
  }
  const qs = sp.toString()
  return qs ? `?${qs}` : ''
}

export function listMarketplace(params: MarketplaceListQuery = {}): Promise<MarketplaceListing> {
  return marketplaceFetch<MarketplaceListing>(`/marketplace${buildQuery(params)}`)
}

export function getMarketplaceKb(
  orgSlug: string,
  kbSlug: string,
): Promise<MarketplaceKbDetail> {
  return marketplaceFetch<MarketplaceKbDetail>(
    `/marketplace/${encodeURIComponent(orgSlug)}/${encodeURIComponent(kbSlug)}`,
  )
}

export function previewMarketplaceKb(
  orgSlug: string,
  kbSlug: string,
): Promise<MarketplacePreview> {
  return marketplaceFetch<MarketplacePreview>(
    `/marketplace/${encodeURIComponent(orgSlug)}/${encodeURIComponent(kbSlug)}/preview`,
  )
}

export function importMarketplaceKb(
  publicKbId: string,
  workspaceId: string,
): Promise<ImportResult> {
  return marketplaceFetch<ImportResult>(
    `/marketplace/import/${encodeURIComponent(publicKbId)}`,
    { method: 'POST', body: JSON.stringify({ workspace_id: workspaceId }) },
  )
}

export function reImportKnowledgeBase(
  orgId: string,
  wsId: string,
  kbId: string,
): Promise<ImportResult> {
  return marketplaceFetch<ImportResult>(
    `/orgs/${encodeURIComponent(orgId)}/workspaces/${encodeURIComponent(wsId)}/knowledge-bases/${encodeURIComponent(kbId)}/re-import`,
    { method: 'POST', body: JSON.stringify({ confirm: true }) },
  )
}

export function submitMarketplaceReport(
  kbId: string,
  reason: string,
): Promise<MarketplaceReport> {
  return marketplaceFetch<MarketplaceReport>('/marketplace/reports', {
    method: 'POST',
    body: JSON.stringify({ kb_id: kbId, reason }),
  })
}
