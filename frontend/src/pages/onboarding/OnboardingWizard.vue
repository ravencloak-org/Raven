<template>
  <div class="fixed inset-0 z-50 bg-slate-900 flex items-center justify-center p-4">
    <div class="w-full max-w-lg bg-slate-800 rounded-2xl shadow-2xl overflow-hidden">
      <!-- Progress dots -->
      <div class="flex justify-center gap-2 pt-6 pb-4">
        <button
          v-for="step in TOTAL_STEPS"
          :key="step"
          class="w-2.5 h-2.5 rounded-full transition-colors duration-200"
          :class="step === currentStep ? 'bg-indigo-500' : 'bg-slate-600'"
          :aria-label="`Step ${step}`"
          @click="currentStep = step"
        />
      </div>

      <!-- Step content -->
      <div class="px-8 pb-6">
        <!-- Step 1: Name your org -->
        <div v-if="currentStep === 1">
          <h2 class="text-2xl font-semibold text-white mb-2">Name your organisation</h2>
          <p class="text-slate-400 text-sm mb-6">
            Give your workspace a memorable name. You can change it later.
          </p>
          <label class="block text-sm text-slate-300 mb-1" for="org-name">Organisation name</label>
          <input
            id="org-name"
            v-model="orgName"
            type="text"
            placeholder="e.g. Acme Corp"
            class="w-full rounded-lg bg-slate-700 text-white placeholder-slate-500 border border-slate-600 px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
        </div>

        <!-- Step 2: Create first Knowledge Base -->
        <div v-else-if="currentStep === 2">
          <h2 class="text-2xl font-semibold text-white mb-2">Create your first Knowledge Base</h2>
          <p class="text-slate-400 text-sm mb-6">
            A Knowledge Base holds the documents your AI will learn from.
          </p>
          <label class="block text-sm text-slate-300 mb-1" for="kb-name">Knowledge Base name</label>
          <input
            id="kb-name"
            v-model="kbName"
            type="text"
            placeholder="e.g. Product Docs"
            class="w-full rounded-lg bg-slate-700 text-white placeholder-slate-500 border border-slate-600 px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
        </div>

        <!-- Step 3: Configure LLM provider -->
        <div v-else-if="currentStep === 3">
          <h2 class="text-2xl font-semibold text-white mb-2">Configure your LLM provider</h2>
          <p class="text-slate-400 text-sm mb-6">
            Raven uses your own API key (BYOK) — your data never leaves your control.
          </p>
          <label class="block text-sm text-slate-300 mb-1" for="llm-provider">Provider</label>
          <select
            id="llm-provider"
            v-model="llmProvider"
            class="w-full rounded-lg bg-slate-700 text-white border border-slate-600 px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 mb-4"
          >
            <option value="openai">OpenAI</option>
            <option value="anthropic">Anthropic</option>
            <option value="gemini">Gemini</option>
          </select>
          <label class="block text-sm text-slate-300 mb-1" for="api-key">API Key</label>
          <input
            id="api-key"
            v-model="apiKey"
            type="password"
            placeholder="sk-..."
            class="w-full rounded-lg bg-slate-700 text-white placeholder-slate-500 border border-slate-600 px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
            autocomplete="off"
          />
        </div>

        <!-- Step 4: Test chatbot -->
        <div v-else-if="currentStep === 4">
          <h2 class="text-2xl font-semibold text-white mb-2">Test your chatbot</h2>
          <p class="text-slate-400 text-sm mb-6">
            Send a quick message to make sure everything is connected.
          </p>
          <div class="bg-slate-700 rounded-xl p-4 min-h-24 mb-4 text-sm text-slate-300 space-y-2">
            <p v-if="chatResponse" class="text-indigo-300">{{ chatResponse }}</p>
            <p v-else class="text-slate-500 italic">Responses will appear here…</p>
          </div>
          <div class="flex gap-2">
            <input
              v-model="chatInput"
              type="text"
              placeholder="Type a message…"
              class="flex-1 rounded-lg bg-slate-700 text-white placeholder-slate-500 border border-slate-600 px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
              @keydown.enter="sendTestMessage"
            />
            <button
              class="bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-medium px-4 rounded-lg transition-colors"
              @click="sendTestMessage"
            >
              Send
            </button>
          </div>
        </div>

        <!-- Step 5: Done -->
        <div v-else-if="currentStep === 5" class="text-center py-4">
          <div class="text-5xl mb-4">🎉</div>
          <h2 class="text-2xl font-semibold text-white mb-2">You're all set!</h2>
          <p class="text-slate-400 text-sm mb-8">
            Raven is ready. Head to your dashboard to start uploading documents and chatting.
          </p>
          <button
            class="w-full bg-indigo-600 hover:bg-indigo-500 text-white font-semibold py-3 rounded-xl transition-colors"
            @click="finish"
          >
            Go to Dashboard
          </button>
        </div>
      </div>

      <!-- Navigation buttons -->
      <div v-if="currentStep < TOTAL_STEPS" class="flex items-center justify-between px-8 pb-8">
        <button
          class="text-slate-400 hover:text-white text-sm transition-colors"
          @click="skip"
        >
          Skip
        </button>
        <div class="flex gap-3">
          <button
            v-if="currentStep > 1"
            class="px-5 py-2 rounded-lg bg-slate-700 hover:bg-slate-600 text-white text-sm transition-colors"
            @click="currentStep--"
          >
            Back
          </button>
          <button
            class="px-5 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-medium transition-colors"
            @click="next"
          >
            {{ currentStep === TOTAL_STEPS - 1 ? 'Finish' : 'Next' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useOnboardingStore } from '../../stores/onboarding'

const TOTAL_STEPS = 5

const onboarding = useOnboardingStore()
const currentStep = ref<number>(1)

// Step 1
const orgName = ref('')
// Step 2
const kbName = ref('')
// Step 3
const llmProvider = ref<'openai' | 'anthropic' | 'gemini'>('openai')
const apiKey = ref('')
// Step 4
const chatInput = ref('')
const chatResponse = ref('')

function next(): void {
  if (currentStep.value < TOTAL_STEPS) {
    // TODO: Wire step submissions to backend APIs:
    // Step 1 → POST /api/v1/orgs (create org with orgName.value)
    // Step 2 → POST /api/v1/orgs/:orgId/knowledge-bases (create KB with kbName.value)
    // Step 3 → POST /api/v1/orgs/:orgId/llm-providers (save provider config)
    currentStep.value++
  }
}

function skip(): void {
  if (currentStep.value < TOTAL_STEPS) {
    currentStep.value++
  }
}

function finish(): void {
  // TODO: Validate all resources were created before marking complete
  onboarding.markComplete()
}

function sendTestMessage(): void {
  if (!chatInput.value.trim()) return
  // TODO: Wire to POST /api/v1/chat/:kb_id/completions once KB is created in step 2
  chatResponse.value = 'This is a preview — the chatbot will respond once setup is complete.'
  chatInput.value = ''
}
</script>
