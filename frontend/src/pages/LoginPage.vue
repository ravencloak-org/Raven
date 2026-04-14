<template>
  <div class="min-h-screen flex items-center justify-center bg-white dark:bg-black p-4">
    <div class="w-full max-w-sm text-center">
      <h1 class="text-3xl font-bold text-neutral-900 dark:text-white mb-2">Raven</h1>
      <p class="text-neutral-500 text-sm mb-8">Sign in to continue</p>

      <button
        class="w-full flex items-center justify-center gap-3 bg-white dark:bg-neutral-900 border border-neutral-300 dark:border-neutral-700 text-neutral-900 dark:text-white font-medium py-3 px-4 rounded-lg hover:bg-neutral-50 dark:hover:bg-neutral-800 transition-colors mb-3"
        :disabled="loading"
        @click="signInWithGoogle"
      >
        <svg class="w-5 h-5" viewBox="0 0 24 24"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/></svg>
        Sign in with Google
      </button>

      <p v-if="error" class="text-red-500 text-sm mt-4">{{ error }}</p>

      <p class="text-neutral-400 text-xs mt-8">
        By continuing, you agree to our
        <router-link to="/legal/terms" class="underline">Terms</router-link> &amp;
        <router-link to="/legal/privacy" class="underline">Privacy Policy</router-link>
      </p>
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
