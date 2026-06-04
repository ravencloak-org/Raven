<script setup lang="ts">
import { nextTick, ref } from 'vue'
import {
  PROVIDER_MODELS,
  type LlmProvider,
  type ProviderType,
} from '../../../api/llm-providers'
import { PROVIDER_TYPE_LABEL } from '../providerHelp'

const props = defineProps<{
  provider: LlmProvider
  isHealthy: boolean
  healthFailureReason: string
  /** Provider id whose "Make default" request is currently in-flight. */
  settingDefaultId: string | null
  /** Provider id with the "Last test failed" warning. */
  testFailedForId: boolean
  /** Provider id flashing the "Saved ✓" tick. */
  showSavedTick: boolean
}>()

const emit = defineEmits<{
  (e: 'make-default', id: string): void
  (e: 'edit-credentials', provider: LlmProvider): void
  (e: 'delete', id: string, name: string): void
  (e: 'rename', provider: LlmProvider, nextName: string): void
  (e: 'change-model', provider: LlmProvider, nextModel: string): void
}>()

function modelsForType(type: ProviderType) {
  return PROVIDER_MODELS[type] ?? []
}

function providerModel(provider: LlmProvider): string {
  const m = provider.config?.model
  return typeof m === 'string' ? m : ''
}

// Inline display-name editor (#744): click-to-edit input. Enter / blur
// saves through the rename emit; Esc cancels. The page owns the
// optimistic mutation + rollback — the card only manages the local
// editor state.
const editing = ref(false)
const draft = ref('')
const nameInputRef = ref<HTMLInputElement | null>(null)

async function startEditName() {
  editing.value = true
  draft.value = props.provider.display_name
  await nextTick()
  nameInputRef.value?.focus()
  nameInputRef.value?.select()
}

function cancelEditName() {
  editing.value = false
  draft.value = ''
}

function commitEditName() {
  const target = draft.value.trim()
  // No-op if unchanged or empty — just close the inline editor.
  if (!target || target === props.provider.display_name) {
    cancelEditName()
    return
  }
  editing.value = false
  draft.value = ''
  emit('rename', props.provider, target)
}
</script>

<template>
  <article
    class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm"
    data-testid="provider-card"
  >
    <!-- Header row: icon + (editable name | status | default pill) -->
    <div class="flex items-start gap-3">
      <!-- Provider-type icon (chip glyph for visual consistency with the M13 menu). -->
      <span
        aria-hidden="true"
        class="mt-0.5 inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-indigo-50 text-indigo-600"
      >
        <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <rect x="4" y="4" width="16" height="16" rx="2" />
          <rect x="9" y="9" width="6" height="6" />
          <path d="M9 1v3M15 1v3M9 20v3M15 20v3M1 9h3M1 15h3M20 9h3M20 15h3" />
        </svg>
      </span>

      <div class="min-w-0 flex-1">
        <!-- Editable display_name + status badge + default pill -->
        <div class="flex flex-wrap items-center gap-2">
          <input
            v-if="editing"
            ref="nameInputRef"
            v-model="draft"
            type="text"
            data-testid="display-name-input"
            class="min-w-0 flex-1 rounded border border-indigo-300 px-2 py-1 text-base font-semibold text-gray-900 shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            @keydown.enter.prevent="commitEditName"
            @keydown.esc.prevent="cancelEditName"
            @blur="commitEditName"
          />
          <button
            v-else
            type="button"
            data-testid="display-name-label"
            class="truncate rounded px-1 py-0.5 text-left text-base font-semibold text-gray-900 hover:bg-gray-50 focus:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-indigo-300"
            :title="`Click to rename — ${provider.display_name}`"
            @click="startEditName"
          >
            {{ provider.display_name }}
          </button>
          <span
            :class="[
              'shrink-0 rounded-full px-2 py-0.5 text-xs font-medium',
              provider.status === 'active'
                ? 'bg-green-100 text-green-700'
                : 'bg-gray-100 text-gray-600',
            ]"
          >
            {{ provider.status }}
          </span>
          <span
            v-if="provider.is_default"
            data-testid="default-pill"
            class="shrink-0 rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 transition duration-200 ease-out"
          >
            Default
          </span>
          <span
            v-if="provider.is_default && !isHealthy"
            data-testid="default-connection-error"
            class="shrink-0 rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700"
            :title="healthFailureReason"
          >
            Connection error
          </span>
          <span
            v-if="showSavedTick"
            data-testid="saved-tick"
            class="shrink-0 text-xs font-medium text-green-600"
            role="status"
            aria-live="polite"
          >
            Saved ✓
          </span>
        </div>

        <!-- Type + base_url meta -->
        <p class="mt-1 text-sm text-gray-500">
          {{ PROVIDER_TYPE_LABEL[provider.provider] }}
          <span v-if="provider.base_url" class="break-all"> &middot; {{ provider.base_url }}</span>
        </p>

        <!-- Inline model selector + api-key hint -->
        <div class="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
          <label class="flex items-center gap-2 text-xs text-gray-600">
            <span class="shrink-0">Model</span>
            <select
              data-testid="model-select"
              class="rounded border-gray-300 px-2 py-1 text-xs shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              :value="providerModel(provider)"
              @change="emit('change-model', provider, ($event.target as HTMLSelectElement).value)"
            >
              <option v-if="!providerModel(provider)" value="">Select model…</option>
              <option
                v-for="m in modelsForType(provider.provider)"
                :key="m.value"
                :value="m.value"
              >
                {{ m.label }}
              </option>
            </select>
          </label>
          <span class="text-xs text-gray-400">
            API key {{ provider.api_key_hint ? 'configured' : 'not set' }}
          </span>
        </div>

        <p
          v-if="!provider.is_default && testFailedForId"
          class="mt-2 rounded border border-amber-300 bg-amber-50 px-2 py-1 text-xs text-amber-800"
        >
          Last connection test failed — switching default may break chat.
        </p>
      </div>
    </div>

    <!-- Action row: wraps under content on narrow viewports. -->
    <div class="mt-3 flex flex-wrap justify-end gap-2 border-t border-gray-100 pt-3">
      <button
        v-if="!provider.is_default"
        data-testid="make-default-btn"
        :disabled="settingDefaultId === provider.id"
        class="rounded border border-amber-300 px-3 py-1 text-xs font-medium text-amber-800 hover:bg-amber-50 disabled:opacity-50"
        @click="emit('make-default', provider.id)"
      >
        {{ settingDefaultId === provider.id ? 'Setting...' : 'Make default' }}
      </button>
      <button
        data-testid="edit-credentials-btn"
        class="rounded border border-gray-300 px-3 py-1 text-xs text-gray-700 hover:bg-gray-50"
        @click="emit('edit-credentials', provider)"
      >
        Edit credentials
      </button>
      <button
        data-testid="delete-btn"
        class="rounded border border-red-300 px-3 py-1 text-xs text-red-700 hover:bg-red-50"
        @click="emit('delete', provider.id, provider.display_name)"
      >
        Delete
      </button>
    </div>
  </article>
</template>
