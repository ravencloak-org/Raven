<template>
  <div>
    <h1 class="mb-1 text-center text-2xl font-bold text-gray-900">Sign in to Raven</h1>
    <p class="mb-6 text-center text-sm text-gray-500">
      Click below to sign in via Keycloak
    </p>

    <button
      type="button"
      class="w-full rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700 focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:outline-none"
      @click="handleLogin"
    >
      Login
    </button>

    <p class="mt-4 text-center text-xs text-gray-400">
      Authentication uses Keycloak OIDC with PKCE
    </p>
  </div>
</template>

<script setup lang="ts">
import { useAuth } from '../composables/useAuth'

const { login } = useAuth()

function handleLogin() {
  login()
}
</script>
