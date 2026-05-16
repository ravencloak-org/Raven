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
export type BillingCycle = 'monthly' | 'annual'

export interface Subscription {
  id: string
  org_id: string
  plan: PlanName
  status: string
  seat_count?: number
  current_period_start: string
  current_period_end: string
  created_at: string
  updated_at: string
}

export type PaymentIntentStatus =
  | 'requires_payment_method'
  | 'processing'
  | 'succeeded'
  | 'failed'
  | 'canceled'

export interface PaymentIntent {
  id: string
  client_secret: string
  amount: number
  currency: string
  status: PaymentIntentStatus
}

export interface UpdateSeatCountRequest {
  seat_count: number
}

export interface Plan {
  id: string
  name: PlanName
  price_per_seat_paise: number
  min_seats: number
}

export interface CreateSubscriptionRequest {
  plan_id: string
  seat_count: number
  billing_cycle: BillingCycle
}

export interface CreateSubscriptionResponse {
  id: string
  client_secret: string
  plan: PlanName
  seat_count: number
  billing_cycle: BillingCycle
  status: string
}

// --- API helpers ---

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

async function authFetch(path: string, init?: RequestInit): Promise<Response> {
  return fetch(API_BASE + path, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
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

export async function getPlans(): Promise<Plan[]> {
  const res = await authFetch('/billing/plans')
  if (!res.ok) throw new Error(`getPlans failed: ${res.status}`)
  return res.json()
}

export async function createSubscription(
  req: CreateSubscriptionRequest,
): Promise<CreateSubscriptionResponse> {
  const res = await authFetch('/billing/subscriptions', {
    method: 'POST',
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const body = await res.json().catch(() => null)
    const detail = body?.detail ?? body?.message ?? `status ${res.status}`
    throw new Error(detail)
  }
  return res.json()
}

export async function updateSeatCount(
  subscriptionId: string,
  seatCount: number,
): Promise<PaymentIntent> {
  const res = await authFetch(
    `/billing/subscriptions/${encodeURIComponent(subscriptionId)}/seats`,
    {
      method: 'PATCH',
      body: JSON.stringify({ seat_count: seatCount } satisfies UpdateSeatCountRequest),
    },
  )
  if (!res.ok) {
    const body = await res.json().catch(() => null)
    const detail = body?.detail ?? body?.message ?? `status ${res.status}`
    throw new Error(detail)
  }
  return res.json()
}

export async function cancelSubscription(subscriptionId: string): Promise<void> {
  const res = await authFetch(
    `/billing/subscriptions/${encodeURIComponent(subscriptionId)}`,
    { method: 'DELETE' },
  )
  if (!res.ok) {
    const body = await res.json().catch(() => null)
    const detail = body?.detail ?? body?.message ?? `status ${res.status}`
    throw new Error(detail)
  }
}
