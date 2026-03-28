<template>
  <!-- Mobile backdrop -->
  <Transition name="backdrop">
    <div
      v-if="mobile && open"
      class="fixed inset-0 z-40 bg-black/50"
      @click="$emit('close')"
    />
  </Transition>

  <!-- Sidebar -->
  <Transition name="sidebar">
    <aside
      v-if="!mobile || open"
      :class="[
        'flex flex-col items-center bg-slate-900 py-4',
        mobile
          ? 'fixed inset-y-0 left-0 z-50 w-64'
          : 'w-16',
      ]"
    >
      <div class="flex w-full items-center px-4" :class="mobile ? 'mb-6 justify-between' : 'mb-8 justify-center'">
        <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-indigo-600">
          <span class="text-lg font-bold text-white">R</span>
        </div>
        <button
          v-if="mobile"
          class="flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-slate-800 hover:text-white"
          title="Close menu"
          @click="$emit('close')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="2"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <nav class="flex flex-1 flex-col items-center gap-4" :class="mobile ? 'w-full px-3' : ''">
        <RouterLink
          to="/dashboard"
          :class="[
            'flex items-center rounded-lg text-slate-400 transition-colors hover:bg-slate-800 hover:text-white',
            mobile
              ? 'h-10 w-full gap-3 px-3'
              : 'h-10 w-10 justify-center',
          ]"
          active-class="bg-slate-800 text-white"
          title="Dashboard"
          @click="mobile && $emit('close')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 shrink-0"
            viewBox="0 0 20 20"
            fill="currentColor"
          >
            <path
              d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 001.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z"
            />
          </svg>
          <span v-if="mobile" class="text-sm font-medium">Dashboard</span>
        </RouterLink>
      </nav>
    </aside>
  </Transition>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'

defineProps<{
  mobile?: boolean
  open?: boolean
}>()

defineEmits<{
  close: []
}>()
</script>

<style scoped>
/* Sidebar slide transition */
.sidebar-enter-active,
.sidebar-leave-active {
  transition: transform 0.25s ease;
}

.sidebar-enter-from,
.sidebar-leave-to {
  transform: translateX(-100%);
}

/* Backdrop fade transition */
.backdrop-enter-active,
.backdrop-leave-active {
  transition: opacity 0.25s ease;
}

.backdrop-enter-from,
.backdrop-leave-to {
  opacity: 0;
}
</style>
