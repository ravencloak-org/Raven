<template>
  <aside class="flex w-16 flex-col items-center bg-slate-900 py-4">
    <div class="mb-8 flex h-10 w-10 items-center justify-center rounded-lg bg-indigo-600">
      <span class="text-lg font-bold text-white">R</span>
    </div>

    <nav class="flex flex-1 flex-col items-center gap-4">
      <RouterLink
        to="/dashboard"
        class="flex h-10 w-10 items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-slate-800 hover:text-white"
        active-class="bg-slate-800 text-white"
        title="Dashboard"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5"
          viewBox="0 0 20 20"
          fill="currentColor"
        >
          <path
            d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 001.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z"
          />
        </svg>
      </RouterLink>
    </nav>
  </aside>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>
