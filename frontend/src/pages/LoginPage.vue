<template>
  <div class="min-h-screen flex bg-white dark:bg-black">
    <!-- Left panel — branding -->
    <div class="hidden lg:flex lg:w-1/2 bg-neutral-950 items-center justify-center p-12">
      <div class="max-w-md">
        <h1 class="text-4xl font-bold text-white mb-4">Raven</h1>
        <p class="text-neutral-400 text-lg leading-relaxed">
          The AI brain for your entire team. Organize knowledge, build chatbots, and search smarter.
        </p>
      </div>
    </div>

    <!-- Right panel — login form -->
    <div class="flex-1 flex items-center justify-center p-8">
      <div class="w-full max-w-md">
        <!-- Mobile logo -->
        <h1 class="lg:hidden text-3xl font-bold text-neutral-900 dark:text-white mb-1 text-center">Raven</h1>
        <p class="lg:hidden text-neutral-500 text-sm mb-10 text-center">The AI brain for your entire team</p>

        <h2 class="text-2xl font-bold text-neutral-900 dark:text-white mb-2">Welcome back</h2>
        <p class="text-neutral-500 text-sm mb-8">Sign in to your account to continue</p>

        <TurnstileWidget
          v-if="turnstileSiteKey"
          :site-key="turnstileSiteKey"
          class="mb-4"
          @token="(t) => (turnstileToken = t)"
          @expired="turnstileToken = ''"
        />

        <button
          class="w-full flex items-center justify-center gap-3 bg-white dark:bg-neutral-900 border border-neutral-300 dark:border-neutral-700 text-neutral-900 dark:text-white font-medium py-3.5 px-4 rounded-xl hover:bg-neutral-50 dark:hover:bg-neutral-800 transition-colors shadow-sm disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-neutral-400 dark:focus:ring-neutral-600"
          :disabled="loading || (turnstileSiteKey !== '' && !turnstileToken)"
          @click="signInWithGoogle"
        >
          <svg class="w-5 h-5 shrink-0" viewBox="0 0 24 24">
            <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4" />
            <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853" />
            <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05" />
            <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335" />
          </svg>
          {{ loading ? 'Redirecting...' : 'Sign in with Google' }}
        </button>

        <!-- Divider between Google and Passkey buttons -->
        <div class="flex items-center my-4" role="separator" aria-label="or">
          <div class="flex-1 h-px bg-neutral-200 dark:bg-neutral-800" />
          <span class="px-3 text-xs uppercase tracking-wider text-neutral-400 dark:text-neutral-500">or</span>
          <div class="flex-1 h-px bg-neutral-200 dark:bg-neutral-800" />
        </div>

        <button
          data-testid="signin-passkey-btn"
          class="w-full flex items-center justify-center gap-3 bg-white dark:bg-neutral-900 border border-neutral-300 dark:border-neutral-700 text-neutral-900 dark:text-white font-medium py-3.5 px-4 rounded-xl hover:bg-neutral-50 dark:hover:bg-neutral-800 transition-colors shadow-sm disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-neutral-400 dark:focus:ring-neutral-600"
          :disabled="passkeyLoading || passkeyDisabled"
          :title="passkeyDisabled ? 'Your browser doesn\'t support passkeys' : ''"
          :aria-label="passkeyDisabled ? 'Sign in with Passkey (unsupported in this browser)' : 'Sign in with Passkey'"
          @click="signInWithPasskey"
        >
          <!-- Key/chip icon -->
          <svg
            class="w-5 h-5 shrink-0"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <circle cx="8" cy="12" r="4" />
            <path d="M12 12h10" />
            <path d="M18 12v3" />
            <path d="M22 12v2" />
          </svg>
          {{ passkeyLoading ? 'Waiting for passkey...' : 'Sign in with Passkey' }}
        </button>

        <p v-if="error" class="text-red-500 text-sm mt-4 text-center">{{ error }}</p>
        <p v-if="passkeyError" data-testid="signin-passkey-error" class="text-red-500 text-sm mt-4 text-center">
          {{ passkeyError }}
        </p>

        <p class="text-neutral-400 text-xs mt-10 text-center">
          By continuing, you agree to our
          <router-link to="/legal/terms" class="underline hover:text-neutral-600">Terms</router-link> &amp;
          <router-link to="/legal/privacy" class="underline hover:text-neutral-600">Privacy Policy</router-link>
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  authenticateCredentialWithSignIn,
  doesBrowserSupportWebAuthn,
} from 'supertokens-web-js/recipe/webauthn'
import TurnstileWidget from '../components/TurnstileWidget.vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const loading = ref(false)
const error = ref('')

const passkeyLoading = ref(false)
const passkeyDisabled = ref(false)
const passkeyError = ref('')

const turnstileSiteKey = import.meta.env.VITE_TURNSTILE_SITE_KEY ?? ''
const turnstileToken = ref('')

const NOT_SUPPORTED_MSG =
  "This browser doesn't support passkeys. Use Google sign-in instead."
const INVALID_CREDENTIALS_MSG =
  'Passkey not recognised. Try Google sign-in or set up a passkey from Settings → Authentication.'
const GENERIC_ERROR_MSG = 'Unable to sign in with passkey. Please try again.'

async function signInWithGoogle() {
  loading.value = true
  error.value = ''
  try {
    await auth.loginWithGoogle()
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : 'Unable to start sign-in'
  } finally {
    loading.value = false
  }
}

async function signInWithPasskey() {
  passkeyError.value = ''
  if (passkeyDisabled.value) {
    passkeyError.value = NOT_SUPPORTED_MSG
    return
  }
  passkeyLoading.value = true
  try {
    const support = await doesBrowserSupportWebAuthn({ userContext: {} })
    if (support.status !== 'OK' || !support.browserSupportsWebauthn) {
      passkeyDisabled.value = true
      passkeyError.value = NOT_SUPPORTED_MSG
      return
    }

    const response = await authenticateCredentialWithSignIn({ userContext: {} })
    if (response.status === 'OK') {
      // Mirror the Google success handler: SuperTokens has set the session
      // tokens; refresh the auth store so the router guard sees the new
      // session, then send the user to the dashboard.
      await auth.init()
      await router.push('/')
      return
    }
    if (response.status === 'INVALID_CREDENTIALS_ERROR') {
      passkeyError.value = INVALID_CREDENTIALS_MSG
      return
    }
    if (response.status === 'WEBAUTHN_NOT_SUPPORTED') {
      passkeyDisabled.value = true
      passkeyError.value = NOT_SUPPORTED_MSG
      return
    }
    passkeyError.value = GENERIC_ERROR_MSG
  } catch (e: unknown) {
    passkeyError.value = e instanceof Error ? e.message : GENERIC_ERROR_MSG
  } finally {
    passkeyLoading.value = false
  }
}

onMounted(async () => {
  try {
    const support = await doesBrowserSupportWebAuthn({ userContext: {} })
    if (support.status !== 'OK' || !support.browserSupportsWebauthn) {
      passkeyDisabled.value = true
    }
  } catch {
    // SDK probe failed (network or environment quirk) — be conservative:
    // disable the button rather than letting the user click into a broken
    // flow. The Google button is still available as fallback.
    passkeyDisabled.value = true
  }
})
</script>
