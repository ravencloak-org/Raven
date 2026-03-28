<template>
  <Transition
    enter-active-class="transition duration-300 ease-out"
    enter-from-class="translate-y-full opacity-0"
    enter-to-class="translate-y-0 opacity-100"
    leave-active-class="transition duration-200 ease-in"
    leave-from-class="translate-y-0 opacity-100"
    leave-to-class="translate-y-full opacity-0"
  >
    <div
      v-if="!hasConsented"
      class="fixed inset-x-0 bottom-0 z-50 p-4"
    >
      <div
        class="mx-auto max-w-4xl rounded-xl border border-gray-200 bg-white p-6 shadow-2xl"
      >
        <!-- Main consent message -->
        <div v-if="!showCustomise" class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div class="flex-1">
            <p class="text-sm leading-relaxed text-gray-700">
              We use cookies to ensure the best experience on our platform.
              Essential cookies are required for core functionality. Analytics
              and marketing cookies help us improve Raven and are optional.
              Learn more in our
              <RouterLink
                to="/privacy"
                class="font-medium text-indigo-600 underline underline-offset-2 transition-colors hover:text-indigo-800"
              >Privacy Policy</RouterLink>.
            </p>
          </div>
          <div class="flex flex-shrink-0 flex-wrap items-center gap-2">
            <button
              type="button"
              class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:outline-none"
              @click="showCustomise = true"
            >
              Customise
            </button>
            <button
              type="button"
              class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:outline-none"
              @click="handleReject"
            >
              Reject Non-Essential
            </button>
            <button
              type="button"
              class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700 focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:outline-none"
              @click="handleAcceptAll"
            >
              Accept All
            </button>
          </div>
        </div>

        <!-- Customise panel -->
        <div v-else>
          <h3 class="mb-4 text-base font-semibold text-gray-900">
            Cookie Preferences
          </h3>
          <div class="space-y-4">
            <!-- Essential - always on -->
            <div class="flex items-center justify-between">
              <div>
                <p class="text-sm font-medium text-gray-900">Essential Cookies</p>
                <p class="text-xs text-gray-500">
                  Required for authentication, session management, and core
                  functionality. Cannot be disabled.
                </p>
              </div>
              <div
                class="relative inline-flex h-6 w-11 cursor-not-allowed items-center rounded-full bg-indigo-600 opacity-60"
              >
                <span
                  class="inline-block h-4 w-4 translate-x-6 rounded-full bg-white transition-transform"
                />
              </div>
            </div>

            <!-- Analytics -->
            <div class="flex items-center justify-between">
              <div>
                <p class="text-sm font-medium text-gray-900">Analytics Cookies</p>
                <p class="text-xs text-gray-500">
                  Help us understand how you use Raven so we can improve
                  performance and features.
                </p>
              </div>
              <button
                type="button"
                class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:outline-none"
                :class="analyticsEnabled ? 'bg-indigo-600' : 'bg-gray-300'"
                @click="analyticsEnabled = !analyticsEnabled"
              >
                <span
                  class="inline-block h-4 w-4 rounded-full bg-white transition-transform"
                  :class="analyticsEnabled ? 'translate-x-6' : 'translate-x-1'"
                />
              </button>
            </div>

            <!-- Marketing -->
            <div class="flex items-center justify-between">
              <div>
                <p class="text-sm font-medium text-gray-900">Marketing Cookies</p>
                <p class="text-xs text-gray-500">
                  Used to deliver relevant content and measure the effectiveness
                  of campaigns.
                </p>
              </div>
              <button
                type="button"
                class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:outline-none"
                :class="marketingEnabled ? 'bg-indigo-600' : 'bg-gray-300'"
                @click="marketingEnabled = !marketingEnabled"
              >
                <span
                  class="inline-block h-4 w-4 rounded-full bg-white transition-transform"
                  :class="marketingEnabled ? 'translate-x-6' : 'translate-x-1'"
                />
              </button>
            </div>
          </div>

          <div class="mt-6 flex items-center justify-end gap-2">
            <button
              type="button"
              class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:outline-none"
              @click="showCustomise = false"
            >
              Back
            </button>
            <button
              type="button"
              class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700 focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:outline-none"
              @click="handleSavePreferences"
            >
              Save Preferences
            </button>
          </div>
        </div>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useCookieConsent } from '../composables/useCookieConsent'

const { hasConsented, acceptAll, rejectNonEssential, customise } =
  useCookieConsent()

const showCustomise = ref(false)
const analyticsEnabled = ref(true)
const marketingEnabled = ref(false)

function handleAcceptAll(): void {
  acceptAll()
}

function handleReject(): void {
  rejectNonEssential()
}

function handleSavePreferences(): void {
  customise({
    analytics: analyticsEnabled.value,
    marketing: marketingEnabled.value,
  })
}
</script>
