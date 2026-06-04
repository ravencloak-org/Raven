import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import MarketplaceListView from '../../pages/marketplace/MarketplaceListView.vue'

// IntersectionObserver isn't available in happy-dom — stub before mount.
class FakeIO {
  observe = vi.fn()
  disconnect = vi.fn()
  unobserve = vi.fn()
}
;(globalThis as unknown as { IntersectionObserver: typeof FakeIO }).IntersectionObserver = FakeIO

describe('MarketplaceListView filter behaviour', () => {
  const originalFetch = globalThis.fetch
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    setActivePinia(createPinia())
    fetchMock = vi.fn().mockImplementation(
      () =>
        Promise.resolve(
          new Response(JSON.stringify({ items: [] }), { status: 200 }),
        ),
    )
    globalThis.fetch = fetchMock as unknown as typeof fetch
  })
  afterEach(() => {
    globalThis.fetch = originalFetch
    vi.restoreAllMocks()
  })

  it('toggles a license filter and re-queries with the SPDX id', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div/>' } }],
    })
    const wrapper = mount(MarketplaceListView, {
      global: { plugins: [router] },
    })
    await flushPromises()
    fetchMock.mockClear()

    const mitButton = wrapper.get('[data-test="marketplace-license-filter-MIT"]')
    await mitButton.trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalled()
    const url = String(fetchMock.mock.calls[0][0])
    expect(url).toContain('license=MIT')

    // Toggling again removes the filter
    fetchMock.mockClear()
    await mitButton.trigger('click')
    await flushPromises()
    const url2 = String(fetchMock.mock.calls[0][0])
    expect(url2).not.toContain('license=MIT')
  })

  it('updates sort dropdown drives a refetch with the new sort param', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div/>' } }],
    })
    const wrapper = mount(MarketplaceListView, {
      global: { plugins: [router] },
    })
    await flushPromises()
    fetchMock.mockClear()

    await wrapper.get('[data-test="marketplace-sort"]').setValue('most_imported')
    await flushPromises()

    const url = String(fetchMock.mock.calls[0][0])
    expect(url).toContain('sort=most_imported')
  })
})
