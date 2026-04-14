<template>
  <div class="min-h-screen flex items-center justify-center bg-white dark:bg-black">
    <div class="text-center">
      <p class="text-neutral-500" role="status" aria-live="polite">Completing sign in...</p>
      <p v-if="error" class="text-red-500 text-sm mt-4">{{ error }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'

const router = useRouter()
const auth = useAuthStore()
const error = ref('')

onMounted(async () => {
  try {
    const result = await auth.handleCallback()

    if (result.isNewUser || !result.orgId) {
      router.push('/onboarding')
    } else {
      auth.setOrgId(result.orgId)
      router.push('/dashboard')
    }
  } catch (err) {
    console.error('Callback error:', err)
    error.value = 'Sign-in failed. Please try again.'
    setTimeout(() => router.push('/login'), 3000)
  }
})
</script>
