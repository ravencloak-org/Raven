<template>
  <div class="flex h-screen bg-gray-100">
    <AppSidebar
      :mobile="isMobile"
      :open="sidebarOpen"
      @close="sidebarOpen = false"
    />
    <div class="flex flex-1 flex-col overflow-hidden">
      <AppHeader @toggle-sidebar="sidebarOpen = !sidebarOpen" />
      <main class="flex-1 overflow-y-auto p-4 md:p-6">
        <RouterView />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { RouterView } from 'vue-router'
import { useRoute } from 'vue-router'
import AppSidebar from '../components/AppSidebar.vue'
import AppHeader from '../components/AppHeader.vue'
import { useMobile } from '../composables/useMediaQuery'

const { isMobile } = useMobile()
const sidebarOpen = ref(false)
const route = useRoute()

// Close sidebar on route change (mobile)
watch(
  () => route.path,
  () => {
    if (isMobile.value) {
      sidebarOpen.value = false
    }
  },
)
</script>
