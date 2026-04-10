<template>
  <div class="min-h-screen bg-gray-50 flex items-center justify-center p-4">
    <div class="w-full max-w-2xl">
      <!-- Header -->
      <div class="text-center mb-8">
        <h1 class="text-3xl font-bold text-gray-900">Welcome to Raven</h1>
        <p class="mt-2 text-gray-500">Let's get you set up in just a few steps.</p>
      </div>

      <!-- Step indicator -->
      <div class="flex items-center justify-between mb-8">
        <template v-for="(step, index) in steps" :key="step.id">
          <div class="flex flex-col items-center">
            <div
              class="w-10 h-10 rounded-full flex items-center justify-center text-sm font-semibold border-2 transition-colors"
              :class="stepCircleClass(index)"
            >
              <svg v-if="index < currentStep" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
              </svg>
              <span v-else>{{ index + 1 }}</span>
            </div>
            <span class="mt-1 text-xs font-medium" :class="index <= currentStep ? 'text-indigo-600' : 'text-gray-400'">
              {{ step.label }}
            </span>
          </div>
          <div
            v-if="index < steps.length - 1"
            class="flex-1 h-0.5 mx-2"
            :class="index < currentStep ? 'bg-indigo-600' : 'bg-gray-200'"
          />
        </template>
      </div>

      <!-- Step card -->
      <div class="bg-white shadow-sm rounded-xl p-8">
        <!-- Step 0: Welcome -->
        <div v-if="currentStep === 0">
          <div class="flex items-center justify-center w-16 h-16 bg-indigo-100 rounded-full mx-auto mb-6">
            <svg class="w-8 h-8 text-indigo-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.828 14.828a4 4 0 01-5.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </div>
          <h2 class="text-2xl font-semibold text-center text-gray-800 mb-3">Welcome aboard!</h2>
          <p class="text-gray-500 text-center leading-relaxed">
            Raven is your AI-powered knowledge management platform. This short wizard will help you set up
            your workspace, connect an AI provider, and get your first chatbot running.
          </p>
        </div>

        <!-- Step 1: Create Knowledge Base -->
        <div v-else-if="currentStep === 1">
          <div class="flex items-center justify-center w-16 h-16 bg-green-100 rounded-full mx-auto mb-6">
            <svg class="w-8 h-8 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
            </svg>
          </div>
          <h2 class="text-2xl font-semibold text-center text-gray-800 mb-3">Create your Knowledge Base</h2>
          <p class="text-gray-500 text-center leading-relaxed">
            A Knowledge Base is where you store documents, URLs, and other content that your AI chatbot
            will learn from. Your first workspace and knowledge base have been automatically created — head
            to the dashboard to upload your first document.
          </p>
        </div>

        <!-- Step 2: LLM Provider -->
        <div v-else-if="currentStep === 2">
          <div class="flex items-center justify-center w-16 h-16 bg-purple-100 rounded-full mx-auto mb-6">
            <svg class="w-8 h-8 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
            </svg>
          </div>
          <h2 class="text-2xl font-semibold text-center text-gray-800 mb-3">Connect an LLM Provider</h2>
          <p class="text-gray-500 text-center leading-relaxed">
            Raven supports OpenAI, Anthropic, Google, Mistral, and self-hosted models. Navigate to
            <strong>LLM Providers</strong> in the sidebar to add your API key and set a default model for
            your chatbots.
          </p>
        </div>

        <!-- Step 3: Test Chatbot -->
        <div v-else-if="currentStep === 3">
          <div class="flex items-center justify-center w-16 h-16 bg-yellow-100 rounded-full mx-auto mb-6">
            <svg class="w-8 h-8 text-yellow-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
            </svg>
          </div>
          <h2 class="text-2xl font-semibold text-center text-gray-800 mb-3">Test your Chatbot</h2>
          <p class="text-gray-500 text-center leading-relaxed">
            Once you've uploaded documents and connected an LLM provider, use the <strong>Sandbox</strong>
            to test your chatbot and fine-tune its behaviour before embedding it into your product.
          </p>
        </div>

        <!-- Step 4: Done -->
        <div v-else-if="currentStep === 4">
          <div class="flex items-center justify-center w-16 h-16 bg-indigo-100 rounded-full mx-auto mb-6">
            <svg class="w-8 h-8 text-indigo-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </div>
          <h2 class="text-2xl font-semibold text-center text-gray-800 mb-3">You're all set!</h2>
          <p class="text-gray-500 text-center leading-relaxed">
            Your Raven workspace is ready. Head to the dashboard to upload documents, configure your AI
            providers, and start building powerful chatbots.
          </p>
        </div>

        <!-- Navigation -->
        <div class="mt-8 flex items-center" :class="currentStep > 0 ? 'justify-between' : 'justify-end'">
          <button
            v-if="currentStep > 0"
            class="px-5 py-2.5 text-sm font-medium text-gray-600 bg-gray-100 hover:bg-gray-200 rounded-lg transition-colors"
            @click="prev"
          >
            Back
          </button>
          <button
            v-if="currentStep < steps.length - 1"
            class="px-5 py-2.5 text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 rounded-lg transition-colors"
            @click="next"
          >
            Continue
          </button>
          <button
            v-else
            class="px-5 py-2.5 text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 rounded-lg transition-colors"
            @click="finish"
          >
            Go to Dashboard
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useOnboarding } from '../../composables/useOnboarding'

const router = useRouter()
const { markCompleted } = useOnboarding()

const steps = [
  { id: 'welcome', label: 'Welcome' },
  { id: 'kb', label: 'Knowledge Base' },
  { id: 'llm', label: 'LLM Provider' },
  { id: 'chatbot', label: 'Test Chatbot' },
  { id: 'done', label: 'Done' },
]

const currentStep = ref(0)

function stepCircleClass(index: number): string {
  if (index < currentStep.value) {
    return 'bg-indigo-600 border-indigo-600 text-white'
  }
  if (index === currentStep.value) {
    return 'bg-white border-indigo-600 text-indigo-600'
  }
  return 'bg-white border-gray-300 text-gray-400'
}

function next(): void {
  if (currentStep.value < steps.length - 1) {
    currentStep.value++
  }
}

function prev(): void {
  if (currentStep.value > 0) {
    currentStep.value--
  }
}

function finish(): void {
  markCompleted()
  router.push({ name: 'dashboard' })
}
</script>
