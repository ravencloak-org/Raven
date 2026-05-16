/**
 * Feature flags read at runtime from window.__RAVEN_CONFIG__ (set by the
 * container entrypoint), falling back to build-time VITE_* env vars for
 * dev. Keeping the surface intentionally tiny — flags here should be
 * about WHICH FEATURES SHOW, not deep configuration.
 */

interface RavenRuntimeConfig {
  voiceEnabled?: boolean
  turnstileSiteKey?: string
}

function runtimeConfig(): RavenRuntimeConfig | undefined {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  return (window as unknown as { __RAVEN_CONFIG__?: RavenRuntimeConfig })
    .__RAVEN_CONFIG__
}

/**
 * Whether voice/LiveKit features are exposed to the user.
 *
 * The public demo at demo.raven.ravencloak.org disables this (text-only
 * mode) because Cloudflare Tunnel can't carry WebRTC's UDP media.
 * Self-hosted and dev builds default to enabled.
 */
export function isVoiceEnabled(): boolean {
  const r = runtimeConfig()
  if (typeof r?.voiceEnabled === 'boolean') return r.voiceEnabled
  // Build-time fallback for dev. Treat the explicit string 'false' as
  // disabled; anything else (including unset) as enabled.
  return import.meta.env.VITE_VOICE_ENABLED !== 'false'
}
