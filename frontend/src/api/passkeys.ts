// Passkey API — thin wrapper around the Raven backend's per-user
// passkey label endpoints. Credential storage itself lives in the
// SuperTokens core and is proxied via /auth/webauthn/*; this module
// only deals with the metadata Raven owns (label, timestamps).
//
// Endpoints mirror the design in
// docs/superpowers/specs/2026-06-04-passkey-auth-design.md (Issue B):
//   GET    /api/v1/me/passkeys
//   PATCH  /api/v1/me/passkeys/:credential_id
//   DELETE /api/v1/me/passkeys/:credential_id

export interface Passkey {
  credential_id: string
  label: string
  created_at: string
  last_used_at: string | null
}

const API_BASE = () => import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

async function authFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(API_BASE() + path, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.message || data.error || `Request failed (${res.status})`)
  }
  // DELETE may return 204 No Content with no body; guard JSON parsing.
  if (res.status === 204) {
    return undefined as T
  }
  const text = await res.text()
  if (!text) {
    return undefined as T
  }
  return JSON.parse(text) as T
}

export async function listPasskeys(): Promise<Passkey[]> {
  const data = await authFetch<Passkey[] | { items: Passkey[] }>(`/me/passkeys`)
  return Array.isArray(data) ? data : (data?.items ?? [])
}

export async function relabelPasskey(
  credentialId: string,
  label: string,
): Promise<Passkey> {
  return authFetch<Passkey>(`/me/passkeys/${encodeURIComponent(credentialId)}`, {
    method: 'PATCH',
    body: JSON.stringify({ label }),
  })
}

export async function removePasskey(credentialId: string): Promise<void> {
  await authFetch<void>(`/me/passkeys/${encodeURIComponent(credentialId)}`, {
    method: 'DELETE',
  })
}

export { authFetch as _authFetch }
