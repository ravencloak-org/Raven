<template>
  <div>
    <h1 class="mb-1 text-center text-2xl font-bold text-gray-900">Sign in to Raven</h1>
    <p class="mb-6 text-center text-sm text-gray-500">
      Enter your credentials to access your workspace
    </p>

    <form @submit.prevent="handleLogin">
      <div class="mb-4">
        <label for="email" class="mb-1 block text-sm font-medium text-gray-700">Email</label>
        <input
          id="email"
          v-model="email"
          type="email"
          required
          class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 focus:outline-none"
          placeholder="you@example.com"
        />
      </div>

      <div class="mb-6">
        <label for="password" class="mb-1 block text-sm font-medium text-gray-700">Password</label>
        <input
          id="password"
          v-model="password"
          type="password"
          required
          class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 focus:outline-none"
          placeholder="Enter your password"
        />
      </div>

      <button
        type="submit"
        class="w-full rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700 focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:outline-none"
      >
        Sign in
      </button>
    </form>

    <p class="mt-4 text-center text-xs text-gray-400">
      Authentication will use Keycloak OIDC in production
    </p>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'

const router = useRouter()
const { login } = useAuth()

const email = ref('')
const password = ref('')

async function handleLogin() {
  await login(email.value, password.value)
  const redirect = (router.currentRoute.value.query.redirect as string) || '/dashboard'
  router.push(redirect)
}
</script>
