<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { getOrg, type Org } from '../../api/orgs'

const route = useRoute()
const currentOrg = ref<Org | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

async function fetchOrg(orgId: string) {
  loading.value = true
  error.value = null
  try {
    currentOrg.value = await getOrg(orgId)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

onMounted(() => fetchOrg(route.params.orgId as string))
</script>

<template>
  <div class="p-6">
    <div v-if="loading" class="text-gray-500">Loading…</div>
    <div v-else-if="error" class="text-red-600">{{ error }}</div>
    <div v-else-if="currentOrg">
      <h1 class="text-2xl font-bold">{{ currentOrg.name }}</h1>
      <p class="text-sm text-gray-500">{{ currentOrg.slug }}</p>
      <span
        class="inline-block mt-2 px-2 py-0.5 rounded text-xs"
        :class="currentOrg.status === 'active' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'"
      >
        {{ currentOrg.status }}
      </span>
    </div>
  </div>
</template>
