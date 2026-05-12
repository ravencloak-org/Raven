<template>
  <RouterView />
  <CookieBanner />
</template>

<script setup lang="ts">
import { RouterView } from 'vue-router'
import CookieBanner from './components/CookieBanner.vue'
</script>
