<script setup lang="ts">
// Marketplace list — search + license filter + sort + infinite-scroll grid.
// Login is enforced by the route guard (meta.requiresAuth). Per plan §7,
// anonymous browsing is explicitly deferred.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  listMarketplace,
  SPDX_LICENSES,
  type MarketplaceListItem,
  type MarketplaceSort,
} from '../../api/marketplace'
import MarketplaceKbCard from '../../components/marketplace/MarketplaceKbCard.vue'
import MarketplaceImportDialog from '../../components/marketplace/MarketplaceImportDialog.vue'
import { useRouter } from 'vue-router'

const PAGE_LIMIT = 20

const router = useRouter()

const query = ref('')
const sort = ref<MarketplaceSort>('recently_updated')
const licenseFilter = ref<string[]>([])

const items = ref<MarketplaceListItem[]>([])
const loading = ref(false)
const loadingMore = ref(false)
const errorMessage = ref('')
const offset = ref(0)
const hasMore = ref(false)

const sentinel = ref<HTMLDivElement | null>(null)
let observer: IntersectionObserver | null = null

// Import dialog state — list view supports import from cards in the future,
// but for now is opened only from the card's RouterLink → detail page. We
// still mount it here so deep-link "import this kb" works in a follow-up.
const importDialogOpen = ref(false)
const importPublicKbId = ref('')

const showEmptyState = computed(
  () => !loading.value && items.value.length === 0 && !errorMessage.value,
)

async function loadFirstPage() {
  loading.value = true
  errorMessage.value = ''
  offset.value = 0
  items.value = []
  hasMore.value = false
  try {
    const res = await listMarketplace({
      q: query.value.trim() || undefined,
      sort: sort.value,
      license: licenseFilter.value.length > 0 ? licenseFilter.value : undefined,
      limit: PAGE_LIMIT,
      offset: 0,
    })
    const next = Array.isArray(res.items) ? res.items : []
    items.value = next
    hasMore.value = next.length === PAGE_LIMIT
    offset.value = next.length
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : 'Failed to load marketplace.'
  } finally {
    loading.value = false
  }
}

async function loadMore() {
  if (loading.value || loadingMore.value || !hasMore.value) return
  loadingMore.value = true
  try {
    const res = await listMarketplace({
      q: query.value.trim() || undefined,
      sort: sort.value,
      license: licenseFilter.value.length > 0 ? licenseFilter.value : undefined,
      limit: PAGE_LIMIT,
      offset: offset.value,
    })
    const next = Array.isArray(res.items) ? res.items : []
    items.value.push(...next)
    hasMore.value = next.length === PAGE_LIMIT
    offset.value += next.length
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : 'Failed to load more results.'
  } finally {
    loadingMore.value = false
  }
}

// Debounce search input so we don't fire on every keystroke.
let searchTimer: ReturnType<typeof setTimeout> | null = null
watch(query, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(loadFirstPage, 250)
})
watch([sort, licenseFilter], loadFirstPage, { deep: true })

function toggleLicense(license: string) {
  const idx = licenseFilter.value.indexOf(license)
  if (idx === -1) {
    licenseFilter.value = [...licenseFilter.value, license]
  } else {
    licenseFilter.value = licenseFilter.value.filter((l) => l !== license)
  }
}

onMounted(() => {
  loadFirstPage()
  if (sentinel.value) {
    observer = new IntersectionObserver((entries) => {
      if (entries[0]?.isIntersecting) loadMore()
    })
    observer.observe(sentinel.value)
  }
})

onBeforeUnmount(() => {
  observer?.disconnect()
  if (searchTimer) clearTimeout(searchTimer)
})

function onImported(payload: { orgId: string; workspaceId: string; kbId: string }) {
  importDialogOpen.value = false
  router.push(
    `/orgs/${payload.orgId}/workspaces/${payload.workspaceId}/knowledge-bases/${payload.kbId}`,
  )
}
</script>

<template>
  <div class="mx-auto max-w-7xl px-4 py-6" data-test="marketplace-list-view">
    <header class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900">Marketplace</h1>
      <p class="text-sm text-gray-500">
        Browse Public Knowledge Bases published by other organizations.
      </p>
    </header>

    <!-- Filter bar — inlined here because the list view is the only consumer.
         Extract to its own SFC if filtering grows beyond search + sort + license. -->
    <section class="mb-6 space-y-3 rounded-lg border border-gray-200 bg-white p-4">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
        <input
          v-model="query"
          type="search"
          placeholder="Search KBs by name or description…"
          class="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          data-test="marketplace-search-input"
        />
        <select
          v-model="sort"
          class="rounded-md border border-gray-300 bg-white px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          data-test="marketplace-sort"
        >
          <option value="recently_updated">Recently updated</option>
          <option value="newest">Newest</option>
          <option value="most_imported">Most imported</option>
          <option value="alphabetic">Alphabetic</option>
        </select>
      </div>
      <div class="flex flex-wrap items-center gap-2 text-xs">
        <span class="text-gray-500">License:</span>
        <button
          v-for="lic in SPDX_LICENSES"
          :key="lic"
          type="button"
          class="rounded-full border px-2.5 py-0.5"
          :class="licenseFilter.includes(lic)
            ? 'border-indigo-500 bg-indigo-50 text-indigo-700'
            : 'border-gray-300 bg-white text-gray-700 hover:bg-gray-50'"
          :data-test="`marketplace-license-filter-${lic}`"
          @click="toggleLicense(lic)"
        >
          {{ lic }}
        </button>
        <button
          v-if="licenseFilter.length > 0"
          type="button"
          class="ml-2 text-xs text-indigo-600 hover:text-indigo-800"
          @click="licenseFilter = []"
        >
          Clear
        </button>
      </div>
    </section>

    <div
      v-if="errorMessage"
      class="mb-4 rounded-md bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700"
      data-test="marketplace-list-error"
    >
      {{ errorMessage }}
    </div>

    <div v-if="loading" class="py-12 text-center text-sm text-gray-500">
      Loading marketplace…
    </div>
    <div
      v-else-if="showEmptyState"
      class="py-12 text-center text-sm text-gray-500"
      data-test="marketplace-empty-state"
    >
      No Public KBs match your filters yet.
    </div>
    <div
      v-else
      class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3"
      data-test="marketplace-grid"
    >
      <MarketplaceKbCard v-for="item in items" :key="item.kb_id" :item="item" />
    </div>

    <div ref="sentinel" class="h-8" data-test="marketplace-sentinel"></div>
    <div v-if="loadingMore" class="py-4 text-center text-xs text-gray-500">
      Loading more…
    </div>

    <MarketplaceImportDialog
      :open="importDialogOpen"
      :public-kb-id="importPublicKbId"
      @close="importDialogOpen = false"
      @imported="onImported"
    />

    <!-- DMCA footer link (issue #736, ADR-0006 launch blocker). The link
         is intentionally minimal — operational details live at
         /legal/dmca. -->
    <footer class="mt-8 border-t border-gray-200 pt-4 text-xs text-gray-500">
      <RouterLink to="/legal/dmca" class="text-indigo-600 hover:underline">
        DMCA policy
      </RouterLink>
    </footer>
  </div>
</template>
