export interface Org {
  id: string
  name: string
  slug: string
  status: 'active' | 'deactivated'
  settings: Record<string, unknown>
  created_at: string
  updated_at: string
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

export async function getOrg(orgId: string): Promise<Org> {
  const res = await authFetch(`/orgs/${orgId}`)
  if (!res.ok) throw new Error(`getOrg failed: ${res.status}`)
  return res.json()
}

export async function createOrg(name: string): Promise<Org> {
  const res = await authFetch('/orgs', {
    method: 'POST',
    body: JSON.stringify({ name }),
  })
  if (!res.ok) throw new Error(`createOrg failed: ${res.status}`)
  return res.json()
}
