<template>
  <div class="min-h-screen flex items-center justify-center bg-white dark:bg-black">
    <div class="text-center">
      <h1 class="text-2xl font-bold text-neutral-900 dark:text-white mb-4">Redirecting to Google...</h1>
      <p class="text-neutral-500">If you are not redirected, <button class="text-amber-500 underline min-h-[44px] min-w-[44px] px-1" @click="login">click here</button>.</p>
      <p v-if="error" class="text-red-500 text-sm mt-4">{{ error }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const googleIdpId = import.meta.env.VITE_GOOGLE_IDP_ID
const error = ref('')

async function login() {
  try {
    error.value = ''
    await auth.login(googleIdpId)
  } catch (e: unknown) {
    console.error('Login redirect failed:', e)
    error.value = 'Unable to start sign-in. Please try again.'
  }
}

onMounted(() => {
  login()
})
</script>
