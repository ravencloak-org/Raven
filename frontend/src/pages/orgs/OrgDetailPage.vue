<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useOrgsStore } from '../../stores/orgs'

const route = useRoute()
const store = useOrgsStore()
onMounted(() => store.fetchOrg(route.params.orgId as string))
</script>

<template>
  <div class="p-6">
    <div v-if="store.loading" class="text-gray-500">Loading…</div>
    <div v-else-if="store.error" class="text-red-600">{{ store.error }}</div>
    <div v-else-if="store.currentOrg">
      <h1 class="text-2xl font-bold">{{ store.currentOrg.name }}</h1>
      <p class="text-sm text-gray-500">{{ store.currentOrg.slug }}</p>
      <span
        class="inline-block mt-2 px-2 py-0.5 rounded text-xs"
        :class="store.currentOrg.status === 'active' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'"
      >
        {{ store.currentOrg.status }}
      </span>
    </div>
  </div>
</template>
