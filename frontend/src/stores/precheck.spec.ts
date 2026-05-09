import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePrecheckStore, ramTier } from './precheck'

describe('ramTier', () => {
  it('classifies under 8 GB as floor', () => {
    expect(ramTier(4)).toBe('floor')
    expect(ramTier(7)).toBe('floor')
  })
  it('classifies 8-11 as low', () => {
    expect(ramTier(8)).toBe('low')
    expect(ramTier(11)).toBe('low')
  })
  it('classifies 12-15 as mid', () => {
    expect(ramTier(12)).toBe('mid')
    expect(ramTier(15)).toBe('mid')
  })
  it('classifies 16+ as high', () => {
    expect(ramTier(16)).toBe('high')
    expect(ramTier(64)).toBe('high')
  })
})

describe('usePrecheckStore', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('starts unknown', () => {
    const store = usePrecheckStore()
    expect(store.result).toBeNull()
    expect(store.tier).toBe('unknown')
  })

  it('exposes tier from applied result', () => {
    const store = usePrecheckStore()
    store.applyResult({
      ram_gb: 16,
      free_disk_gb: 100,
      cpu_cores: 8,
      ok: true,
      warnings: [],
    })
    expect(store.result?.ram_gb).toBe(16)
    expect(store.tier).toBe('high')
  })
})
