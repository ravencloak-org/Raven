<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

// Storage key shared with the SuperTokens preAPIHook that reads the
// token back when /auth/signinup is called after the Google OAuth
// round-trip. sessionStorage survives same-tab redirects but is
// scoped per origin and cleared on tab close — exactly the lifetime
// we want for a signup-flow Turnstile token.
const STORAGE_KEY = 'raven.turnstile.token.v1'

const props = defineProps<{
  siteKey: string
}>()

const emit = defineEmits<{
  (e: 'token', token: string): void
  (e: 'expired'): void
}>()

const container = ref<HTMLDivElement | null>(null)
let widgetId: string | null = null

function loadScript(): Promise<void> {
  return new Promise((resolve, reject) => {
    if (document.querySelector('script[data-raven-turnstile]')) {
      resolve()
      return
    }
    const s = document.createElement('script')
    s.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js'
    s.async = true
    s.defer = true
    s.dataset.ravenTurnstile = 'true'
    s.onload = () => resolve()
    s.onerror = () => reject(new Error('Failed to load Turnstile'))
    document.head.appendChild(s)
  })
}

onMounted(async () => {
  if (!props.siteKey) return // dev / non-demo: render nothing
  try {
    await loadScript()
  } catch {
    return
  }
  // Wait one tick for window.turnstile to register itself.
  await new Promise((r) => setTimeout(r, 0))
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const turnstile = (window as any).turnstile
  if (!turnstile || !container.value) return

  widgetId = turnstile.render(container.value, {
    sitekey: props.siteKey,
    callback: (token: string) => {
      try {
        sessionStorage.setItem(STORAGE_KEY, token)
      } catch {
        // ignore — sessionStorage may be unavailable in private mode
      }
      emit('token', token)
    },
    'expired-callback': () => {
      try {
        sessionStorage.removeItem(STORAGE_KEY)
      } catch {
        // ignore
      }
      emit('expired')
    },
  })
})

onBeforeUnmount(() => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const turnstile = (window as any).turnstile
  if (widgetId !== null && turnstile) {
    try {
      turnstile.remove(widgetId)
    } catch {
      // ignore
    }
  }
})
</script>

<template>
  <div v-if="siteKey" ref="container" class="raven-turnstile" />
</template>

<style scoped>
.raven-turnstile {
  display: flex;
  justify-content: center;
}
</style>
