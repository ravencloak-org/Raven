// PostHog analytics plugin for Vue.js
//
// Initialises the PostHog JS SDK when a valid API key is provided via the
// VITE_POSTHOG_API_KEY environment variable.  Tracking is entirely opt-in:
//
//  1. No API key configured  -> plugin is a no-op.
//  2. User has not given cookie consent (or declined analytics) -> events are
//     not sent until consent is granted.
//
// Usage:
//   import { posthogPlugin } from './plugins/posthog'
//   app.use(posthogPlugin, { router })
//
// TODO: run `npm install posthog-js` before using this module.

import type { App } from 'vue'
import type { Router } from 'vue-router'
// TODO: uncomment the import below after installing posthog-js
// import posthog from 'posthog-js'
import { ref, type Ref } from 'vue'
import type { CookieConsent } from '../composables/useCookieConsent'

// ---------------------------------------------------------------------------
// Inline type stubs so the file compiles without the posthog-js dependency.
// Remove this block once `posthog-js` is installed and the import above is
// uncommented.
// ---------------------------------------------------------------------------
interface PostHogStub {
  init(apiKey: string, options: Record<string, unknown>): void
  identify(distinctId: string, properties?: Record<string, unknown>): void
  capture(event: string, properties?: Record<string, unknown>): void
  opt_in_capturing(): void
  opt_out_capturing(): void
  has_opted_in_capturing(): boolean
  reset(): void
  isFeatureEnabled(flag: string): boolean | undefined
  onFeatureFlags(callback: (flags: string[]) => void): void
  getFeatureFlag(flag: string): string | boolean | undefined
  reloadFeatureFlags(): void
}
declare const posthog: PostHogStub
// ---------------------------------------------------------------------------

const POSTHOG_API_KEY = import.meta.env.VITE_POSTHOG_API_KEY ?? ''
const POSTHOG_HOST = import.meta.env.VITE_POSTHOG_HOST ?? 'https://us.i.posthog.com'
const COOKIE_CONSENT_KEY = 'raven_cookie_consent'

/** Whether PostHog has been successfully initialised. */
const isInitialised = ref(false)

/** Reactive reference to the underlying posthog instance (or null). */
const posthogInstance: Ref<PostHogStub | null> = ref(null)

// ---------------------------------------------------------------------------
// Cookie consent helpers
// ---------------------------------------------------------------------------

function hasAnalyticsConsent(): boolean {
  try {
    const raw = localStorage.getItem(COOKIE_CONSENT_KEY)
    if (!raw) return false
    const consent: CookieConsent = JSON.parse(raw)
    return consent.analytics === true
  } catch {
    return false
  }
}

function syncCaptureConsent(): void {
  if (!posthogInstance.value) return
  if (hasAnalyticsConsent()) {
    posthogInstance.value.opt_in_capturing()
  } else {
    posthogInstance.value.opt_out_capturing()
  }
}

// ---------------------------------------------------------------------------
// Initialisation
// ---------------------------------------------------------------------------

function initPostHog(): void {
  if (!POSTHOG_API_KEY) return

  posthog.init(POSTHOG_API_KEY, {
    api_host: POSTHOG_HOST,
    // Respect cookie consent: start opted-out; explicit opt_in later.
    opt_out_capturing_by_default: true,
    capture_pageview: false, // We handle page views via the router hook.
    capture_pageleave: true,
    persistence: 'localStorage+cookie',
    loaded: (ph: PostHogStub) => {
      posthogInstance.value = ph
      isInitialised.value = true
      syncCaptureConsent()
    },
  } as Record<string, unknown>)
}

// ---------------------------------------------------------------------------
// Public composable
// ---------------------------------------------------------------------------

/**
 * Returns the PostHog instance and helpers.  Safe to call even when PostHog
 * is not configured -- every method degrades to a no-op.
 */
export function usePostHog() {
  /** Identify the current user after authentication. */
  function identify(userId: string, properties?: Record<string, unknown>): void {
    posthogInstance.value?.identify(userId, properties)
  }

  /** Track a custom event. */
  function capture(event: string, properties?: Record<string, unknown>): void {
    posthogInstance.value?.capture(event, properties)
  }

  /** Reset identity (call on logout). */
  function reset(): void {
    posthogInstance.value?.reset()
  }

  /** Check a feature flag value. */
  function isFeatureEnabled(flag: string): boolean | undefined {
    return posthogInstance.value?.isFeatureEnabled(flag)
  }

  /** Get a feature flag value (supports multivariate flags). */
  function getFeatureFlag(flag: string): string | boolean | undefined {
    return posthogInstance.value?.getFeatureFlag(flag)
  }

  /** Register a callback to be invoked when feature flags are loaded. */
  function onFeatureFlags(callback: (flags: string[]) => void): void {
    posthogInstance.value?.onFeatureFlags(callback)
  }

  /** Force a reload of feature flags from PostHog. */
  function reloadFeatureFlags(): void {
    posthogInstance.value?.reloadFeatureFlags()
  }

  return {
    isInitialised,
    posthog: posthogInstance,
    identify,
    capture,
    reset,
    isFeatureEnabled,
    getFeatureFlag,
    onFeatureFlags,
    reloadFeatureFlags,
  }
}

// ---------------------------------------------------------------------------
// Vue plugin
// ---------------------------------------------------------------------------

export interface PostHogPluginOptions {
  router?: Router
}

export const posthogPlugin = {
  install(_app: App, options?: PostHogPluginOptions): void {
    // Bail out if no API key is configured -- entirely opt-in.
    if (!POSTHOG_API_KEY) return

    initPostHog()

    // Listen for consent changes at runtime so that granting/revoking consent
    // takes effect immediately without a page reload.
    window.addEventListener('raven:cookie-consent', ((event: CustomEvent<CookieConsent>) => {
      if (event.detail) {
        syncCaptureConsent()
      }
    }) as EventListener)

    // Automatic page-view tracking via the Vue Router afterEach hook.
    if (options?.router) {
      options.router.afterEach((to) => {
        if (!posthogInstance.value) return
        if (!hasAnalyticsConsent()) return

        posthogInstance.value.capture('$pageview', {
          $current_url: window.location.href,
          path: to.fullPath,
          route_name: (to.name as string) ?? undefined,
        })
      })
    }
  },
}
