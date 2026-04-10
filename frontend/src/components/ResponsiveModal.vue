<script setup lang="ts">
import { useMobile } from '../composables/useMediaQuery'

defineProps<{
  open: boolean
  title: string
}>()

defineEmits<{
  close: []
}>()

const { isMobile } = useMobile()
</script>

<template>
  <Teleport to="body">
    <!-- Mobile: full-screen page -->
    <div
      v-if="open && isMobile"
      role="dialog"
      aria-modal="true"
      :aria-label="title"
      class="fixed inset-0 z-50 flex flex-col bg-slate-900"
      @keydown.escape="$emit('close')"
    >
      <!-- Header -->
      <div class="flex h-14 items-center border-b border-slate-700 px-4">
        <button
          class="flex min-h-[44px] min-w-[44px] items-center justify-center text-slate-300 hover:text-white"
          :aria-label="`Close ${title}`"
          @click="$emit('close')"
        >
          <svg width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path d="M19 12H5M12 19l-7-7 7-7" />
          </svg>
        </button>
        <span class="flex-1 text-center text-[15px] font-semibold text-white">{{ title }}</span>
        <!-- Spacer to keep title centered -->
        <div class="w-11" />
      </div>

      <!-- Body -->
      <div class="flex-1 overflow-y-auto p-4">
        <slot />
      </div>

      <!-- Footer / Actions -->
      <div
        class="border-t border-slate-700 p-4"
        :style="{ paddingBottom: 'max(env(safe-area-inset-bottom), 1rem)' }"
      >
        <slot name="actions" />
      </div>
    </div>

    <!-- Desktop: centered overlay -->
    <div
      v-else-if="open && !isMobile"
      role="dialog"
      aria-modal="true"
      :aria-label="title"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
      @click.self="$emit('close')"
      @keydown.escape="$emit('close')"
    >
      <div class="w-full max-w-md rounded-xl bg-slate-800 mx-4">
        <slot />
      </div>
    </div>
  </Teleport>
</template>
