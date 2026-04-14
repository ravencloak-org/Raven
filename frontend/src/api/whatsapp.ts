// --- Phone number types ---

export interface WhatsAppPhoneNumber {
  id: string
  org_id: string
  phone_number: string
  display_name: string
  waba_id: string
  verified: boolean
  created_at: string
  updated_at: string
}

export interface WhatsAppPhoneNumberListResponse {
  phone_numbers: WhatsAppPhoneNumber[]
  total: number
  limit: number
  offset: number
}

export interface CreatePhoneNumberRequest {
  phone_number: string
  display_name?: string
  waba_id: string
}

// --- Call types ---

export type CallDirection = 'inbound' | 'outbound'
export type CallState = 'ringing' | 'connected' | 'ended'

export interface WhatsAppCall {
  id: string
  org_id: string
  call_id: string
  phone_number_id: string
  direction: CallDirection
  state: CallState
  caller: string
  callee: string
  from?: string
  to?: string
  started_at?: string
  ended_at?: string
  duration_seconds?: number
  created_at: string
  updated_at: string
}

export interface WhatsAppCallListResponse {
  calls: WhatsAppCall[]
  total: number
  limit: number
  offset: number
}

export interface InitiateCallRequest {
  phone_number_id: string
  callee: string
}

// --- Bridge types ---

export type BridgeState = 'initializing' | 'active' | 'failed' | 'closed'

export interface WhatsAppBridge {
  id: string
  org_id: string
  call_id: string
  livekit_room: string
  bridge_state: BridgeState
  voice_session_id?: string
  metadata?: string
  created_at: string
  updated_at: string
  closed_at?: string
}

export interface BridgeStatusResponse {
  bridge: WhatsAppBridge
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

// --- Phone number endpoints ---

export async function listPhoneNumbers(
  orgId: string,
  offset = 0,
  limit = 20,
): Promise<WhatsAppPhoneNumberListResponse> {
  const res = await authFetch(
    `/orgs/${orgId}/whatsapp/phone-numbers?offset=${offset}&limit=${limit}`,
  )
  if (!res.ok) throw new Error(`listPhoneNumbers failed: ${res.status}`)
  return res.json()
}

export async function createPhoneNumber(
  orgId: string,
  req: CreatePhoneNumberRequest,
): Promise<WhatsAppPhoneNumber> {
  const res = await authFetch(`/orgs/${orgId}/whatsapp/phone-numbers`, {
    method: 'POST',
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(`createPhoneNumber failed: ${res.status}`)
  return res.json()
}

export async function deletePhoneNumber(orgId: string, phoneId: string): Promise<void> {
  const res = await authFetch(`/orgs/${orgId}/whatsapp/phone-numbers/${phoneId}`, {
    method: 'DELETE',
  })
  if (!res.ok) throw new Error(`deletePhoneNumber failed: ${res.status}`)
}

// --- Call endpoints ---

export async function listCalls(
  orgId: string,
  offset = 0,
  limit = 20,
): Promise<WhatsAppCallListResponse> {
  const res = await authFetch(`/orgs/${orgId}/whatsapp/calls?offset=${offset}&limit=${limit}`)
  if (!res.ok) throw new Error(`listCalls failed: ${res.status}`)
  return res.json()
}

export async function getCall(orgId: string, callId: string): Promise<WhatsAppCall> {
  const res = await authFetch(`/orgs/${orgId}/whatsapp/calls/${callId}`)
  if (!res.ok) throw new Error(`getCall failed: ${res.status}`)
  return res.json()
}

export async function initiateCall(
  orgId: string,
  req: InitiateCallRequest,
): Promise<WhatsAppCall> {
  const res = await authFetch(`/orgs/${orgId}/whatsapp/calls`, {
    method: 'POST',
    body: JSON.stringify(req),
  })
  if (!res.ok) throw new Error(`initiateCall failed: ${res.status}`)
  return res.json()
}

export async function updateCallState(
  orgId: string,
  callId: string,
  state: CallState,
): Promise<WhatsAppCall> {
  const res = await authFetch(`/orgs/${orgId}/whatsapp/calls/${callId}`, {
    method: 'PATCH',
    body: JSON.stringify({ state }),
  })
  if (!res.ok) throw new Error(`updateCallState failed: ${res.status}`)
  return res.json()
}

// --- Bridge endpoints ---

export async function getBridgeStatus(
  orgId: string,
  callId: string,
): Promise<BridgeStatusResponse> {
  const res = await authFetch(`/orgs/${orgId}/whatsapp/calls/${callId}/bridge`)
  if (!res.ok) throw new Error(`getBridgeStatus failed: ${res.status}`)
  return res.json()
}

export async function listActiveBridges(orgId: string): Promise<WhatsAppBridge[]> {
  const res = await authFetch(`/orgs/${orgId}/whatsapp/bridges`)
  if (!res.ok) throw new Error(`listActiveBridges failed: ${res.status}`)
  return res.json()
}
