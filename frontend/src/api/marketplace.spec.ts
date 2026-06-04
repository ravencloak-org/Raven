import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import {
  listMarketplace,
  getMarketplaceKb,
  previewMarketplaceKb,
  reImportKnowledgeBase,
  MarketplaceApiError,
  SPDX_LICENSES,
} from './marketplace'

// `buildQuery` is unexported; we re-test through the public list endpoint
// by inspecting the fetched URL via the mocked global fetch.

describe('marketplace api client', () => {
  const originalFetch = globalThis.fetch
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    fetchMock = vi.fn()
    globalThis.fetch = fetchMock as unknown as typeof fetch
  })
  afterEach(() => {
    globalThis.fetch = originalFetch
  })

  it('serialises license[] as repeated query parameters', async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ items: [] }), { status: 200 }),
    )
    await listMarketplace({
      q: 'rag',
      sort: 'most_imported',
      license: ['MIT', 'Apache-2.0'],
      limit: 50,
      offset: 20,
    })
    const url = String(fetchMock.mock.calls[0][0])
    expect(url).toContain('q=rag')
    expect(url).toContain('sort=most_imported')
    expect(url).toContain('limit=50')
    expect(url).toContain('offset=20')
    expect(url).toMatch(/license=MIT/)
    expect(url).toMatch(/license=Apache-2\.0/)
  })

  it('throws MarketplaceApiError with status on 410 Gone', async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ message: 'gone' }), { status: 410 }),
    )
    await expect(getMarketplaceKb('acme', 'docs')).rejects.toMatchObject({
      name: 'MarketplaceApiError',
      status: 410,
    })
  })

  it('throws MarketplaceApiError with status on 403 forbidden preview', async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ message: 'private' }), { status: 403 }),
    )
    let caught: unknown = null
    try {
      await previewMarketplaceKb('acme', 'docs')
    } catch (err) {
      caught = err
    }
    expect(caught).toBeInstanceOf(MarketplaceApiError)
    expect((caught as MarketplaceApiError).status).toBe(403)
  })

  it('re-import POST sends confirm:true to the workspace-scoped endpoint', async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({ kb_id: 'k', imported_from_revision_at: '2026-06-01T00:00:00Z' }),
        { status: 200 },
      ),
    )
    await reImportKnowledgeBase('org-1', 'ws-1', 'kb-1')
    const [url, init] = fetchMock.mock.calls[0]
    expect(String(url)).toMatch(
      /\/orgs\/org-1\/workspaces\/ws-1\/knowledge-bases\/kb-1\/re-import$/,
    )
    expect(init?.method).toBe('POST')
    expect(JSON.parse(String(init?.body))).toEqual({ confirm: true })
  })

  it('exposes the curated 7-entry SPDX allow-list (ADR-0006)', () => {
    expect([...SPDX_LICENSES]).toEqual([
      'CC0-1.0',
      'CC-BY-4.0',
      'CC-BY-SA-4.0',
      'CC-BY-NC-4.0',
      'MIT',
      'Apache-2.0',
      'GPL-3.0',
    ])
  })
})

