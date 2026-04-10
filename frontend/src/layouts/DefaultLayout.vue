<template>
  <div class="flex h-screen bg-gray-100">
    <AppSidebar v-if="!isMobile" :mobile="false" :open="false" />
    <div class="flex flex-1 flex-col overflow-hidden">
      <AppHeader />
      <main class="flex-1 overflow-y-auto p-4 md:p-6" :class="isMobile ? 'pb-24' : ''">
        <RouterView />
      </main>
    </div>
    <MobileTabBar v-if="isMobile" />
    <UpgradePrompt
      :open="billingStore.upgradePromptOpen"
      :feature="billingStore.upgradeFeature"
      @close="billingStore.hideUpgradePrompt()"
    />
  </div>
</template>

<script setup lang="ts">
import { RouterView } from 'vue-router'
import AppSidebar from '../components/AppSidebar.vue'
import AppHeader from '../components/AppHeader.vue'
import MobileTabBar from '../components/MobileTabBar.vue'
import UpgradePrompt from '../components/UpgradePrompt.vue'
import { useMobile } from '../composables/useMediaQuery'
import { useBillingStore } from '../stores/billing'

const { isMobile } = useMobile()
const billingStore = useBillingStore()
</script>
