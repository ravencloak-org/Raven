export interface Workspace {
  id: string
  org_id: string
  name: string
  slug: string
  settings: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface WorkspaceListResponse {
  items: Workspace[]
  total: number
  offset: number
  limit: number
}

export interface WorkspaceMember {
  user_id: string
  role: string
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

export async function listWorkspaces(
  orgId: string,
  offset = 0,
  limit = 20,
): Promise<WorkspaceListResponse> {
  const res = await authFetch(
    `/orgs/${orgId}/workspaces?offset=${offset}&limit=${limit}`,
  )
  if (!res.ok) throw new Error(`listWorkspaces failed: ${res.status}`)
  return res.json()
}

export async function getWorkspace(orgId: string, wsId: string): Promise<Workspace> {
  const res = await authFetch(`/orgs/${orgId}/workspaces/${wsId}`)
  if (!res.ok) throw new Error(`getWorkspace failed: ${res.status}`)
  return res.json()
}

export async function createWorkspace(orgId: string, name: string): Promise<Workspace> {
  const res = await authFetch(`/orgs/${orgId}/workspaces`, {
    method: 'POST',
    body: JSON.stringify({ name }),
  })
  if (!res.ok) throw new Error(`createWorkspace failed: ${res.status}`)
  return res.json()
}

export async function deleteWorkspace(orgId: string, wsId: string): Promise<void> {
  const res = await authFetch(`/orgs/${orgId}/workspaces/${wsId}`, {
    method: 'DELETE',
  })
  if (!res.ok) throw new Error(`deleteWorkspace failed: ${res.status}`)
}

export async function addMember(
  orgId: string,
  wsId: string,
  userId: string,
  role: string,
): Promise<WorkspaceMember> {
  const res = await authFetch(`/orgs/${orgId}/workspaces/${wsId}/members`, {
    method: 'POST',
    body: JSON.stringify({ user_id: userId, role }),
  })
  if (!res.ok) throw new Error(`addMember failed: ${res.status}`)
  return res.json()
}
