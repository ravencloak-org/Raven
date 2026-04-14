<template>
  <header class="flex h-14 items-center justify-between border-b border-gray-200 bg-white px-4 md:px-6">
    <div class="flex items-center gap-3">
      <h2 class="text-sm font-semibold text-gray-700">Raven</h2>
    </div>

    <div class="relative flex items-center gap-3">
      <button
        class="flex h-8 w-8 items-center justify-center rounded-full bg-indigo-100 text-indigo-700 transition-colors hover:bg-indigo-200"
        title="User menu"
        aria-haspopup="true"
        :aria-expanded="menuOpen"
        aria-controls="user-menu"
        @click="menuOpen = !menuOpen"
      >
        <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24" aria-hidden="true">
          <path stroke-linecap="round" stroke-linejoin="round" d="M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0zM12 14a7 7 0 0 0-7 7h14a7 7 0 0 0-7-7z" />
        </svg>
      </button>

      <div
        v-if="menuOpen"
        id="user-menu"
        role="menu"
        class="absolute right-0 top-10 z-50 w-48 rounded-md border border-gray-200 bg-white py-1 shadow-lg"
      >
        <button
          class="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-50"
          @click="handleLogout"
        >
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
          </svg>
          Sign out
        </button>
      </div>

      <div v-if="menuOpen" class="fixed inset-0 z-40" @click="menuOpen = false" />
    </div>
  </header>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from '../stores/auth'

const authStore = useAuthStore()
const menuOpen = ref(false)

function onEscape(e: KeyboardEvent) {
  if (e.key === 'Escape' && menuOpen.value) menuOpen.value = false
}
onMounted(() => window.addEventListener('keydown', onEscape))
onUnmounted(() => window.removeEventListener('keydown', onEscape))

function handleLogout() {
  menuOpen.value = false
  authStore.logout()
}
</script>
