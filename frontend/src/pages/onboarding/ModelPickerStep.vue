<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { usePrecheckStore } from '../../stores/precheck'

const emit = defineEmits<{
  (e: 'complete', model: string): void
  (e: 'skip'): void
}>()

const precheck = usePrecheckStore()

interface ModelOption {
  name: string
  sizeGb: number
  description: string
  /** Smallest tier this model is offered to. */
  minTier: 'floor' | 'low' | 'mid' | 'high'
}

const allModels: ModelOption[] = [
  { name: 'llama3.2:3b', sizeGb: 2.0, description: 'Smallest viable — fits 8 GB hosts', minTier: 'floor' },
  { name: 'llama3.1:8b', sizeGb: 4.7, description: 'Recommended for most users', minTier: 'low' },
  { name: 'llama3.1:13b', sizeGb: 7.4, description: 'Higher quality — needs 16 GB+', minTier: 'high' },
]

const tierToDefault: Record<string, string> = {
  floor: 'llama3.2:3b',
  low: 'llama3.1:8b',
  mid: 'llama3.1:8b',
  high: 'llama3.1:8b',
  unknown: 'llama3.2:3b',
}

const tierRank: Record<string, number> = { floor: 0, low: 1, mid: 2, high: 3 }

const visibleModels = computed(() => {
  const tier = precheck.tier
  if (tier === 'unknown' || tier === 'floor') {
    return allModels.filter((m) => m.minTier === 'floor')
  }
  return allModels.filter((m) => tierRank[m.minTier] <= tierRank[tier])
})

const selectedModel = ref<string>(tierToDefault[precheck.tier] ?? 'llama3.2:3b')

const pullState = ref<'idle' | 'pulling' | 'done' | 'error'>('idle')
const percent = ref(0)
const errorMessage = ref<string | null>(null)
let unlisten: (() => void) | null = null

interface PullProgress {
  model: string
  completed: number
  total: number
  percent: number
}

onMounted(async () => {
  try {
    const { listen } = await import('@tauri-apps/api/event')
    unlisten = await listen<PullProgress>('ollama:pull-progress', (e) => {
      if (e.payload.model === selectedModel.value) {
        percent.value = e.payload.percent
      }
    })
  } catch {
    // Not in a Tauri context — leave listener unbound (e.g. component tests).
  }
})

onBeforeUnmount(() => {
  unlisten?.()
})

async function pullSelected() {
  pullState.value = 'pulling'
  percent.value = 0
  errorMessage.value = null
  try {
    const { invoke } = await import('@tauri-apps/api/core')
    await invoke('ollama_pull_model', { model: selectedModel.value })
    pullState.value = 'done'
    emit('complete', selectedModel.value)
  } catch (e) {
    pullState.value = 'error'
    errorMessage.value = e instanceof Error ? e.message : String(e)
  }
}
</script>

<template>
  <div class="model-picker">
    <h2 class="text-2xl font-bold text-neutral-900 dark:text-white mb-2">
      Pick a local model
    </h2>
    <p class="text-neutral-500 text-sm mb-6">
      Raven Local will download an Ollama model to run chat on your machine.
    </p>

    <ul class="space-y-2 mb-6">
      <li
        v-for="m in visibleModels"
        :key="m.name"
        class="rounded-lg border p-3 cursor-pointer transition-colors"
        :class="
          m.name === selectedModel
            ? 'border-amber-500 bg-amber-50 dark:bg-amber-900/10'
            : 'border-neutral-300 dark:border-neutral-700'
        "
      >
        <label class="flex items-start gap-3 cursor-pointer">
          <input
            type="radio"
            :value="m.name"
            v-model="selectedModel"
            class="mt-1"
          />
          <div>
            <div class="font-semibold text-neutral-900 dark:text-white">
              {{ m.name }}
              <span class="text-xs text-neutral-500 font-normal">
                · {{ m.sizeGb.toFixed(1) }} GB
              </span>
            </div>
            <div class="text-sm text-neutral-500">{{ m.description }}</div>
          </div>
        </label>
      </li>
    </ul>

    <p
      data-test="selected-model"
      class="text-sm text-neutral-500 mb-3"
    >
      Selected: <code class="font-mono">{{ selectedModel }}</code>
    </p>

    <button
      data-test="pull-button"
      :disabled="pullState === 'pulling'"
      class="w-full bg-amber-500 hover:bg-amber-600 disabled:opacity-50 text-white font-semibold py-3 rounded-lg transition-colors mb-3"
      @click="pullSelected"
    >
      {{
        pullState === 'pulling'
          ? `Downloading… ${percent}%`
          : pullState === 'done'
            ? 'Downloaded'
            : 'Download model'
      }}
    </button>

    <progress
      v-if="pullState === 'pulling'"
      :value="percent"
      max="100"
      class="w-full h-2 mb-3"
    />

    <div
      v-if="errorMessage"
      class="text-red-500 text-sm mb-3"
    >
      {{ errorMessage }}
      <button class="underline ml-2" @click="pullSelected">Retry</button>
    </div>

    <button
      data-test="skip-model"
      class="text-sm text-neutral-500 underline"
      @click="emit('skip')"
    >
      Skip — I'll add a BYOK key instead
    </button>
  </div>
</template>
