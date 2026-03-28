import { ref, readonly } from 'vue'

export interface CookieConsent {
  essential: boolean
  analytics: boolean
  marketing: boolean
  timestamp: string
}

const STORAGE_KEY = 'raven_cookie_consent'

const hasConsented = ref(false)
const consentGiven = ref<CookieConsent | null>(null)

function loadConsent(): CookieConsent | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    return JSON.parse(raw) as CookieConsent
  } catch {
    return null
  }
}

function saveConsent(consent: CookieConsent): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(consent))
  consentGiven.value = consent
  hasConsented.value = true
  window.dispatchEvent(
    new CustomEvent('raven:cookie-consent', { detail: consent }),
  )
}

function init(): void {
  const stored = loadConsent()
  if (stored) {
    hasConsented.value = true
    consentGiven.value = stored
  }
}

init()

export function useCookieConsent() {
  function acceptAll(): void {
    saveConsent({
      essential: true,
      analytics: true,
      marketing: true,
      timestamp: new Date().toISOString(),
    })
  }

  function rejectNonEssential(): void {
    saveConsent({
      essential: true,
      analytics: false,
      marketing: false,
      timestamp: new Date().toISOString(),
    })
  }

  function customise(options: { analytics: boolean; marketing: boolean }): void {
    saveConsent({
      essential: true,
      analytics: options.analytics,
      marketing: options.marketing,
      timestamp: new Date().toISOString(),
    })
  }

  function getConsent(): CookieConsent | null {
    return consentGiven.value
  }

  return {
    hasConsented: readonly(hasConsented),
    consentGiven: readonly(consentGiven),
    acceptAll,
    rejectNonEssential,
    customise,
    getConsent,
  }
}
