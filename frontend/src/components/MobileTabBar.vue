<template>
  <nav
    aria-label="Primary navigation"
    class="fixed bottom-0 inset-x-0 z-40 flex justify-around items-center bg-slate-800 border-t border-slate-700"
    style="min-height: 56px; padding-bottom: env(safe-area-inset-bottom);"
  >
    <!-- Home -->
    <RouterLink
      to="/dashboard"
      class="flex flex-col items-center justify-center gap-0.5 min-w-[56px] min-h-[44px] px-2 transition-colors"
      :class="route.name === 'dashboard' ? 'text-indigo-400' : 'text-slate-400'"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 shrink-0" viewBox="0 0 20 20" fill="currentColor">
        <path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 001.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z" />
      </svg>
      <span class="text-[10px] font-medium">Home</span>
    </RouterLink>

    <!-- Voice — only active when orgId is known -->
    <RouterLink
      v-if="currentOrgId"
      :to="`/orgs/${currentOrgId}/voice`"
      class="flex flex-col items-center justify-center gap-0.5 min-w-[56px] min-h-[44px] px-2 transition-colors"
      :class="route.name === 'voice-session-list' ? 'text-indigo-400' : 'text-slate-400'"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 shrink-0" viewBox="0 0 20 20" fill="currentColor">
        <path fill-rule="evenodd" d="M7 4a3 3 0 016 0v4a3 3 0 11-6 0V4zm4 10.93A7.001 7.001 0 0017 8a1 1 0 10-2 0A5 5 0 015 8a1 1 0 00-2 0 7.001 7.001 0 006 6.93V17H6a1 1 0 100 2h8a1 1 0 100-2h-3v-2.07z" clip-rule="evenodd" />
      </svg>
      <span class="text-[10px] font-medium">Voice</span>
    </RouterLink>
    <span
      v-else
      class="flex flex-col items-center justify-center gap-0.5 min-w-[56px] min-h-[44px] px-2 text-slate-600"
      aria-disabled="true"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 shrink-0" viewBox="0 0 20 20" fill="currentColor">
        <path fill-rule="evenodd" d="M7 4a3 3 0 016 0v4a3 3 0 11-6 0V4zm4 10.93A7.001 7.001 0 0017 8a1 1 0 10-2 0A5 5 0 015 8a1 1 0 00-2 0 7.001 7.001 0 006 6.93V17H6a1 1 0 100 2h8a1 1 0 100-2h-3v-2.07z" clip-rule="evenodd" />
      </svg>
      <span class="text-[10px] font-medium">Voice</span>
    </span>

    <!-- Calls -->
    <RouterLink
      v-if="currentOrgId"
      :to="`/orgs/${currentOrgId}/whatsapp/calls`"
      class="flex flex-col items-center justify-center gap-0.5 min-w-[56px] min-h-[44px] px-2 transition-colors"
      :class="route.name === 'whatsapp-calls' ? 'text-indigo-400' : 'text-slate-400'"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 shrink-0" viewBox="0 0 20 20" fill="currentColor">
        <path d="M2 3a1 1 0 011-1h2.153a1 1 0 01.986.836l.74 4.435a1 1 0 01-.54 1.06l-1.548.773a11.037 11.037 0 006.105 6.105l.774-1.548a1 1 0 011.059-.54l4.435.74a1 1 0 01.836.986V17a1 1 0 01-1 1h-2C7.82 18 2 12.18 2 5V3z" />
      </svg>
      <span class="text-[10px] font-medium">Calls</span>
    </RouterLink>
    <span
      v-else
      class="flex flex-col items-center justify-center gap-0.5 min-w-[56px] min-h-[44px] px-2 text-slate-600"
      aria-disabled="true"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 shrink-0" viewBox="0 0 20 20" fill="currentColor">
        <path d="M2 3a1 1 0 011-1h2.153a1 1 0 01.986.836l.74 4.435a1 1 0 01-.54 1.06l-1.548.773a11.037 11.037 0 006.105 6.105l.774-1.548a1 1 0 011.059-.54l4.435.74a1 1 0 01.836.986V17a1 1 0 01-1 1h-2C7.82 18 2 12.18 2 5V3z" />
      </svg>
      <span class="text-[10px] font-medium">Calls</span>
    </span>

    <!-- Numbers -->
    <RouterLink
      v-if="currentOrgId"
      :to="`/orgs/${currentOrgId}/whatsapp/phone-numbers`"
      class="flex flex-col items-center justify-center gap-0.5 min-w-[56px] min-h-[44px] px-2 transition-colors"
      :class="route.name === 'whatsapp-phone-numbers' ? 'text-indigo-400' : 'text-slate-400'"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 shrink-0" viewBox="0 0 20 20" fill="currentColor">
        <path d="M17.924 2.617a.997.997 0 00-.215-.322l-.004-.004A.997.997 0 0017 2h-4a1 1 0 100 2h1.586l-3.293 3.293a1 1 0 001.414 1.414L16 5.414V7a1 1 0 102 0V3a.997.997 0 00-.076-.383zM2 3a1 1 0 011-1h2.153a1 1 0 01.986.836l.74 4.435a1 1 0 01-.54 1.06l-1.548.773a11.037 11.037 0 006.105 6.105l.774-1.548a1 1 0 011.059-.54l4.435.74a1 1 0 01.836.986V17a1 1 0 01-1 1h-2C7.82 18 2 12.18 2 5V3z" />
      </svg>
      <span class="text-[10px] font-medium">Numbers</span>
    </RouterLink>
    <span
      v-else
      class="flex flex-col items-center justify-center gap-0.5 min-w-[56px] min-h-[44px] px-2 text-slate-600"
      aria-disabled="true"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 shrink-0" viewBox="0 0 20 20" fill="currentColor">
        <path d="M17.924 2.617a.997.997 0 00-.215-.322l-.004-.004A.997.997 0 0017 2h-4a1 1 0 100 2h1.586l-3.293 3.293a1 1 0 001.414 1.414L16 5.414V7a1 1 0 102 0V3a.997.997 0 00-.076-.383zM2 3a1 1 0 011-1h2.153a1 1 0 01.986.836l.74 4.435a1 1 0 01-.54 1.06l-1.548.773a11.037 11.037 0 006.105 6.105l.774-1.548a1 1 0 011.059-.54l4.435.74a1 1 0 01.836.986V17a1 1 0 01-1 1h-2C7.82 18 2 12.18 2 5V3z" />
      </svg>
      <span class="text-[10px] font-medium">Numbers</span>
    </span>

    <!-- More -->
    <button
      class="flex flex-col items-center justify-center gap-0.5 min-w-[56px] min-h-[44px] px-2 transition-colors"
      :class="moreSheetOpen ? 'text-indigo-400' : 'text-slate-400'"
      aria-label="More navigation options"
      @click="moreSheetOpen = true"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 shrink-0" viewBox="0 0 20 20" fill="currentColor">
        <path d="M6 10a2 2 0 11-4 0 2 2 0 014 0zM12 10a2 2 0 11-4 0 2 2 0 014 0zM16 12a2 2 0 100-4 2 2 0 000 4z" />
      </svg>
      <span class="text-[10px] font-medium">More</span>
    </button>
  </nav>

  <BottomSheet :open="moreSheetOpen" title="Menu" @close="moreSheetOpen = false">
    <div class="px-2 pb-4">
      <RouterLink
        v-for="item in moreItems"
        :key="item.name"
        :to="item.to"
        class="flex items-center gap-3 w-full min-h-[48px] px-4 rounded-xl text-slate-300 hover:bg-slate-700 transition-colors"
        @click="moreSheetOpen = false"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-slate-400 shrink-0" viewBox="0 0 20 20" fill="currentColor">
          <path :d="item.iconPath" />
        </svg>
        <span class="text-sm font-medium">{{ item.label }}</span>
      </RouterLink>
    </div>
  </BottomSheet>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import BottomSheet from './BottomSheet.vue'

const route = useRoute()
const moreSheetOpen = ref(false)

const currentOrgId = computed(() => route.params.orgId as string | undefined)

const moreItems = computed(() => [
  {
    name: 'knowledge-bases',
    label: 'Knowledge Bases',
    to: currentOrgId.value ? `/orgs/${currentOrgId.value}/workspaces` : '/dashboard',
    iconPath: 'M9 4.804A7.968 7.968 0 005.5 4c-1.255 0-2.443.29-3.5.804v10A7.969 7.969 0 015.5 14c1.669 0 3.218.51 4.5 1.385A7.962 7.962 0 0114.5 14c1.255 0 2.443.29 3.5.804v-10A7.968 7.968 0 0014.5 4c-1.255 0-2.443.29-3.5.804V12a1 1 0 11-2 0V4.804z',
  },
  {
    name: 'chatbot-config',
    label: 'Chatbot Config',
    to: '/chatbot-config',
    iconPath: 'M2 5a2 2 0 012-2h7a2 2 0 012 2v4a2 2 0 01-2 2H9l-3 3v-3H4a2 2 0 01-2-2V5z',
  },
  {
    name: 'analytics',
    label: 'Analytics',
    to: '/analytics',
    iconPath: 'M2 11a1 1 0 011-1h2a1 1 0 011 1v5a1 1 0 01-1 1H3a1 1 0 01-1-1v-5zM8 7a1 1 0 011-1h2a1 1 0 011 1v9a1 1 0 01-1 1H9a1 1 0 01-1-1V7zM14 4a1 1 0 011-1h2a1 1 0 011 1v12a1 1 0 01-1 1h-2a1 1 0 01-1-1V4z',
  },
  {
    name: 'api-keys',
    label: 'API Keys',
    to: '/api-keys',
    iconPath: 'M15 7a1 1 0 011 1v4a1 1 0 01-2 0V9.414l-4.293 4.293a1 1 0 01-1.414 0L7 12.414V13a1 1 0 01-2 0V9a1 1 0 01.293-.707l2-2a1 1 0 011.414 1.414L7.414 9H8a1 1 0 010 2h-.586l4.293 4.293 2.293-2.293A1 1 0 0115 7z',
  },
  {
    name: 'llm-providers',
    label: 'LLM Providers',
    to: '/llm-providers',
    iconPath: 'M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z',
  },
  {
    name: 'sandbox',
    label: 'Sandbox',
    to: '/sandbox',
    iconPath: 'M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z',
  },
  {
    name: 'billing',
    label: 'Billing',
    to: currentOrgId.value ? `/orgs/${currentOrgId.value}/billing` : '/dashboard',
    iconPath: 'M4 4a2 2 0 00-2 2v4a2 2 0 002 2V6h10a2 2 0 00-2-2H4zm2 6a2 2 0 012-2h8a2 2 0 012 2v4a2 2 0 01-2 2H8a2 2 0 01-2-2v-4zm6 4a2 2 0 100-4 2 2 0 000 4z',
  },
])
</script>
