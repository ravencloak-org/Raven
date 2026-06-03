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
    <!-- Onboarding overlay only shows for users without an org -->
    <OnboardingWizard v-if="!authStore.hasOrg && !onboardingStore.completed" />
    <!--
      Persistent toast that fires when the org's default LLM provider
      probe fails. Mounted at layout level so it follows the user across
      every authenticated route — the chat path the toast warns about
      is global, so the warning should be too.
    -->
    <LlmProviderHealthToast />
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, watch } from 'vue'
import { RouterView } from 'vue-router'
import AppSidebar from '../components/AppSidebar.vue'
import AppHeader from '../components/AppHeader.vue'
import MobileTabBar from '../components/MobileTabBar.vue'
import UpgradePrompt from '../components/UpgradePrompt.vue'
import LlmProviderHealthToast from '../components/LlmProviderHealthToast.vue'
import OnboardingWizard from '../pages/onboarding/OnboardingWizard.vue'
import { useMobile } from '../composables/useMediaQuery'
import { useAuthStore } from '../stores/auth'
import { useBillingStore } from '../stores/billing'
import { useOnboardingStore } from '../stores/onboarding'
import { useLlmProviderHealthStore } from '../stores/llm-provider-health'

const { isMobile } = useMobile()
const authStore = useAuthStore()
const billingStore = useBillingStore()
const onboardingStore = useOnboardingStore()
const healthStore = useLlmProviderHealthStore()

// Drive the LLM-provider health cron off the auth store's orgId. Using
// a watcher (immediate) instead of an onMounted hook means we also pick
// up the case where orgId becomes available AFTER mount — e.g. the
// cold-boot path where DefaultLayout renders before authStore.init()
// finishes refetching the org from /api/v1/me.
const stopWatch = watch(
  () => authStore.orgId,
  (orgId) => {
    if (orgId) {
      healthStore.start(orgId)
    } else {
      healthStore.stop()
      healthStore.resetState()
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  stopWatch()
  healthStore.stop()
})
</script>
