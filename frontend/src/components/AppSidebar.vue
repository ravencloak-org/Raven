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

        <!-- Voice Sessions -->
        <RouterLink
          v-if="orgPrefix"
          :to="`${orgPrefix}/voice`"
          :class="[
            'flex items-center rounded-lg text-slate-400 transition-colors hover:bg-slate-800 hover:text-white',
            mobile
              ? 'h-10 w-full gap-3 px-3'
              : 'h-10 w-10 justify-center',
          ]"
          active-class="bg-slate-800 text-white"
          title="Voice Sessions"
          @click="mobile && $emit('close')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 shrink-0"
            viewBox="0 0 20 20"
            fill="currentColor"
          >
            <path
              fill-rule="evenodd"
              d="M7 4a3 3 0 016 0v4a3 3 0 11-6 0V4zm4 10.93A7.001 7.001 0 0017 8a1 1 0 10-2 0A5 5 0 015 8a1 1 0 00-2 0 7.001 7.001 0 006 6.93V17H6a1 1 0 100 2h8a1 1 0 100-2h-3v-2.07z"
              clip-rule="evenodd"
            />
          </svg>
          <span v-if="mobile" class="text-sm font-medium">Voice Sessions</span>
        </RouterLink>

        <!-- WhatsApp section label (mobile only) -->
        <span v-if="mobile && orgPrefix" class="mt-4 px-1 text-xs font-semibold uppercase tracking-wider text-slate-500">
          WhatsApp
        </span>

        <!-- Phone Numbers -->
        <RouterLink
          v-if="orgPrefix"
          :to="`${orgPrefix}/whatsapp/phone-numbers`"
          :class="[
            'flex items-center rounded-lg text-slate-400 transition-colors hover:bg-slate-800 hover:text-white',
            mobile
              ? 'h-10 w-full gap-3 px-3'
              : 'h-10 w-10 justify-center',
          ]"
          active-class="bg-slate-800 text-white"
          title="Phone Numbers"
          @click="mobile && $emit('close')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 shrink-0"
            viewBox="0 0 20 20"
            fill="currentColor"
          >
            <path
              d="M2 3a1 1 0 011-1h2.153a1 1 0 01.986.836l.74 4.435a1 1 0 01-.54 1.06l-1.548.773a11.037 11.037 0 006.105 6.105l.774-1.548a1 1 0 011.059-.54l4.435.74a1 1 0 01.836.986V17a1 1 0 01-1 1h-2C7.82 18 2 12.18 2 5V3z"
            />
          </svg>
          <span v-if="mobile" class="text-sm font-medium">Phone Numbers</span>
        </RouterLink>

        <!-- Calls -->
        <RouterLink
          v-if="orgPrefix"
          :to="`${orgPrefix}/whatsapp/calls`"
          :class="[
            'flex items-center rounded-lg text-slate-400 transition-colors hover:bg-slate-800 hover:text-white',
            mobile
              ? 'h-10 w-full gap-3 px-3'
              : 'h-10 w-10 justify-center',
          ]"
          active-class="bg-slate-800 text-white"
          title="Calls"
          @click="mobile && $emit('close')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 shrink-0"
            viewBox="0 0 20 20"
            fill="currentColor"
          >
            <path
              d="M17.924 2.617a.997.997 0 00-.215-.322l-.004-.004A.997.997 0 0017 2h-4a1 1 0 100 2h1.586l-3.293 3.293a1 1 0 001.414 1.414L16 5.414V7a1 1 0 102 0V3a.997.997 0 00-.076-.383zM2 3a1 1 0 011-1h2.153a1 1 0 01.986.836l.74 4.435a1 1 0 01-.54 1.06l-1.548.773a11.037 11.037 0 006.105 6.105l.774-1.548a1 1 0 011.059-.54l4.435.74a1 1 0 01.836.986V17a1 1 0 01-1 1h-2C7.82 18 2 12.18 2 5V3z"
            />
          </svg>
          <span v-if="mobile" class="text-sm font-medium">Calls</span>
        </RouterLink>

        <!-- Billing section label (mobile only) -->
        <span v-if="mobile && orgPrefix" class="mt-4 px-1 text-xs font-semibold uppercase tracking-wider text-slate-500">
          Account
        </span>

        <!-- Billing -->
        <RouterLink
          v-if="orgPrefix"
          :to="`${orgPrefix}/billing`"
          :class="[
            'flex items-center rounded-lg text-slate-400 transition-colors hover:bg-slate-800 hover:text-white',
            mobile
              ? 'h-10 w-full gap-3 px-3'
              : 'h-10 w-10 justify-center',
          ]"
          active-class="bg-slate-800 text-white"
          title="Billing"
          @click="mobile && $emit('close')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 shrink-0"
            viewBox="0 0 20 20"
            fill="currentColor"
          >
            <path
              d="M4 4a2 2 0 00-2 2v4a2 2 0 002 2V6h10a2 2 0 00-2-2H4zm2 6a2 2 0 012-2h8a2 2 0 012 2v4a2 2 0 01-2 2H8a2 2 0 01-2-2v-4zm6 4a2 2 0 100-4 2 2 0 000 4z"
            />
          </svg>
          <span v-if="mobile" class="text-sm font-medium">Billing</span>
        </RouterLink>
      </nav>
    </aside>
  </Transition>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const orgPrefix = computed(() =>
  auth.orgId ? `/orgs/${auth.orgId}` : null,
)

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
