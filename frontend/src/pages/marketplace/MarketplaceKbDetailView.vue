<script setup lang="ts">
// Marketplace KB detail page.
// Loads `/marketplace/{org_slug}/{kb_slug}`. The 410 Gone branch renders
// a dedicated "this KB was unpublished" state (ADR-0007); 404 falls
// through to a generic missing-KB state.
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  getMarketplaceKb,
  MarketplaceApiError,
  type MarketplaceKbDetail,
} from '../../api/marketplace'
import LicenseBadge from '../../components/marketplace/LicenseBadge.vue'
import ReportButton from '../../components/marketplace/ReportButton.vue'
import MarketplacePreviewDialog from '../../components/marketplace/MarketplacePreviewDialog.vue'
import MarketplaceImportDialog from '../../components/marketplace/MarketplaceImportDialog.vue'

const route = useRoute()
const router = useRouter()

const detail = ref<MarketplaceKbDetail | null>(null)
const loading = ref(false)
const errorMessage = ref('')
const errorStatus = ref<number | null>(null)
const previewOpen = ref(false)
const importOpen = ref(false)

const orgSlug = computed(() => String(route.params.orgSlug))
const kbSlug = computed(() => String(route.params.kbSlug))

const isGone = computed(() => errorStatus.value === 410)
const isMissing = computed(() => errorStatus.value === 404)

async function load() {
  loading.value = true
  errorMessage.value = ''
  errorStatus.value = null
  detail.value = null
  try {
    detail.value = await getMarketplaceKb(orgSlug.value, kbSlug.value)
  } catch (err) {
    if (err instanceof MarketplaceApiError) {
      errorStatus.value = err.status
      errorMessage.value = err.message
    } else {
      errorMessage.value = err instanceof Error ? err.message : 'Failed to load KB.'
    }
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch([orgSlug, kbSlug], load)

function onImported(payload: { orgId: string; workspaceId: string; kbId: string }) {
  importOpen.value = false
  router.push(
    `/orgs/${payload.orgId}/workspaces/${payload.workspaceId}/knowledge-bases/${payload.kbId}`,
  )
}

function openImport() {
  previewOpen.value = false
  importOpen.value = true
}
</script>

<template>
  <div class="mx-auto max-w-4xl px-4 py-6" data-test="marketplace-detail-view">
    <RouterLink
      to="/marketplace"
      class="mb-4 inline-block text-sm text-indigo-600 hover:text-indigo-800"
    >
      &larr; Back to marketplace
    </RouterLink>

    <div v-if="loading" class="py-12 text-center text-sm text-gray-500">Loading KB…</div>

    <!-- 410 Gone — unpublished within the 90-day slug hold (ADR-0007). -->
    <div
      v-else-if="isGone"
      class="rounded-xl border border-yellow-200 bg-yellow-50 p-8 text-center"
      data-test="marketplace-detail-gone"
    >
      <h2 class="text-lg font-semibold text-yellow-900">This KB was unpublished</h2>
      <p class="mt-2 text-sm text-yellow-800">
        The publisher took this KB off the marketplace. Existing imports continue to work; the slug
        is reserved for 90 days before it can be reused.
      </p>
      <RouterLink
        to="/marketplace"
        class="mt-4 inline-block rounded-md bg-yellow-600 px-4 py-2 text-sm font-medium text-white hover:bg-yellow-700"
      >
        Browse other KBs
      </RouterLink>
    </div>

    <div
      v-else-if="isMissing"
      class="rounded-xl border border-gray-200 bg-white p-8 text-center"
      data-test="marketplace-detail-missing"
    >
      <h2 class="text-lg font-semibold text-gray-900">KB not found</h2>
      <p class="mt-2 text-sm text-gray-600">
        We couldn't find a public KB at that address.
      </p>
    </div>

    <div
      v-else-if="errorMessage"
      class="rounded-md bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700"
    >
      {{ errorMessage }}
    </div>

    <article v-else-if="detail" class="space-y-6">
      <header class="space-y-2">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <h1 class="text-2xl font-bold text-gray-900">{{ detail.name }}</h1>
            <p class="mt-1 text-sm text-gray-500">
              by
              <span class="font-medium text-gray-800">{{ detail.org_display_name }}</span>
              <span class="text-gray-400"> · {{ detail.org_slug }}</span>
            </p>
          </div>
          <LicenseBadge :license="detail.license_spdx_id" size="md" />
        </div>
        <p
          v-if="detail.source_org_slug"
          class="text-xs text-gray-500"
          data-test="marketplace-detail-fork-line"
        >
          Forked from
          <RouterLink
            :to="`/marketplace/${detail.source_org_slug}/`"
            class="text-indigo-600 hover:text-indigo-800"
          >
            {{ detail.source_org_display_name ?? detail.source_org_slug }}
          </RouterLink>
        </p>
      </header>

      <section class="flex flex-wrap gap-3">
        <button
          type="button"
          class="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
          data-test="marketplace-detail-preview"
          @click="previewOpen = true"
        >
          Preview
        </button>
        <button
          type="button"
          class="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
          data-test="marketplace-detail-import"
          @click="importOpen = true"
        >
          Import
        </button>
        <ReportButton :kb-id="detail.kb_id" />
      </section>

      <section v-if="detail.description" class="prose prose-sm max-w-none">
        <!-- Description is rendered as plain text; the backend does not return
             rendered markdown today. When/if a server-side renderer lands,
             swap this for a sanitised v-html wrapper. -->
        <pre class="whitespace-pre-wrap text-sm text-gray-700">{{ detail.description }}</pre>
      </section>

      <section
        class="grid grid-cols-2 gap-4 rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm sm:grid-cols-4"
        data-test="marketplace-detail-counts"
      >
        <div>
          <dt class="text-xs text-gray-500">Sources</dt>
          <dd class="font-medium text-gray-900">{{ detail.source_count ?? '—' }}</dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500">Documents</dt>
          <dd class="font-medium text-gray-900">{{ detail.document_count ?? '—' }}</dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500">Chunks</dt>
          <dd class="font-medium text-gray-900">{{ detail.chunk_count ?? '—' }}</dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500">Imports</dt>
          <dd class="font-medium text-gray-900">{{ detail.import_count }}</dd>
        </div>
      </section>

      <MarketplacePreviewDialog
        v-if="detail"
        :open="previewOpen"
        :org-slug="detail.org_slug"
        :kb-slug="detail.kb_slug"
        @close="previewOpen = false"
        @import="openImport"
      />

      <MarketplaceImportDialog
        v-if="detail"
        :open="importOpen"
        :public-kb-id="detail.kb_id"
        @close="importOpen = false"
        @imported="onImported"
      />
    </article>
  </div>
</template>
