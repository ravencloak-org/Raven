// useHyperswitch — small helper around the Hyperswitch HyperLoader SDK.
//
// CheckoutPage.vue defines an equivalent inline loader; this composable
// extracts the CDN bootstrap so that other surfaces (e.g. the in-cycle
// seat upgrade flow on BillingPage) can reuse it without duplicating the
// script-tag dance.

const HYPERSWITCH_SDK_URL = 'https://beta.hyperswitch.io/v1/HyperLoader.js'
const HYPERSWITCH_SDK_ID = 'hyperswitch-sdk'

export interface HyperPaymentElement {
  mount: (selector: string) => void
  unmount: () => void
}

export interface HyperElements {
  create: (type: string) => HyperPaymentElement
  submit: () => Promise<{ error?: { message: string } }>
}

export interface HyperInstance {
  elements: (opts: { clientSecret: string }) => HyperElements
}

declare global {
  interface Window {
    Hyper?: (key: string) => HyperInstance
  }
}

/**
 * Dynamically inject the Hyperswitch HyperLoader script tag (once globally)
 * and resolve when `window.Hyper` is available.
 */
export function loadHyperswitchSDK(): Promise<void> {
  return new Promise((resolve, reject) => {
    if (typeof window === 'undefined') {
      reject(new Error('Hyperswitch SDK requires a browser environment'))
      return
    }

    if (window.Hyper) {
      resolve()
      return
    }

    const existing = document.getElementById(HYPERSWITCH_SDK_ID)
    if (existing) {
      existing.addEventListener('load', () => resolve())
      existing.addEventListener('error', () =>
        reject(new Error('Hyperswitch SDK failed to load')),
      )
      return
    }

    const script = document.createElement('script')
    script.id = HYPERSWITCH_SDK_ID
    script.src = HYPERSWITCH_SDK_URL
    script.async = true
    script.onload = () => resolve()
    script.onerror = () =>
      reject(new Error('Hyperswitch SDK failed to load from CDN'))
    document.head.appendChild(script)
  })
}

/**
 * Resolve the publishable key from Vite env at call time so consumers can
 * surface a sensible error if the deployment is misconfigured.
 */
export function getHyperswitchPublishableKey(): string {
  return import.meta.env.VITE_HYPERSWITCH_PUBLISHABLE_KEY ?? ''
}
