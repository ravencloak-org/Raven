import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePasskeysStore } from './passkeys'
import * as passkeysApi from '../api/passkeys'
import Webauthn from 'supertokens-web-js/recipe/webauthn'
import type { Passkey } from '../api/passkeys'

vi.mock('../api/passkeys')
vi.mock('supertokens-web-js/recipe/webauthn', () => ({
  default: {
    getRegisterOptions: vi.fn(),
    createCredential: vi.fn(),
    registerCredential: vi.fn(),
  },
}))

const passkeyA: Passkey = {
  credential_id: 'cred-aaaa',
  label: 'MacBook Touch ID',
  created_at: '2026-06-01T10:00:00Z',
  last_used_at: null,
}

const passkeyB: Passkey = {
  credential_id: 'cred-bbbb',
  label: 'iPhone',
  created_at: '2026-06-02T10:00:00Z',
  last_used_at: '2026-06-03T11:00:00Z',
}

function okGetRegisterOptions() {
  return {
    status: 'OK' as const,
    registerOptions: { challenge: 'chal', timeout: 60000 },
  }
}

function okCreateCredential(credentialId: string) {
  return {
    status: 'OK' as const,
    registrationResponse: { id: credentialId, type: 'public-key' },
  }
}

function okRegisterCredential() {
  return { status: 'OK' as const }
}

describe('usePasskeysStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('fetchPasskeys', () => {
    it('populates passkeys on success', async () => {
      vi.mocked(passkeysApi.listPasskeys).mockResolvedValue([passkeyA, passkeyB])
      const store = usePasskeysStore()
      await store.fetchPasskeys()
      expect(store.passkeys).toHaveLength(2)
      expect(store.passkeys[0].credential_id).toBe('cred-aaaa')
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('sets error and leaves passkeys empty on failure', async () => {
      vi.mocked(passkeysApi.listPasskeys).mockRejectedValue(new Error('Network down'))
      const store = usePasskeysStore()
      await store.fetchPasskeys()
      expect(store.error).toBe('Network down')
      expect(store.passkeys).toHaveLength(0)
      expect(store.loading).toBe(false)
    })
  })

  describe('addPasskey', () => {
    it('runs the full WebAuthn ceremony and appends the labeled passkey', async () => {
      vi.mocked(Webauthn.getRegisterOptions).mockResolvedValue(
        okGetRegisterOptions() as never,
      )
      vi.mocked(Webauthn.createCredential).mockResolvedValue(
        okCreateCredential('cred-new') as never,
      )
      vi.mocked(Webauthn.registerCredential).mockResolvedValue(
        okRegisterCredential() as never,
      )
      const persisted: Passkey = {
        credential_id: 'cred-new',
        label: 'Work Laptop',
        created_at: '2026-06-04T09:00:00Z',
        last_used_at: null,
      }
      vi.mocked(passkeysApi.relabelPasskey).mockResolvedValue(persisted)

      const store = usePasskeysStore()
      const result = await store.addPasskey('Work Laptop')

      expect(Webauthn.getRegisterOptions).toHaveBeenCalledOnce()
      expect(Webauthn.createCredential).toHaveBeenCalledOnce()
      expect(Webauthn.registerCredential).toHaveBeenCalledOnce()
      expect(passkeysApi.relabelPasskey).toHaveBeenCalledWith('cred-new', 'Work Laptop')
      expect(result).toEqual(persisted)
      expect(store.passkeys).toHaveLength(1)
      expect(store.passkeys[0]).toEqual(persisted)
      expect(store.error).toBeNull()
    })

    it('rolls back on getRegisterOptions failure (no API call)', async () => {
      vi.mocked(passkeysApi.listPasskeys).mockResolvedValue([passkeyA])
      vi.mocked(Webauthn.getRegisterOptions).mockRejectedValue(
        new Error('Core unreachable'),
      )

      const store = usePasskeysStore()
      await store.fetchPasskeys()
      const before = store.passkeys.slice()

      await expect(store.addPasskey('Phone')).rejects.toThrow('Core unreachable')
      expect(store.passkeys).toEqual(before)
      expect(passkeysApi.relabelPasskey).not.toHaveBeenCalled()
      expect(store.error).toBe('Core unreachable')
    })

    it('rolls back when the authenticator declines (createCredential non-OK)', async () => {
      vi.mocked(passkeysApi.listPasskeys).mockResolvedValue([passkeyA])
      vi.mocked(Webauthn.getRegisterOptions).mockResolvedValue(
        okGetRegisterOptions() as never,
      )
      vi.mocked(Webauthn.createCredential).mockResolvedValue({
        status: 'AUTHENTICATOR_ALREADY_REGISTERED',
      } as never)

      const store = usePasskeysStore()
      await store.fetchPasskeys()
      const before = store.passkeys.slice()

      await expect(store.addPasskey('Phone')).rejects.toThrow(
        /Authenticator declined enrolment/,
      )
      expect(store.passkeys).toEqual(before)
      expect(passkeysApi.relabelPasskey).not.toHaveBeenCalled()
      expect(store.error).toMatch(/Authenticator declined enrolment/)
    })

    it('rolls back when the label PATCH API fails after registration', async () => {
      vi.mocked(passkeysApi.listPasskeys).mockResolvedValue([passkeyA])
      vi.mocked(Webauthn.getRegisterOptions).mockResolvedValue(
        okGetRegisterOptions() as never,
      )
      vi.mocked(Webauthn.createCredential).mockResolvedValue(
        okCreateCredential('cred-zzz') as never,
      )
      vi.mocked(Webauthn.registerCredential).mockResolvedValue(
        okRegisterCredential() as never,
      )
      vi.mocked(passkeysApi.relabelPasskey).mockRejectedValue(
        new Error('Label save failed'),
      )

      const store = usePasskeysStore()
      await store.fetchPasskeys()
      const before = store.passkeys.slice()

      await expect(store.addPasskey('Tablet')).rejects.toThrow('Label save failed')
      expect(store.passkeys).toEqual(before)
      expect(store.error).toBe('Label save failed')
    })
  })

  describe('removePasskey', () => {
    it('removes the passkey on success', async () => {
      vi.mocked(passkeysApi.listPasskeys).mockResolvedValue([passkeyA, passkeyB])
      vi.mocked(passkeysApi.removePasskey).mockResolvedValue(undefined)

      const store = usePasskeysStore()
      await store.fetchPasskeys()
      await store.removePasskey('cred-aaaa')

      expect(store.passkeys).toHaveLength(1)
      expect(store.passkeys[0].credential_id).toBe('cred-bbbb')
      expect(passkeysApi.removePasskey).toHaveBeenCalledWith('cred-aaaa')
    })

    it('rolls back on API failure', async () => {
      vi.mocked(passkeysApi.listPasskeys).mockResolvedValue([passkeyA, passkeyB])
      vi.mocked(passkeysApi.removePasskey).mockRejectedValue(new Error('500 Server'))

      const store = usePasskeysStore()
      await store.fetchPasskeys()
      const before = store.passkeys.slice()

      await expect(store.removePasskey('cred-aaaa')).rejects.toThrow('500 Server')
      expect(store.passkeys).toEqual(before)
      expect(store.error).toBe('500 Server')
    })
  })

  describe('relabelPasskey', () => {
    it('updates the label in place on success', async () => {
      vi.mocked(passkeysApi.listPasskeys).mockResolvedValue([passkeyA, passkeyB])
      vi.mocked(passkeysApi.relabelPasskey).mockResolvedValue({
        ...passkeyA,
        label: 'Renamed Mac',
      })

      const store = usePasskeysStore()
      await store.fetchPasskeys()
      await store.relabelPasskey('cred-aaaa', 'Renamed Mac')

      expect(store.passkeys[0].label).toBe('Renamed Mac')
      // The other passkey is untouched.
      expect(store.passkeys[1]).toEqual(passkeyB)
    })

    it('rolls back on API failure', async () => {
      vi.mocked(passkeysApi.listPasskeys).mockResolvedValue([passkeyA, passkeyB])
      vi.mocked(passkeysApi.relabelPasskey).mockRejectedValue(
        new Error('Conflict'),
      )

      const store = usePasskeysStore()
      await store.fetchPasskeys()
      const before = store.passkeys.map((p) => ({ ...p }))

      await expect(
        store.relabelPasskey('cred-aaaa', 'Renamed Mac'),
      ).rejects.toThrow('Conflict')
      expect(store.passkeys).toEqual(before)
      expect(store.error).toBe('Conflict')
    })

    it('throws when the credential is not in local state', async () => {
      const store = usePasskeysStore()
      await expect(
        store.relabelPasskey('cred-missing', 'X'),
      ).rejects.toThrow(/not found in local state/)
      expect(passkeysApi.relabelPasskey).not.toHaveBeenCalled()
    })
  })
})
