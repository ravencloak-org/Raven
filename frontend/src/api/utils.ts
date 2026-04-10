import { useAuthStore } from '../stores/auth'

export const API_BASE = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

/**
 * Authenticated fetch wrapper that intercepts 402 Payment Required responses
 * and notifies the billing store so the upgrade banner can be shown.
 */
export async function authFetch(path: string, init?: RequestInit): Promise<Response> {
  const auth = useAuthStore()
  const res = await fetch(API_BASE + path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${auth.accessToken ?? ''}`,
      ...init?.headers,
    },
  })

  if (res.status === 402) {
    try {
      const body = await res.clone().json()
      // Lazy import to avoid circular dependency; billing store depends on this util
      const { useBillingStore } = await import('../stores/billing')
      const billing = useBillingStore()
      billing.flagQuotaExceeded(body.message ?? 'Quota exceeded. Please upgrade your plan.')
    } catch {
      // Silently ignore parse errors so the caller still gets the Response
    }
  }

  return res
}
