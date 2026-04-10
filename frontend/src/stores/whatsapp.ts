import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  listPhoneNumbers,
  createPhoneNumber,
  deletePhoneNumber,
  listCalls,
  getCall,
  initiateCall,
  endCall,
  getBridge,
  type WhatsAppPhoneNumber,
  type WhatsAppCall,
  type WhatsAppBridge,
  type CreatePhoneNumberRequest,
  type InitiateCallRequest,
} from '../api/whatsapp'

export const useWhatsAppStore = defineStore('whatsapp', () => {
  // --- Phone numbers ---
  const phoneNumbers = ref<WhatsAppPhoneNumber[]>([])
  const phoneNumbersTotal = ref(0)
  const phoneNumbersLoading = ref(false)
  const phoneNumbersError = ref<string | null>(null)

  // --- Calls ---
  const calls = ref<WhatsAppCall[]>([])
  const callsTotal = ref(0)
  const callsLoading = ref(false)
  const callsError = ref<string | null>(null)

  // --- Active call ---
  const activeCall = ref<WhatsAppCall | null>(null)
  const activeBridge = ref<WhatsAppBridge | null>(null)

  // --- Actions: Phone Numbers ---

  async function fetchPhoneNumbers(orgId: string, limit = 20, offset = 0): Promise<void> {
    phoneNumbersLoading.value = true
    phoneNumbersError.value = null
    try {
      const res = await listPhoneNumbers(orgId, limit, offset)
      phoneNumbers.value = res.phone_numbers
      phoneNumbersTotal.value = res.total
    } catch (e) {
      phoneNumbersError.value = e instanceof Error ? e.message : String(e)
    } finally {
      phoneNumbersLoading.value = false
    }
  }

  async function addPhoneNumber(
    orgId: string,
    data: CreatePhoneNumberRequest,
  ): Promise<WhatsAppPhoneNumber> {
    const phone = await createPhoneNumber(orgId, data)
    phoneNumbers.value.push(phone)
    phoneNumbersTotal.value += 1
    return phone
  }

  async function removePhoneNumber(orgId: string, phoneId: string): Promise<void> {
    await deletePhoneNumber(orgId, phoneId)
    phoneNumbers.value = phoneNumbers.value.filter((p) => p.id !== phoneId)
    phoneNumbersTotal.value -= 1
  }

  // --- Actions: Calls ---

  async function fetchCalls(orgId: string, limit = 20, offset = 0): Promise<void> {
    callsLoading.value = true
    callsError.value = null
    try {
      const res = await listCalls(orgId, limit, offset)
      calls.value = res.calls
      callsTotal.value = res.total
    } catch (e) {
      callsError.value = e instanceof Error ? e.message : String(e)
    } finally {
      callsLoading.value = false
    }
  }

  async function startCall(orgId: string, data: InitiateCallRequest): Promise<WhatsAppCall> {
    const call = await initiateCall(orgId, data)
    calls.value.unshift(call)
    callsTotal.value += 1
    activeCall.value = call
    return call
  }

  async function refreshActiveCall(orgId: string, callId: string): Promise<void> {
    try {
      const call = await getCall(orgId, callId)
      activeCall.value = call
      // Sync the call in the list too
      const idx = calls.value.findIndex((c) => c.id === callId)
      if (idx !== -1) calls.value[idx] = call
    } catch {
      // silent — caller handles polling logic
    }
  }

  async function terminateCall(orgId: string, callId: string): Promise<WhatsAppCall> {
    const call = await endCall(orgId, callId)
    activeCall.value = call
    const idx = calls.value.findIndex((c) => c.id === callId)
    if (idx !== -1) calls.value[idx] = call
    return call
  }

  async function fetchBridge(orgId: string, callId: string): Promise<void> {
    try {
      activeBridge.value = await getBridge(orgId, callId)
    } catch {
      activeBridge.value = null
    }
  }

  function clearActiveCall(): void {
    activeCall.value = null
    activeBridge.value = null
  }

  return {
    // state
    phoneNumbers,
    phoneNumbersTotal,
    phoneNumbersLoading,
    phoneNumbersError,
    calls,
    callsTotal,
    callsLoading,
    callsError,
    activeCall,
    activeBridge,
    // actions
    fetchPhoneNumbers,
    addPhoneNumber,
    removePhoneNumber,
    fetchCalls,
    startCall,
    refreshActiveCall,
    terminateCall,
    fetchBridge,
    clearActiveCall,
  }
})
