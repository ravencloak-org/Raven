<script setup lang="ts">
import { onMounted, ref } from 'vue'

// Essential-only cookie consent. The demo sets no analytics or
// advertising cookies — only SuperTokens session + CSRF cookies, both
// strictly necessary under GDPR Art. 4(11) / 6(1)(b) / 6(1)(f).
// The banner is informational; clicking "Got it" stops it appearing
// again on this device. Consent is persisted in localStorage only —
// no cookie is set to record consent (would be paradoxical).

const STORAGE_KEY = 'raven.cookie.consent.v1'

const visible = ref(false)

onMounted(() => {
  try {
    if (!localStorage.getItem(STORAGE_KEY)) {
      visible.value = true
    }
  } catch {
    // localStorage may be unavailable (private mode, embedded webview).
    // In that case, suppress the banner — there's nowhere to record
    // dismissal, and re-showing on every page load would be obnoxious.
  }
})

function accept(): void {
  try {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ essential: true, accepted_at: new Date().toISOString() }),
    )
  } catch {
    // swallow — see onMounted
  }
  visible.value = false
}
</script>

<template>
  <Transition name="cookie-fade">
    <div v-if="visible" class="cookie-banner" role="dialog" aria-live="polite" aria-label="Cookie notice">
      <p class="cookie-banner__copy">
        Raven uses essential cookies for login and security only. No tracking,
        no analytics.
        <a href="/legal/privacy" class="cookie-banner__link">Learn more</a>.
      </p>
      <button
        type="button"
        class="cookie-banner__button"
        aria-label="Dismiss cookie notice"
        @click="accept"
      >
        Got it
      </button>
    </div>
  </Transition>
</template>

<style scoped>
.cookie-banner {
  position: fixed;
  inset-inline: 0;
  bottom: 0;
  z-index: 1000;
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  align-items: center;
  justify-content: space-between;
  padding: 0.875rem 1.25rem;
  background: var(--surface-1, #111);
  color: var(--text-1, #fff);
  border-top: 1px solid var(--surface-3, #333);
  font-size: 0.875rem;
  line-height: 1.4;
  box-shadow: 0 -4px 16px rgba(0, 0, 0, 0.2);
}

.cookie-banner__copy {
  margin: 0;
  flex: 1 1 320px;
}

.cookie-banner__link {
  color: var(--accent, #5b8dee);
  text-decoration: underline;
}

.cookie-banner__button {
  padding: 0.5rem 1rem;
  background: var(--accent, #5b8dee);
  color: #fff;
  border: 0;
  border-radius: 0.375rem;
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}

.cookie-banner__button:hover {
  filter: brightness(1.1);
}

.cookie-fade-enter-active,
.cookie-fade-leave-active {
  transition: opacity 200ms ease, transform 200ms ease;
}

.cookie-fade-enter-from,
.cookie-fade-leave-to {
  opacity: 0;
  transform: translateY(10px);
}
</style>
