<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  listConversations,
  getConversation,
  type ConversationSession,
  type ConversationSessionSummary,
} from '../../api/conversations'

const props = defineProps<{
  orgId: string
  kbId: string
}>()

const loading = ref(false)
const error = ref<string | null>(null)
const sessions = ref<ConversationSessionSummary[]>([])
const selected = ref<ConversationSession | null>(null)
const loadingTranscript = ref(false)

const returningGreeting = computed(() => {
  if (!sessions.value.length) return null
  const latest = sessions.value[0]
  const topic = latest.summary ?? `your last ${latest.channel} conversation`
  return `Welcome back! Last time you asked about ${topic}.`
})

const channelIcon = (c: string): string => {
  switch (c) {
    case 'chat': return 'message-square'
    case 'voice': return 'phone'
    case 'webrtc': return 'video'
    default: return 'circle'
  }
}

const fmtDate = (iso: string): string => {
  try {
    return new Date(iso).toLocaleString(undefined, {
      month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
    })
  } catch {
    return iso
  }
}

async function load() {
  loading.value = true
  error.value = null
  try {
    const resp = await listConversations(props.orgId, props.kbId, 5, 0)
    sessions.value = resp.sessions
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

async function openSession(id: string) {
  loadingTranscript.value = true
  try {
    selected.value = await getConversation(props.orgId, props.kbId, id)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loadingTranscript.value = false
  }
}

function closeTranscript() {
  selected.value = null
}

onMounted(load)
watch(() => [props.orgId, props.kbId], () => {
  sessions.value = []
  selected.value = null
  void load()
})
</script>

<template>
  <section class="recent-conversations" aria-labelledby="recent-conv-heading">
    <header class="recent-conversations__header">
      <h3 id="recent-conv-heading">Recent conversations</h3>
      <button
        type="button"
        class="recent-conversations__refresh"
        :disabled="loading"
        @click="load"
      >
        {{ loading ? 'Loading…' : 'Refresh' }}
      </button>
    </header>

    <p v-if="returningGreeting" class="recent-conversations__greeting">
      {{ returningGreeting }}
    </p>

    <p v-if="error" class="recent-conversations__error" role="alert">
      {{ error }}
    </p>

    <p v-if="!loading && !error && sessions.length === 0" class="recent-conversations__empty">
      No past sessions yet. Start a chat or voice call and it will appear here.
    </p>

    <ul v-if="sessions.length" class="recent-conversations__list">
      <li
        v-for="s in sessions"
        :key="s.id"
        class="recent-conversations__item"
      >
        <button
          type="button"
          class="recent-conversations__item-btn"
          @click="openSession(s.id)"
        >
          <span :data-icon="channelIcon(s.channel)" class="recent-conversations__icon">
            {{ s.channel }}
          </span>
          <span class="recent-conversations__meta">
            <span class="recent-conversations__date">{{ fmtDate(s.started_at) }}</span>
            <span class="recent-conversations__count">{{ s.message_count }} messages</span>
          </span>
          <span v-if="s.summary" class="recent-conversations__summary">{{ s.summary }}</span>
        </button>
      </li>
    </ul>

    <div
      v-if="selected"
      class="recent-conversations__transcript"
      role="dialog"
      aria-modal="true"
    >
      <header class="recent-conversations__transcript-header">
        <h4>Transcript</h4>
        <button type="button" @click="closeTranscript">Close</button>
      </header>
      <p v-if="loadingTranscript">Loading transcript…</p>
      <ol v-else class="recent-conversations__turns">
        <li
          v-for="(t, idx) in selected.messages"
          :key="idx"
          :data-role="t.role"
          class="recent-conversations__turn"
        >
          <span class="recent-conversations__turn-role">{{ t.role }}</span>
          <p class="recent-conversations__turn-content">{{ t.content }}</p>
          <time class="recent-conversations__turn-ts">{{ fmtDate(t.ts) }}</time>
        </li>
      </ol>
    </div>
  </section>
</template>

<style scoped>
.recent-conversations {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1rem;
  border-radius: 12px;
  background: var(--surface-1, #fff);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
}
.recent-conversations__header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}
.recent-conversations__header h3 {
  margin: 0;
  font-size: 1rem;
}
.recent-conversations__refresh {
  background: transparent;
  border: none;
  color: var(--color-primary, #2563eb);
  cursor: pointer;
  font-size: 0.85rem;
}
.recent-conversations__greeting {
  margin: 0;
  padding: 0.5rem 0.75rem;
  border-radius: 8px;
  background: var(--surface-2, #f4f6fb);
  font-size: 0.9rem;
}
.recent-conversations__error {
  color: #b91c1c;
  font-size: 0.9rem;
}
.recent-conversations__empty {
  color: var(--text-muted, #6b7280);
  font-size: 0.9rem;
}
.recent-conversations__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.recent-conversations__item-btn {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.25rem 0.75rem;
  width: 100%;
  padding: 0.6rem 0.75rem;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 10px;
  background: transparent;
  text-align: left;
  cursor: pointer;
}
.recent-conversations__item-btn:hover,
.recent-conversations__item-btn:focus-visible {
  border-color: var(--color-primary, #2563eb);
  outline: none;
}
.recent-conversations__icon {
  grid-row: span 2;
  align-self: start;
  padding: 0.15rem 0.45rem;
  border-radius: 999px;
  background: var(--surface-2, #f4f6fb);
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.recent-conversations__meta {
  display: flex;
  justify-content: space-between;
  font-size: 0.85rem;
  color: var(--text-muted, #6b7280);
}
.recent-conversations__summary {
  grid-column: 2;
  font-size: 0.9rem;
  color: var(--text-default, #111827);
}
.recent-conversations__transcript {
  margin-top: 0.75rem;
  padding: 0.75rem;
  border-radius: 10px;
  border: 1px solid var(--border, #e5e7eb);
}
.recent-conversations__transcript-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.recent-conversations__turns {
  list-style: none;
  padding: 0;
  margin: 0.5rem 0 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.recent-conversations__turn {
  padding: 0.5rem 0.6rem;
  border-radius: 8px;
  background: var(--surface-2, #f4f6fb);
}
.recent-conversations__turn[data-role='assistant'] {
  background: var(--surface-3, #eef2ff);
}
.recent-conversations__turn-role {
  display: inline-block;
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted, #6b7280);
}
.recent-conversations__turn-content {
  margin: 0.15rem 0;
  font-size: 0.95rem;
  white-space: pre-wrap;
}
.recent-conversations__turn-ts {
  font-size: 0.75rem;
  color: var(--text-muted, #6b7280);
}
</style>
