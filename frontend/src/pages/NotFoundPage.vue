<template>
  <div class="flex min-h-screen flex-col items-center justify-center bg-gray-50">
    <h1 class="mb-2 text-6xl font-bold text-gray-300">404</h1>
    <p class="mb-6 text-lg text-gray-600">Page not found</p>
    <RouterLink
      to="/dashboard"
      class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700"
    >
      Back to Dashboard
    </RouterLink>
  </div>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>
