<script setup lang="ts">
// MarketplaceKbCard — single Public KB tile rendered in the listing grid.
// Self-contained (no API calls of its own) so the card can be re-used in
// tests, the related-KB rail (future), and the detail page header.
import { computed } from 'vue'
import type { MarketplaceListItem } from '../../api/marketplace'
import LicenseBadge from './LicenseBadge.vue'
import ReportButton from './ReportButton.vue'

const props = defineProps<{
  item: MarketplaceListItem
}>()

const updatedLabel = computed(() => formatRelative(props.item.last_modified_at))

function formatRelative(iso: string): string {
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) return ''
  const seconds = Math.floor((Date.now() - then) / 1000)
  if (seconds < 60) return 'Updated just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `Updated ${minutes} minute${minutes === 1 ? '' : 's'} ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `Updated ${hours} hour${hours === 1 ? '' : 's'} ago`
  const days = Math.floor(hours / 24)
  if (days < 30) return `Updated ${days} day${days === 1 ? '' : 's'} ago`
  const months = Math.floor(days / 30)
  if (months < 12) return `Updated ${months} month${months === 1 ? '' : 's'} ago`
  const years = Math.floor(days / 365)
  return `Updated ${years} year${years === 1 ? '' : 's'} ago`
}
</script>

<template>
  <article
    class="group flex h-full flex-col rounded-xl border border-gray-200 bg-white p-5 shadow-sm transition hover:border-indigo-300 hover:shadow-md"
    data-test="marketplace-card"
    :data-kb-id="item.kb_id"
  >
    <header class="flex items-start justify-between gap-3">
      <RouterLink
        :to="`/marketplace/${item.org_slug}/${item.kb_slug}`"
        class="min-w-0 flex-1"
      >
        <h3 class="truncate text-base font-semibold text-gray-900 group-hover:text-indigo-700">
          {{ item.name }}
        </h3>
        <p class="mt-0.5 truncate text-xs text-gray-500">
          by
          <span class="font-medium text-gray-700">{{ item.org_display_name }}</span>
          <span class="text-gray-400"> · {{ item.org_slug }}</span>
        </p>
      </RouterLink>
      <LicenseBadge :license="item.license_spdx_id" />
    </header>

    <p
      v-if="item.description"
      class="mt-3 line-clamp-3 text-sm text-gray-600"
    >
      {{ item.description }}
    </p>

    <footer class="mt-auto pt-4 flex items-center justify-between text-xs text-gray-500">
      <div class="flex items-center gap-3">
        <span data-test="marketplace-card-updated">{{ updatedLabel }}</span>
        <span aria-hidden="true">·</span>
        <span data-test="marketplace-card-imports">
          {{ item.import_count }} import{{ item.import_count === 1 ? '' : 's' }}
        </span>
      </div>
      <ReportButton :kb-id="item.kb_id" compact />
    </footer>
  </article>
</template>
