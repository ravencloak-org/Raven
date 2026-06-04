import { defineStore } from 'pinia'
import { ref } from 'vue'
import { findIndex } from 'remeda'
import WebauthnRecipe from 'supertokens-web-js/recipe/webauthn'
import {
  listPasskeys,
  relabelPasskey as relabelPasskeyApi,
  removePasskey as removePasskeyApi,
  type Passkey,
} from '../api/passkeys'

// The SuperTokens v0.16 webauthn recipe ships overloaded static
// methods with many error variants (see
// supertokens-web-js/recipe/webauthn types). For the enrolment flow we
// only need a narrow contract: each call returns `{ status, ... }`,
// failure surfaces a non-OK status. Cast through a local minimal shape
// so the store doesn't have to thread every error union through the
// optimistic-update code, and so the tests can mock the same surface
// without re-declaring every SDK type.
type PasskeyEnrolment = {
  getRegisterOptions: (input: {
    userContext: Record<string, unknown>
  }) => Promise<{ status: string; registerOptions?: unknown }>
  createCredential: (input: {
    registrationOptions: unknown
    userContext: Record<string, unknown>
  }) => Promise<{ status: string; registrationResponse?: { id: string } }>
  registerCredential: (input: {
    response: unknown
    userContext: Record<string, unknown>
  }) => Promise<{ status: string }>
}

const Webauthn = WebauthnRecipe as unknown as PasskeyEnrolment

/**
 * usePasskeysStore — manages the current user's enrolled passkeys.
 *
 * The store keeps its `passkeys` array in lockstep with the backend.
 * Mutations are optimistic: the local state is updated immediately so
 * the UI feels instant, and the action rolls the state back to the
 * previous snapshot if the API call (or the WebAuthn browser ceremony)
 * fails. This matches the design in
 * docs/superpowers/specs/2026-06-04-passkey-auth-design.md (Issue B).
 */
export const usePasskeysStore = defineStore('passkeys', () => {
  const passkeys = ref<Passkey[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchPasskeys(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      passkeys.value = await listPasskeys()
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  /**
   * Run the WebAuthn enrolment ceremony and persist the user-supplied
   * label.
   *
   * Flow (per spec Issue B):
   *   1. SuperTokens core returns challenge + RP info via
   *      `Webauthn.getRegisterOptions()`.
   *   2. The browser opens the authenticator
   *      (`Webauthn.createCredential`) — biometric / PIN gate.
   *   3. The signed attestation is posted back to the core via
   *      `Webauthn.registerCredential`.
   *   4. The Raven backend stores the user-friendly label against the
   *      new credential ID.
   *
   * Optimistic: a placeholder row is pushed to local state as soon as
   * the credential ID is known. Any failure rolls back to the snapshot
   * taken before the ceremony started.
   */
  async function addPasskey(label: string): Promise<Passkey> {
    error.value = null
    const snapshot = passkeys.value.slice()
    try {
      const options = await Webauthn.getRegisterOptions({ userContext: {} })
      if (options.status !== 'OK' || options.registerOptions === undefined) {
        throw new Error(`Failed to start passkey enrolment: ${options.status}`)
      }
      const created = await Webauthn.createCredential({
        registrationOptions: options.registerOptions,
        userContext: {},
      })
      if (created.status !== 'OK' || !created.registrationResponse) {
        throw new Error(`Authenticator declined enrolment: ${created.status}`)
      }
      const registered = await Webauthn.registerCredential({
        response: created.registrationResponse,
        userContext: {},
      })
      if (registered.status !== 'OK') {
        throw new Error(`Passkey registration rejected: ${registered.status}`)
      }
      const credentialId = created.registrationResponse.id

      // Optimistic: render the new row immediately while the label
      // PATCH is in flight so the Settings UI doesn't flicker.
      const optimistic: Passkey = {
        credential_id: credentialId,
        label,
        created_at: new Date().toISOString(),
        last_used_at: null,
      }
      passkeys.value = [...snapshot, optimistic]

      const persisted = await relabelPasskeyApi(credentialId, label)
      const idx = findIndex(passkeys.value, (p) => p.credential_id === credentialId)
      if (idx !== -1) {
        passkeys.value[idx] = persisted
      }
      return persisted
    } catch (e) {
      passkeys.value = snapshot
      error.value = (e as Error).message
      throw e
    }
  }

  async function removePasskey(credentialId: string): Promise<void> {
    error.value = null
    const snapshot = passkeys.value.slice()
    passkeys.value = passkeys.value.filter(
      (p) => p.credential_id !== credentialId,
    )
    try {
      await removePasskeyApi(credentialId)
    } catch (e) {
      passkeys.value = snapshot
      error.value = (e as Error).message
      throw e
    }
  }

  async function relabelPasskey(
    credentialId: string,
    label: string,
  ): Promise<void> {
    error.value = null
    const snapshot = passkeys.value.slice()
    const idx = findIndex(passkeys.value, (p) => p.credential_id === credentialId)
    if (idx === -1) {
      // Nothing to update — surface a no-op error so the caller can
      // refetch instead of silently ignoring the request.
      throw new Error(`Passkey ${credentialId} not found in local state`)
    }
    passkeys.value[idx] = { ...passkeys.value[idx], label }
    try {
      const updated = await relabelPasskeyApi(credentialId, label)
      passkeys.value[idx] = updated
    } catch (e) {
      passkeys.value = snapshot
      error.value = (e as Error).message
      throw e
    }
  }

  return {
    passkeys,
    loading,
    error,
    fetchPasskeys,
    addPasskey,
    removePasskey,
    relabelPasskey,
  }
})
