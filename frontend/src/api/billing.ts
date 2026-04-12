import { useAuthStore } from '../stores/auth'

// --- Types ---

export interface UsageMetric {
  current: number
  max: number
}

export interface BillingUsage {
  knowledge_base_kbs: UsageMetric
  seats: UsageMetric
  voice_minutes: UsageMetric
  api_calls: UsageMetric
}

export type PlanName = 'free' | 'pro' | 'enterprise'

export interface Subscription {
  id: string
  org_id: string
  plan: PlanName
  status: string
  current_period_start: string
  current_period_end: string
  created_at: string
  updated_at: string
}

// --- API helpers ---

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

async function authFetch(path: string, init?: RequestInit): Promise<Response> {
  const auth = useAuthStore()
  return fetch(API_BASE + path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${auth.accessToken ?? ''}`,
      ...init?.headers,
    },
  })
}

// --- Billing endpoints ---

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export async function getUsage(_orgId: string): Promise<BillingUsage> {
  const res = await authFetch('/billing/usage')
  if (res.status === 402) {
    throw Object.assign(new Error('Payment required'), { status: 402 })
  }
  if (!res.ok) throw new Error(`getUsage failed: ${res.status}`)
  return res.json()
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export async function getSubscription(_orgId: string): Promise<Subscription> {
  const res = await authFetch('/billing/subscriptions/current')
  if (res.status === 402) {
    throw Object.assign(new Error('Payment required'), { status: 402 })
  }
  if (!res.ok) throw new Error(`getSubscription failed: ${res.status}`)
  return res.json()
}

export async function cancelSubscription(subscriptionId: string): Promise<void> {
  const res = await authFetch(
    `/billing/subscriptions/${encodeURIComponent(subscriptionId)}`,
    { method: 'DELETE' },
  )
  if (!res.ok) throw new Error(`cancelSubscription failed: ${res.status}`)
}
