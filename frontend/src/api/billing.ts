import { authFetch } from './utils'

export interface Plan {
  id: string
  name: string
  price_monthly: number
  currency: string
  max_users: number
  max_workspaces: number
  max_knowledge_bases: number
  max_storage_gb: number
  max_voice_sessions: number
  max_voice_minutes: number
}

export interface Subscription {
  id: string
  org_id: string
  plan_id: string
  status: 'active' | 'cancelled' | 'past_due' | 'trialing'
  current_period_end: string
  payment_method_id?: string
}

export interface UsageResponse {
  knowledge_bases_used: number
  knowledge_bases_limit: number
  seats_used: number
  seats_limit: number
  voice_minutes_used: number
  voice_minutes_limit: number
  concurrent_sessions: number
  concurrent_sessions_limit: number
}

export interface CreatePaymentIntentResponse {
  client_secret: string
  payment_intent_id: string
}

export async function getPlans(): Promise<Plan[]> {
  const res = await authFetch('/billing/plans')
  if (!res.ok) throw new Error(`getPlans failed: ${res.status}`)
  return res.json()
}

export async function getUsage(): Promise<UsageResponse> {
  const res = await authFetch('/billing/usage')
  if (!res.ok) throw new Error(`getUsage failed: ${res.status}`)
  return res.json()
}

export async function createPaymentIntent(planId: string): Promise<CreatePaymentIntentResponse> {
  const res = await authFetch('/billing/payment-intents', {
    method: 'POST',
    body: JSON.stringify({ plan_id: planId }),
  })
  if (!res.ok) throw new Error(`createPaymentIntent failed: ${res.status}`)
  return res.json()
}

export async function createSubscription(
  planId: string,
  paymentMethodId: string,
): Promise<Subscription> {
  const res = await authFetch('/billing/subscriptions', {
    method: 'POST',
    body: JSON.stringify({ plan_id: planId, payment_method_id: paymentMethodId }),
  })
  if (!res.ok) throw new Error(`createSubscription failed: ${res.status}`)
  return res.json()
}

export async function cancelSubscription(subscriptionId: string): Promise<void> {
  const res = await authFetch(`/billing/subscriptions/${subscriptionId}`, {
    method: 'DELETE',
  })
  if (!res.ok && res.status !== 204) throw new Error(`cancelSubscription failed: ${res.status}`)
}
