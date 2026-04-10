import { defineStore } from 'pinia'
import { ref } from 'vue'
import { filter } from 'remeda'
import {
  listPhoneNumbers,
  createPhoneNumber,
  deletePhoneNumber,
  listCalls,
  getCall,
  initiateCall,
  updateCallState,
  getBridgeStatus,
  listActiveBridges,
  type WhatsAppPhoneNumber,
  type WhatsAppCall,
  type WhatsAppBridge,
  type CreatePhoneNumberRequest,
  type InitiateCallRequest,
  type CallState,
} from '../api/whatsapp'

export const useWhatsAppStore = defineStore('whatsapp', () => {
  // --- Phone numbers ---
  const phoneNumbers = ref<WhatsAppPhoneNumber[]>([])
  const phoneTotal = ref(0)

  // --- Calls ---
  const calls = ref<WhatsAppCall[]>([])
  const callTotal = ref(0)
  const activeCall = ref<WhatsAppCall | null>(null)

  // --- Bridges ---
  const bridges = ref<WhatsAppBridge[]>([])
  const currentBridge = ref<WhatsAppBridge | null>(null)

  // --- Shared state ---
  const loading = ref(false)
  const error = ref<string | null>(null)

  // --- Phone number actions ---

  async function fetchPhoneNumbers(orgId: string, offset = 0, limit = 20) {
    loading.value = true
    error.value = null
    try {
      const res = await listPhoneNumbers(orgId, offset, limit)
      phoneNumbers.value = res.phone_numbers
      phoneTotal.value = res.total
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function addPhoneNumber(
    orgId: string,
    req: CreatePhoneNumberRequest,
  ): Promise<WhatsAppPhoneNumber> {
    const phone = await createPhoneNumber(orgId, req)
    phoneNumbers.value.push(phone)
    phoneTotal.value += 1
    return phone
  }

  async function removePhoneNumber(orgId: string, phoneId: string): Promise<void> {
    await deletePhoneNumber(orgId, phoneId)
    phoneNumbers.value = filter(phoneNumbers.value, (p) => p.id !== phoneId)
    phoneTotal.value -= 1
  }

  // --- Call actions ---

  async function fetchCalls(orgId: string, offset = 0, limit = 20) {
    loading.value = true
    error.value = null
    try {
      const res = await listCalls(orgId, offset, limit)
      calls.value = res.calls
      callTotal.value = res.total
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function fetchCall(orgId: string, callId: string) {
    loading.value = true
    error.value = null
    try {
      activeCall.value = await getCall(orgId, callId)
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function startCall(orgId: string, req: InitiateCallRequest): Promise<WhatsAppCall> {
    const call = await initiateCall(orgId, req)
    calls.value.unshift(call)
    callTotal.value += 1
    activeCall.value = call
    return call
  }

  async function endCall(orgId: string, callId: string): Promise<WhatsAppCall> {
    const call = await updateCallState(orgId, callId, 'ended' as CallState)
    activeCall.value = call
    // Update the call in the calls list
    const idx = calls.value.findIndex((c) => c.id === callId)
    if (idx !== -1) {
      calls.value[idx] = call
    }
    return call
  }

  // --- Bridge actions ---

  async function fetchBridgeStatus(orgId: string, callId: string) {
    try {
      const res = await getBridgeStatus(orgId, callId)
      currentBridge.value = res.bridge
    } catch {
      currentBridge.value = null
    }
  }

  async function fetchActiveBridges(orgId: string) {
    try {
      bridges.value = await listActiveBridges(orgId)
    } catch (e) {
      error.value = (e as Error).message
    }
  }

  function clearActiveCall() {
    activeCall.value = null
    currentBridge.value = null
  }

  return {
    // Phone numbers
    phoneNumbers,
    phoneTotal,
    fetchPhoneNumbers,
    addPhoneNumber,
    removePhoneNumber,

    // Calls
    calls,
    callTotal,
    activeCall,
    fetchCalls,
    fetchCall,
    startCall,
    endCall,

    // Bridges
    bridges,
    currentBridge,
    fetchBridgeStatus,
    fetchActiveBridges,

    // Shared
    loading,
    error,
    clearActiveCall,
  }
})
