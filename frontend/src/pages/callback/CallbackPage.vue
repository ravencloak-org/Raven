<template>
  <div class="min-h-screen flex items-center justify-center bg-white dark:bg-black">
    <div class="text-center">
      <p class="text-neutral-500" role="status" aria-live="polite">Completing sign in...</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'

const router = useRouter()
const auth = useAuthStore()
const apiUrl = import.meta.env.VITE_API_BASE_URL

onMounted(async () => {
  try {
    const user = await auth.handleCallback()

    // Call backend auth callback
    const res = await fetch(`${apiUrl}/auth/callback`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${user.access_token}`,
        'Content-Type': 'application/json',
      },
    })
    if (!res.ok) {
      throw new Error(`Auth callback failed (${res.status})`)
    }
    const data = await res.json()

    if (data.isNewUser) {
      router.push('/onboarding')
    } else {
      auth.setOrgId(data.orgId)
      router.push('/dashboard')
    }
  } catch (err) {
    console.error('Callback error:', err)
    router.push('/login')
  }
})
</script>
