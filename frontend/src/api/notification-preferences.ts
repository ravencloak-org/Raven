/**
 * Client for M9 email-summary preference endpoints (#257).
 *
 * Two toggles live here:
 *   1. User-level:       PUT /me/notification-preferences/:workspace_id
 *   2. Workspace-admin:  PUT /orgs/:org_id/workspaces/:ws_id/notification-preferences
 *
 * Both expect `{ email_summaries_enabled: boolean }` and return the same shape.
 */

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

export interface EmailSummaryPreference {
  email_summaries_enabled: boolean
}

export async function setUserEmailSummaries(
  workspaceId: string,
  enabled: boolean,
): Promise<EmailSummaryPreference> {
  const res = await authFetch(
    `/me/notification-preferences/${encodeURIComponent(workspaceId)}`,
    {
      method: 'PUT',
      body: JSON.stringify({ email_summaries_enabled: enabled }),
    },
  )
  if (!res.ok) {
    throw new Error(`set user pref failed: ${res.status}`)
  }
  return res.json() as Promise<EmailSummaryPreference>
}

export async function setWorkspaceEmailSummaries(
  orgId: string,
  workspaceId: string,
  enabled: boolean,
): Promise<EmailSummaryPreference> {
  const res = await authFetch(
    `/orgs/${encodeURIComponent(orgId)}/workspaces/${encodeURIComponent(workspaceId)}/notification-preferences`,
    {
      method: 'PUT',
      body: JSON.stringify({ email_summaries_enabled: enabled }),
    },
  )
  if (!res.ok) {
    throw new Error(`set workspace pref failed: ${res.status}`)
  }
  return res.json() as Promise<EmailSummaryPreference>
}
