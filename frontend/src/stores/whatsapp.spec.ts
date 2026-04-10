import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useWhatsAppStore } from './whatsapp'
import type {
  WhatsAppPhoneNumber,
  WhatsAppPhoneNumberListResponse,
  WhatsAppCall,
  WhatsAppCallListResponse,
  WhatsAppBridge,
  BridgeStatusResponse,
} from '../api/whatsapp'

vi.mock('../api/whatsapp', () => ({
  listPhoneNumbers: vi.fn(),
  createPhoneNumber: vi.fn(),
  deletePhoneNumber: vi.fn(),
  listCalls: vi.fn(),
  getCall: vi.fn(),
  initiateCall: vi.fn(),
  updateCallState: vi.fn(),
  getBridgeStatus: vi.fn(),
  listActiveBridges: vi.fn(),
}))

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
} from '../api/whatsapp'

const mockedListPhoneNumbers = vi.mocked(listPhoneNumbers)
const mockedCreatePhoneNumber = vi.mocked(createPhoneNumber)
const mockedDeletePhoneNumber = vi.mocked(deletePhoneNumber)
const mockedListCalls = vi.mocked(listCalls)
const mockedGetCall = vi.mocked(getCall)
const mockedInitiateCall = vi.mocked(initiateCall)
const mockedUpdateCallState = vi.mocked(updateCallState)
const mockedGetBridgeStatus = vi.mocked(getBridgeStatus)
const mockedListActiveBridges = vi.mocked(listActiveBridges)

const ORG_ID = 'org-1'

function fakePhone(overrides: Partial<WhatsAppPhoneNumber> = {}): WhatsAppPhoneNumber {
  return {
    id: 'phone-1',
    org_id: ORG_ID,
    phone_number: '+1234567890',
    display_name: 'Test Phone',
    waba_id: 'waba-1',
    verified: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function fakeCall(overrides: Partial<WhatsAppCall> = {}): WhatsAppCall {
  return {
    id: 'call-1',
    org_id: ORG_ID,
    call_id: 'meta-call-1',
    phone_number_id: 'phone-1',
    direction: 'outbound',
    state: 'ringing',
    caller: '+1234567890',
    callee: '+0987654321',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function fakeBridge(overrides: Partial<WhatsAppBridge> = {}): WhatsAppBridge {
  return {
    id: 'bridge-1',
    org_id: ORG_ID,
    call_id: 'call-1',
    livekit_room: 'room-abc',
    bridge_state: 'active',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('useWhatsAppStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  // --- Phone numbers ---

  describe('fetchPhoneNumbers', () => {
    it('populates phone numbers and total on success', async () => {
      const phone = fakePhone()
      const response: WhatsAppPhoneNumberListResponse = {
        phone_numbers: [phone],
        total: 1,
        limit: 20,
        offset: 0,
      }
      mockedListPhoneNumbers.mockResolvedValue(response)

      const store = useWhatsAppStore()
      await store.fetchPhoneNumbers(ORG_ID)

      expect(mockedListPhoneNumbers).toHaveBeenCalledWith(ORG_ID, 0, 20)
      expect(store.phoneNumbers).toEqual([phone])
      expect(store.phoneTotal).toBe(1)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('sets error on failure', async () => {
      mockedListPhoneNumbers.mockRejectedValue(new Error('listPhoneNumbers failed: 500'))

      const store = useWhatsAppStore()
      await store.fetchPhoneNumbers(ORG_ID)

      expect(store.phoneNumbers).toEqual([])
      expect(store.error).toBe('listPhoneNumbers failed: 500')
      expect(store.loading).toBe(false)
    })

    it('passes offset and limit through', async () => {
      mockedListPhoneNumbers.mockResolvedValue({
        phone_numbers: [],
        total: 0,
        limit: 5,
        offset: 10,
      })

      const store = useWhatsAppStore()
      await store.fetchPhoneNumbers(ORG_ID, 10, 5)

      expect(mockedListPhoneNumbers).toHaveBeenCalledWith(ORG_ID, 10, 5)
    })
  })

  describe('addPhoneNumber', () => {
    it('appends new phone number and increments total', async () => {
      const phone = fakePhone({ id: 'phone-new' })
      mockedCreatePhoneNumber.mockResolvedValue(phone)

      const store = useWhatsAppStore()
      store.phoneTotal = 3

      const result = await store.addPhoneNumber(ORG_ID, {
        phone_number: '+1234567890',
        waba_id: 'waba-1',
      })

      expect(mockedCreatePhoneNumber).toHaveBeenCalledWith(ORG_ID, {
        phone_number: '+1234567890',
        waba_id: 'waba-1',
      })
      expect(result).toEqual(phone)
      expect(store.phoneNumbers).toContainEqual(phone)
      expect(store.phoneTotal).toBe(4)
    })

    it('propagates API errors', async () => {
      mockedCreatePhoneNumber.mockRejectedValue(new Error('createPhoneNumber failed: 400'))

      const store = useWhatsAppStore()

      await expect(
        store.addPhoneNumber(ORG_ID, { phone_number: '', waba_id: '' }),
      ).rejects.toThrow('createPhoneNumber failed: 400')
    })
  })

  describe('removePhoneNumber', () => {
    it('removes phone from list and decrements total', async () => {
      mockedDeletePhoneNumber.mockResolvedValue(undefined)

      const store = useWhatsAppStore()
      const phone = fakePhone()
      store.phoneNumbers = [phone]
      store.phoneTotal = 1

      await store.removePhoneNumber(ORG_ID, 'phone-1')

      expect(mockedDeletePhoneNumber).toHaveBeenCalledWith(ORG_ID, 'phone-1')
      expect(store.phoneNumbers).toEqual([])
      expect(store.phoneTotal).toBe(0)
    })

    it('propagates API errors', async () => {
      mockedDeletePhoneNumber.mockRejectedValue(new Error('deletePhoneNumber failed: 404'))

      const store = useWhatsAppStore()

      await expect(store.removePhoneNumber(ORG_ID, 'phone-1')).rejects.toThrow(
        'deletePhoneNumber failed: 404',
      )
    })
  })

  // --- Calls ---

  describe('fetchCalls', () => {
    it('populates calls and total on success', async () => {
      const call = fakeCall()
      const response: WhatsAppCallListResponse = {
        calls: [call],
        total: 1,
        limit: 20,
        offset: 0,
      }
      mockedListCalls.mockResolvedValue(response)

      const store = useWhatsAppStore()
      await store.fetchCalls(ORG_ID)

      expect(mockedListCalls).toHaveBeenCalledWith(ORG_ID, 0, 20)
      expect(store.calls).toEqual([call])
      expect(store.callTotal).toBe(1)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('sets error on failure', async () => {
      mockedListCalls.mockRejectedValue(new Error('listCalls failed: 500'))

      const store = useWhatsAppStore()
      await store.fetchCalls(ORG_ID)

      expect(store.calls).toEqual([])
      expect(store.error).toBe('listCalls failed: 500')
      expect(store.loading).toBe(false)
    })
  })

  describe('fetchCall', () => {
    it('sets activeCall on success', async () => {
      const call = fakeCall()
      mockedGetCall.mockResolvedValue(call)

      const store = useWhatsAppStore()
      await store.fetchCall(ORG_ID, 'call-1')

      expect(mockedGetCall).toHaveBeenCalledWith(ORG_ID, 'call-1')
      expect(store.activeCall).toEqual(call)
      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      mockedGetCall.mockRejectedValue(new Error('getCall failed: 404'))

      const store = useWhatsAppStore()
      await store.fetchCall(ORG_ID, 'call-1')

      expect(store.activeCall).toBeNull()
      expect(store.error).toBe('getCall failed: 404')
    })
  })

  describe('startCall', () => {
    it('adds call to list, sets activeCall, and increments total', async () => {
      const call = fakeCall({ id: 'call-new' })
      mockedInitiateCall.mockResolvedValue(call)

      const store = useWhatsAppStore()
      store.callTotal = 5

      const result = await store.startCall(ORG_ID, {
        phone_number_id: 'phone-1',
        callee: '+0987654321',
      })

      expect(mockedInitiateCall).toHaveBeenCalledWith(ORG_ID, {
        phone_number_id: 'phone-1',
        callee: '+0987654321',
      })
      expect(result).toEqual(call)
      expect(store.calls[0]).toEqual(call)
      expect(store.callTotal).toBe(6)
      expect(store.activeCall).toEqual(call)
    })

    it('propagates API errors', async () => {
      mockedInitiateCall.mockRejectedValue(new Error('initiateCall failed: 400'))

      const store = useWhatsAppStore()

      await expect(
        store.startCall(ORG_ID, { phone_number_id: '', callee: '' }),
      ).rejects.toThrow('initiateCall failed: 400')
    })
  })

  describe('endCall', () => {
    it('updates call state to ended and updates list', async () => {
      const endedCall = fakeCall({ id: 'call-1', state: 'ended' })
      mockedUpdateCallState.mockResolvedValue(endedCall)

      const store = useWhatsAppStore()
      store.calls = [fakeCall()]
      store.activeCall = fakeCall()

      const result = await store.endCall(ORG_ID, 'call-1')

      expect(mockedUpdateCallState).toHaveBeenCalledWith(ORG_ID, 'call-1', 'ended')
      expect(result.state).toBe('ended')
      expect(store.activeCall?.state).toBe('ended')
      expect(store.calls[0].state).toBe('ended')
    })

    it('propagates API errors', async () => {
      mockedUpdateCallState.mockRejectedValue(new Error('updateCallState failed: 404'))

      const store = useWhatsAppStore()

      await expect(store.endCall(ORG_ID, 'call-1')).rejects.toThrow(
        'updateCallState failed: 404',
      )
    })
  })

  // --- Bridges ---

  describe('fetchBridgeStatus', () => {
    it('sets currentBridge on success', async () => {
      const bridge = fakeBridge()
      const response: BridgeStatusResponse = { bridge }
      mockedGetBridgeStatus.mockResolvedValue(response)

      const store = useWhatsAppStore()
      await store.fetchBridgeStatus(ORG_ID, 'call-1')

      expect(mockedGetBridgeStatus).toHaveBeenCalledWith(ORG_ID, 'call-1')
      expect(store.currentBridge).toEqual(bridge)
    })

    it('clears currentBridge on failure', async () => {
      mockedGetBridgeStatus.mockRejectedValue(new Error('not found'))

      const store = useWhatsAppStore()
      store.currentBridge = fakeBridge()
      await store.fetchBridgeStatus(ORG_ID, 'call-1')

      expect(store.currentBridge).toBeNull()
    })
  })

  describe('fetchActiveBridges', () => {
    it('populates bridges on success', async () => {
      const bridge = fakeBridge()
      mockedListActiveBridges.mockResolvedValue([bridge])

      const store = useWhatsAppStore()
      await store.fetchActiveBridges(ORG_ID)

      expect(mockedListActiveBridges).toHaveBeenCalledWith(ORG_ID)
      expect(store.bridges).toEqual([bridge])
    })

    it('sets error on failure', async () => {
      mockedListActiveBridges.mockRejectedValue(new Error('listActiveBridges failed: 500'))

      const store = useWhatsAppStore()
      await store.fetchActiveBridges(ORG_ID)

      expect(store.error).toBe('listActiveBridges failed: 500')
    })
  })

  describe('clearActiveCall', () => {
    it('clears both activeCall and currentBridge', () => {
      const store = useWhatsAppStore()
      store.activeCall = fakeCall()
      store.currentBridge = fakeBridge()

      store.clearActiveCall()

      expect(store.activeCall).toBeNull()
      expect(store.currentBridge).toBeNull()
    })
  })
})
