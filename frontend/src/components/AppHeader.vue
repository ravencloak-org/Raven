<template>
  <header class="flex h-14 items-center justify-between border-b border-gray-200 bg-white px-4 md:px-6">
    <div class="flex items-center gap-3">
      <button
        class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-600 transition-colors hover:bg-gray-100 md:hidden"
        title="Toggle menu"
        @click="$emit('toggle-sidebar')"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 12h16M4 18h16" />
        </svg>
      </button>
      <h2 class="text-sm font-semibold text-gray-700">Raven</h2>
    </div>

    <div class="flex items-center gap-3">
      <button
        class="flex h-8 w-8 items-center justify-center rounded-full bg-indigo-100 text-sm font-medium text-indigo-700 transition-colors hover:bg-indigo-200"
        title="User menu"
        @click="handleUserMenu"
      >
        {{ userInitial }}
      </button>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useAuth } from '../composables/useAuth'

defineEmits<{
  'toggle-sidebar': []
}>()

const { user } = useAuth()

const userInitial = computed(() => {
  if (user.value?.username) {
    return user.value.username.charAt(0).toUpperCase()
  }
  return 'U'
})

function handleUserMenu() {
  // Placeholder: will open a dropdown with profile/settings/logout
}
</script>
