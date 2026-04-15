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

        <button
          class="w-full flex items-center justify-center gap-3 bg-white dark:bg-neutral-900 border border-neutral-300 dark:border-neutral-700 text-neutral-900 dark:text-white font-medium py-3.5 px-4 rounded-xl hover:bg-neutral-50 dark:hover:bg-neutral-800 transition-colors shadow-sm"
          :disabled="loading"
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

        <p v-if="error" class="text-red-500 text-sm mt-4 text-center">{{ error }}</p>

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
import { ref } from 'vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const loading = ref(false)
const error = ref('')

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
</script>
