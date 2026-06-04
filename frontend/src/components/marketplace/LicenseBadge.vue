<script setup lang="ts">
// LicenseBadge — SPDX license chip with a tooltip explaining the license.
// Per ADR-0006, only the 7 curated SPDX ids appear on Public KBs; unknown
// ids still render but fall through to a generic description so we never
// blank-render an unfamiliar identifier (defensive — a backend that lands
// a new allow-list entry should still be displayable).
import { computed } from 'vue'

const props = defineProps<{
  /** SPDX identifier (e.g. "CC-BY-4.0"). May be empty when the row is mid-publish. */
  license: string | null | undefined
  /** Optional override — default sm for cards, md for the detail header. */
  size?: 'sm' | 'md'
}>()

const SPDX_INFO: Record<string, { label: string; description: string }> = {
  'CC0-1.0': {
    label: 'CC0 1.0',
    description: 'No rights reserved. Use freely without attribution.',
  },
  'CC-BY-4.0': {
    label: 'CC BY 4.0',
    description: 'Free to use with attribution to the publisher.',
  },
  'CC-BY-SA-4.0': {
    label: 'CC BY-SA 4.0',
    description: 'Free to use with attribution; derivatives must use the same license.',
  },
  'CC-BY-NC-4.0': {
    label: 'CC BY-NC 4.0',
    description: 'Free for non-commercial use with attribution.',
  },
  MIT: {
    label: 'MIT',
    description: 'Permissive open-source license — minimal restrictions, attribution required.',
  },
  'Apache-2.0': {
    label: 'Apache 2.0',
    description: 'Permissive open-source license with explicit patent grant.',
  },
  'GPL-3.0': {
    label: 'GPL 3.0',
    description: 'Copyleft — derivatives must also be GPL-3.0 licensed.',
  },
}

const info = computed(() => {
  const id = props.license ?? ''
  return (
    SPDX_INFO[id] ?? {
      label: id || 'Unlicensed',
      description: id
        ? `SPDX identifier ${id}. See the publisher for terms.`
        : 'No license metadata available.',
    }
  )
})

const sizeClass = computed(() =>
  (props.size ?? 'sm') === 'md'
    ? 'text-xs px-2.5 py-1'
    : 'text-[11px] px-2 py-0.5',
)
</script>

<template>
  <span
    class="inline-flex items-center rounded-full border border-indigo-200 bg-indigo-50 font-medium text-indigo-700"
    :class="sizeClass"
    :title="info.description"
    data-test="license-badge"
  >
    {{ info.label }}
  </span>
</template>
