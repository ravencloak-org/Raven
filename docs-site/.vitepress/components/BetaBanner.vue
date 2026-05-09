<script setup lang="ts">
import { useData } from "vitepress";
import { computed } from "vue";

const { page, frontmatter } = useData();

// Show on every /api/* page UNLESS the operation declares x-status: stable.
// Operations are surfaced by vitepress-openapi as front-matter; we also
// allow a manual override via `betaBanner: false`.
const isApiPage = computed(() => page.value.relativePath.startsWith("api/"));
const stable = computed(() => frontmatter.value?.["x-status"] === "stable");
const muted = computed(() => frontmatter.value?.betaBanner === false);
const visible = computed(() => isApiPage.value && !stable.value && !muted.value);
</script>

<template>
  <div v-if="visible" class="beta-banner">
    <strong>Beta.</strong>
    The API surface is documented incrementally. Operations marked
    <code>x-status: stable</code> are guaranteed; everything else may change
    without notice.
  </div>
</template>

<style scoped>
.beta-banner {
  margin: 0 0 1.5rem;
  padding: 0.75rem 1rem;
  border-left: 4px solid var(--vp-c-brand-1);
  background: var(--vp-c-brand-soft);
  color: var(--vp-c-text-1);
  font-size: 0.95em;
  border-radius: 4px;
}
.beta-banner code {
  font-size: 0.9em;
  padding: 0.05em 0.35em;
  background: var(--vp-c-bg-soft);
  border-radius: 3px;
}
</style>
