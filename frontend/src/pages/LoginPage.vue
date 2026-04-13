<template>
  <div class="min-h-screen flex items-center justify-center bg-white dark:bg-black">
    <div class="text-center">
      <h1 class="text-2xl font-bold text-neutral-900 dark:text-white mb-4">Redirecting to Google...</h1>
      <p class="text-neutral-500">If you are not redirected, <button @click="login" class="text-amber-500 underline">click here</button>.</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const googleIdpId = import.meta.env.VITE_GOOGLE_IDP_ID

function login() {
  auth.login(googleIdpId)
}

onMounted(() => {
  login()
})
</script>
