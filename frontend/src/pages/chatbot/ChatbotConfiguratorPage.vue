<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useChatbotConfigStore } from '../../stores/chatbot-config'

const store = useChatbotConfigStore()

// --- Local form state bound via v-model ---
const themeColor = ref('#4f46e5')
const avatarUrl = ref('')
const welcomeText = ref('')
const suggestedQuestions = ref<string[]>([])
const position = ref<'bottom-right' | 'bottom-left'>('bottom-right')
const widgetTitle = ref('')

// --- Suggested question input ---
const newQuestion = ref('')

// --- Save feedback ---
const saveSuccess = ref(false)

// --- Embed code ---
const WIDGET_SCRIPT_URL = import.meta.env.VITE_WIDGET_URL ?? 'https://cdn.raven.example/widget.js'
const copiedEmbed = ref(false)

const embedCode = computed(() => {
  return (
    `<script src="${WIDGET_SCRIPT_URL}"` +
    `\n  data-api-key="YOUR_API_KEY"` +
    `\n  data-theme-color="${themeColor.value}"` +
    `\n  data-position="${position.value}"` +
    `\n  data-widget-title="${widgetTitle.value}"` +
    `\n  async><` +
    '/script>'
  )
})

// Sync local form state when store config loads
watch(
  () => store.config,
  (cfg) => {
    if (!cfg) return
    themeColor.value = cfg.theme_color
    avatarUrl.value = cfg.avatar_url
    welcomeText.value = cfg.welcome_text
    suggestedQuestions.value = [...cfg.suggested_questions]
    position.value = cfg.position
    widgetTitle.value = cfg.widget_title
  },
  { immediate: true },
)

onMounted(() => store.fetchConfig())

function addQuestion() {
  const q = newQuestion.value.trim()
  if (!q) return
  if (suggestedQuestions.value.includes(q)) return
  suggestedQuestions.value.push(q)
  newQuestion.value = ''
}

function removeQuestion(index: number) {
  suggestedQuestions.value.splice(index, 1)
}

async function handleSave() {
  saveSuccess.value = false
  await store.saveConfig({
    theme_color: themeColor.value,
    avatar_url: avatarUrl.value,
    welcome_text: welcomeText.value,
    suggested_questions: [...suggestedQuestions.value],
    position: position.value,
    widget_title: widgetTitle.value,
  })
  if (!store.error) {
    saveSuccess.value = true
    setTimeout(() => (saveSuccess.value = false), 3000)
  }
}

async function copyEmbedCode() {
  await navigator.clipboard.writeText(embedCode.value)
  copiedEmbed.value = true
  setTimeout(() => (copiedEmbed.value = false), 2000)
}
</script>

<template>
  <div class="p-6 max-w-7xl">
    <!-- Header -->
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900">Chatbot Configurator</h1>
      <p class="mt-1 text-sm text-gray-500">
        Customize the appearance and behavior of your Raven chat widget.
      </p>
    </div>

    <!-- Loading -->
    <div v-if="store.loading" class="text-gray-500">Loading configuration...</div>

    <!-- Error banner -->
    <div
      v-if="store.error"
      class="mb-6 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700"
    >
      {{ store.error }}
    </div>

    <!-- Save success banner -->
    <div
      v-if="saveSuccess"
      class="mb-6 rounded-lg border border-green-200 bg-green-50 p-4 text-sm text-green-700"
    >
      Configuration saved successfully.
    </div>

    <!-- Main two-column layout -->
    <div v-if="!store.loading" class="grid grid-cols-1 gap-8 lg:grid-cols-2">
      <!-- Left panel: Configuration form -->
      <div class="space-y-6">
        <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
          <h2 class="text-lg font-semibold text-gray-900 mb-4">Widget Settings</h2>

          <form class="space-y-5" @submit.prevent="handleSave">
            <!-- Widget Title -->
            <div>
              <label for="widget-title" class="block text-sm font-medium text-gray-700">
                Widget Title
              </label>
              <input
                id="widget-title"
                v-model="widgetTitle"
                type="text"
                placeholder="e.g. Raven Chat"
                class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            </div>

            <!-- Theme Color -->
            <div>
              <label for="theme-color" class="block text-sm font-medium text-gray-700">
                Theme Color
              </label>
              <div class="mt-1 flex items-center gap-3">
                <input
                  id="theme-color"
                  v-model="themeColor"
                  type="color"
                  class="h-10 w-14 cursor-pointer rounded-lg border border-gray-300 p-0.5"
                />
                <input
                  v-model="themeColor"
                  type="text"
                  pattern="^#[0-9a-fA-F]{6}$"
                  placeholder="#4f46e5"
                  class="block w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm font-mono focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
              </div>
            </div>

            <!-- Avatar URL -->
            <div>
              <label for="avatar-url" class="block text-sm font-medium text-gray-700">
                Avatar URL
              </label>
              <input
                id="avatar-url"
                v-model="avatarUrl"
                type="url"
                placeholder="https://example.com/avatar.png"
                class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
              <p class="mt-1 text-xs text-gray-400">Image displayed as the bot avatar in the chat.</p>
            </div>

            <!-- Welcome Text -->
            <div>
              <label for="welcome-text" class="block text-sm font-medium text-gray-700">
                Welcome Message
              </label>
              <textarea
                id="welcome-text"
                v-model="welcomeText"
                rows="3"
                placeholder="Hi there! How can I help you today?"
                class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            </div>

            <!-- Position -->
            <div>
              <label for="position" class="block text-sm font-medium text-gray-700">
                Widget Position
              </label>
              <select
                id="position"
                v-model="position"
                class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              >
                <option value="bottom-right">Bottom Right</option>
                <option value="bottom-left">Bottom Left</option>
              </select>
            </div>

            <!-- Suggested Questions -->
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-2">
                Suggested Questions
              </label>
              <div class="space-y-2">
                <div
                  v-for="(q, idx) in suggestedQuestions"
                  :key="idx"
                  class="flex items-center gap-2"
                >
                  <span
                    class="flex-1 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700"
                  >
                    {{ q }}
                  </span>
                  <button
                    type="button"
                    class="rounded-md p-1 text-gray-400 hover:text-red-600 hover:bg-red-50"
                    aria-label="Remove question"
                    @click="removeQuestion(idx)"
                  >
                    <svg class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                      <path
                        fill-rule="evenodd"
                        d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                        clip-rule="evenodd"
                      />
                    </svg>
                  </button>
                </div>
              </div>
              <div class="mt-2 flex items-center gap-2">
                <input
                  v-model="newQuestion"
                  type="text"
                  placeholder="Add a suggested question..."
                  class="block w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  @keydown.enter.prevent="addQuestion"
                />
                <button
                  type="button"
                  class="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 whitespace-nowrap"
                  @click="addQuestion"
                >
                  Add
                </button>
              </div>
            </div>

            <!-- Save -->
            <div class="flex items-center gap-3 pt-2">
              <button
                type="submit"
                :disabled="store.saving"
                class="rounded-lg bg-indigo-600 px-5 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {{ store.saving ? 'Saving...' : 'Save Configuration' }}
              </button>
            </div>
          </form>
        </div>
      </div>

      <!-- Right panel: Live preview -->
      <div class="space-y-6">
        <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
          <h2 class="text-lg font-semibold text-gray-900 mb-4">Live Preview</h2>

          <!-- Preview container with simulated page background -->
          <div class="relative rounded-lg border border-gray-200 bg-gray-100 p-4 min-h-[480px]">
            <!-- Simulated chat widget -->
            <div
              class="absolute bottom-4 w-80"
              :class="position === 'bottom-right' ? 'right-4' : 'left-4'"
            >
              <!-- Chat window -->
              <div class="rounded-xl shadow-2xl overflow-hidden border border-gray-200">
                <!-- Header -->
                <div
                  class="flex items-center gap-3 px-4 py-3 text-white"
                  :style="{ backgroundColor: themeColor }"
                >
                  <div class="h-8 w-8 rounded-full bg-white/20 overflow-hidden flex items-center justify-center shrink-0">
                    <img
                      v-if="avatarUrl"
                      :src="avatarUrl"
                      alt="Bot avatar"
                      class="h-full w-full object-cover"
                      @error="($event.target as HTMLImageElement).style.display = 'none'"
                    />
                    <svg
                      v-else
                      class="h-5 w-5 text-white/80"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <path
                        d="M12 2a5 5 0 015 5v1a5 5 0 01-10 0V7a5 5 0 015-5zM20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2"
                      />
                    </svg>
                  </div>
                  <div class="flex-1 min-w-0">
                    <p class="text-sm font-semibold truncate">{{ widgetTitle || 'Chat' }}</p>
                    <p class="text-xs opacity-80">Online</p>
                  </div>
                  <button class="text-white/70 hover:text-white">
                    <svg class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                      <path
                        fill-rule="evenodd"
                        d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                        clip-rule="evenodd"
                      />
                    </svg>
                  </button>
                </div>

                <!-- Chat body -->
                <div class="bg-white px-4 py-4 space-y-3">
                  <!-- Bot welcome message -->
                  <div class="flex items-start gap-2">
                    <div
                      class="h-6 w-6 rounded-full shrink-0 flex items-center justify-center"
                      :style="{ backgroundColor: themeColor + '20' }"
                    >
                      <svg
                        class="h-3.5 w-3.5"
                        :style="{ color: themeColor }"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                      >
                        <path
                          d="M12 2a5 5 0 015 5v1a5 5 0 01-10 0V7a5 5 0 015-5zM20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2"
                        />
                      </svg>
                    </div>
                    <div class="rounded-lg rounded-tl-none bg-gray-100 px-3 py-2 text-sm text-gray-700 max-w-[85%]">
                      {{ welcomeText || 'Hello! How can I help you?' }}
                    </div>
                  </div>

                  <!-- Suggested questions -->
                  <div v-if="suggestedQuestions.length > 0" class="flex flex-wrap gap-1.5 pl-8">
                    <button
                      v-for="(q, idx) in suggestedQuestions"
                      :key="idx"
                      class="rounded-full border px-3 py-1 text-xs font-medium transition-colors"
                      :style="{
                        borderColor: themeColor + '40',
                        color: themeColor,
                      }"
                    >
                      {{ q }}
                    </button>
                  </div>
                </div>

                <!-- Input bar -->
                <div class="border-t border-gray-200 bg-white px-4 py-3">
                  <div class="flex items-center gap-2">
                    <div class="flex-1 rounded-full border border-gray-300 bg-gray-50 px-4 py-2 text-sm text-gray-400">
                      Type a message...
                    </div>
                    <button
                      class="rounded-full p-2 text-white"
                      :style="{ backgroundColor: themeColor }"
                    >
                      <svg class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
                        <path
                          d="M10.894 2.553a1 1 0 00-1.788 0l-7 14a1 1 0 001.169 1.409l5-1.429A1 1 0 009 15.571V11a1 1 0 112 0v4.571a1 1 0 00.725.962l5 1.428a1 1 0 001.17-1.408l-7-14z"
                        />
                      </svg>
                    </button>
                  </div>
                </div>
              </div>

              <!-- Floating trigger button (shown below the widget for reference) -->
              <div
                class="mt-3 flex justify-end"
                :class="position === 'bottom-left' ? 'justify-start' : 'justify-end'"
              >
                <div
                  class="h-12 w-12 rounded-full shadow-lg flex items-center justify-center text-white"
                  :style="{ backgroundColor: themeColor }"
                >
                  <svg class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z" />
                  </svg>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Embed code section -->
        <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
          <h2 class="text-lg font-semibold text-gray-900 mb-2">Embed Code</h2>
          <p class="text-sm text-gray-500 mb-3">
            Copy this snippet and paste it into your website's HTML, just before the closing
            <code class="rounded bg-gray-100 px-1.5 py-0.5 text-xs font-mono">&lt;/body&gt;</code> tag.
          </p>
          <div class="relative">
            <pre class="rounded-lg bg-gray-900 p-4 text-sm font-mono text-green-400 overflow-x-auto select-all whitespace-pre-wrap break-all">{{ embedCode }}</pre>
            <button
              class="absolute top-2 right-2 rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
              :class="
                copiedEmbed
                  ? 'bg-green-600 text-white'
                  : 'bg-gray-700 text-gray-300 hover:bg-gray-600 hover:text-white'
              "
              @click="copyEmbedCode"
            >
              {{ copiedEmbed ? 'Copied!' : 'Copy' }}
            </button>
          </div>
          <p class="mt-2 text-xs text-gray-400">
            Replace <code class="font-mono">YOUR_API_KEY</code> with an actual API key from the
            <a href="/api-keys" class="text-indigo-600 hover:text-indigo-800 underline">API Keys</a> page.
          </p>
        </div>
      </div>
    </div>
  </div>
</template>
