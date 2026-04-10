<template>
  <Teleport to="body">
    <Transition name="backdrop">
      <div
        v-if="open"
        class="fixed inset-0 z-50 bg-black/50"
        @click="$emit('close')"
      />
    </Transition>

    <Transition name="sheet">
      <div
        v-if="open"
        class="fixed inset-x-0 bottom-0 z-50 rounded-t-2xl bg-slate-800"
        :style="{ paddingBottom: 'env(safe-area-inset-bottom)' }"
      >
        <!-- Drag handle -->
        <div class="flex justify-center pt-2 pb-1">
          <div class="h-1 w-9 rounded-full bg-slate-600" />
        </div>

        <div v-if="title" class="px-4 pb-3">
          <h3 class="text-center text-sm font-semibold text-slate-300">{{ title }}</h3>
        </div>

        <slot />
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
defineProps<{
  open: boolean
  title?: string
}>()

defineEmits<{
  close: []
}>()
</script>

<style scoped>
.backdrop-enter-active,
.backdrop-leave-active {
  transition: opacity 0.25s ease;
}
.backdrop-enter-from,
.backdrop-leave-to {
  opacity: 0;
}

.sheet-enter-active,
.sheet-leave-active {
  transition: transform 0.3s ease;
}
.sheet-enter-from,
.sheet-leave-to {
  transform: translateY(100%);
}
</style>
