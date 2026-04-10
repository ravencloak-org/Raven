<template>
  <div class="flex h-screen bg-gray-100">
    <AppSidebar v-if="!isMobile" :mobile="false" :open="false" />
    <div class="flex flex-1 flex-col overflow-hidden">
      <AppHeader />
      <UpgradePromptBanner />
      <main class="flex-1 overflow-y-auto p-4 md:p-6" :class="isMobile ? 'pb-24' : ''">
        <RouterView />
      </main>
    </div>
    <MobileTabBar v-if="isMobile" />
  </div>
</template>

<script setup lang="ts">
import { RouterView } from 'vue-router'
import AppSidebar from '../components/AppSidebar.vue'
import AppHeader from '../components/AppHeader.vue'
import MobileTabBar from '../components/MobileTabBar.vue'
import UpgradePromptBanner from '../components/UpgradePromptBanner.vue'
import { useMobile } from '../composables/useMediaQuery'

const { isMobile } = useMobile()
</script>
