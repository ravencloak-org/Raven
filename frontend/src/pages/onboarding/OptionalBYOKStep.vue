<script setup lang="ts">
import { ref } from 'vue'

const emit = defineEmits<{
  (e: 'skip'): void
  (e: 'saved', payload: { provider: string }): void
}>()

const apiUrl = import.meta.env.VITE_API_BASE_URL ?? ''

const provider = ref('openai')
const apiKey = ref('')
const errorMessage = ref<string | null>(null)
const saving = ref(false)

async function submit() {
  if (!apiKey.value) return
  saving.value = true
  errorMessage.value = null
  try {
    const res = await fetch(`${apiUrl}/api/v1/llm-providers`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        provider: provider.value,
        api_key: apiKey.value,
        display_name: provider.value,
      }),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error(data.message || data.error || `Save failed (${res.status})`)
    }
    emit('saved', { provider: provider.value })
  } catch (e) {
    errorMessage.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="byok-step">
    <h2 class="text-2xl font-bold text-neutral-900 dark:text-white mb-2">
      Add a cloud LLM key (optional)
    </h2>
    <p class="text-neutral-500 text-sm mb-6">
      Skip if you only want local Ollama. You can add cloud keys anytime in Settings.
    </p>

    <form class="space-y-4 mb-4" @submit.prevent="submit">
      <label class="block">
        <span class="text-sm text-neutral-700 dark:text-neutral-300">Provider</span>
        <select
          v-model="provider"
          data-test="provider"
          class="w-full mt-1 rounded-lg border border-neutral-300 dark:border-neutral-700 bg-white dark:bg-neutral-900 text-neutral-900 dark:text-white px-3 py-2"
        >
          <option value="openai">OpenAI</option>
          <option value="anthropic">Anthropic</option>
          <option value="cohere">Cohere</option>
        </select>
      </label>

      <label class="block">
        <span class="text-sm text-neutral-700 dark:text-neutral-300">API key</span>
        <input
          v-model="apiKey"
          type="password"
          data-test="apikey"
          autocomplete="off"
          class="w-full mt-1 rounded-lg border border-neutral-300 dark:border-neutral-700 bg-white dark:bg-neutral-900 text-neutral-900 dark:text-white px-3 py-2"
        />
      </label>

      <button
        type="submit"
        :disabled="!apiKey || saving"
        class="w-full bg-amber-500 hover:bg-amber-600 disabled:opacity-50 text-white font-semibold py-3 rounded-lg transition-colors"
      >
        {{ saving ? 'Saving…' : 'Save' }}
      </button>
    </form>

    <button
      data-test="skip"
      class="text-sm text-neutral-500 underline"
      @click="emit('skip')"
    >
      Skip
    </button>

    <div v-if="errorMessage" class="text-red-500 text-sm mt-3">
      {{ errorMessage }}
    </div>
  </div>
</template>
