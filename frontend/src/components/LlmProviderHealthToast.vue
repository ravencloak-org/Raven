<template>
  <Transition
    enter-active-class="transition ease-out duration-200"
    enter-from-class="opacity-0 translate-y-2"
    enter-to-class="opacity-100 translate-y-0"
    leave-active-class="transition ease-in duration-150"
    leave-from-class="opacity-100 translate-y-0"
    leave-to-class="opacity-0 translate-y-2"
  >
    <!--
      Toast is rendered exactly when the polling cron reports the default
      LLM provider as unhealthy. It is intentionally not dismissable —
      hiding the warning would defeat its purpose, since the chat path is
      genuinely broken until the operator visits /llm-providers and fixes
      the underlying connection.
    -->
    <div
      v-if="visible"
      role="alert"
      aria-live="assertive"
      class="pointer-events-auto fixed right-4 top-4 z-50 w-full max-w-sm rounded-lg border border-red-200 bg-white shadow-lg ring-1 ring-red-100"
      data-testid="llm-health-toast"
    >
      <div class="flex items-start gap-3 p-4">
        <div class="flex-shrink-0">
          <svg
            class="h-5 w-5 text-red-500"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            aria-hidden="true"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M12 9v2m0 4h.01M5 19h14a2 2 0 001.84-2.75L13.74 4a2 2 0 00-3.48 0L3.16 16.25A2 2 0 005 19z"
            />
          </svg>
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-semibold text-gray-900">LLM provider unreachable</p>
          <p class="mt-1 break-words text-sm text-gray-600">
            {{ health.failureReason() }}
          </p>
          <div class="mt-3">
            <button
              type="button"
              class="inline-flex items-center rounded-md bg-red-600 px-3 py-1.5 text-sm font-medium text-white shadow-sm hover:bg-red-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-red-500 focus-visible:ring-offset-2"
              data-testid="llm-health-toast-action"
              @click="goToProviders"
            >
              View providers
              <svg
                class="ml-1.5 h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                aria-hidden="true"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 5l7 7-7 7"
                />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useLlmProviderHealthStore } from '../stores/llm-provider-health'

const health = useLlmProviderHealthStore()
const router = useRouter()
const route = useRoute()

// Hide the toast when the user is already on the LLM providers page,
// since the in-page banner there covers the same information. Otherwise
// the toast would overlap the page banner and read as duplicate noise.
const visible = computed(
  () => !health.isHealthy() && route.name !== 'llm-providers',
)

function goToProviders() {
  void router.push({ name: 'llm-providers' })
}
</script>
