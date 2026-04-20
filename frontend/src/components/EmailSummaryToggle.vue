<script setup lang="ts">
/**
 * Toggle for post-session email summaries (M9 / #257).
 *
 * Two variants:
 *   - mode="user"      — current user opts in for a given workspace.
 *   - mode="workspace" — workspace admin flips the master switch.
 *
 * The component does not fetch the initial value itself — callers pass it in
 * via `modelValue` so the parent (user-settings page or workspace-settings
 * page) can load it alongside other prefs in a single round-trip.
 */

import { ref } from 'vue'
import { usePostHog } from '../plugins/posthog'
import {
  setUserEmailSummaries,
  setWorkspaceEmailSummaries,
} from '../api/notification-preferences'

const props = defineProps<{
  mode: 'user' | 'workspace'
  orgId?: string
  workspaceId: string
  modelValue: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
  (e: 'error', msg: string): void
}>()

const saving = ref(false)
const posthog = usePostHog()

async function onToggle(e: Event) {
  const next = (e.target as HTMLInputElement).checked
  saving.value = true
  try {
    if (props.mode === 'user') {
      await setUserEmailSummaries(props.workspaceId, next)
    } else {
      if (!props.orgId) throw new Error('orgId required for workspace mode')
      await setWorkspaceEmailSummaries(props.orgId, props.workspaceId, next)
    }
    emit('update:modelValue', next)
    posthog?.capture('summary_email_pref_changed', {
      mode: props.mode,
      enabled: next,
      workspace_id: props.workspaceId,
    })
  } catch (err) {
    emit('error', (err as Error).message)
    // Revert the checkbox — two-way binding will re-assert the previous value.
    ;(e.target as HTMLInputElement).checked = props.modelValue
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <label class="email-summary-toggle">
    <input
      type="checkbox"
      :checked="modelValue"
      :disabled="saving"
      @change="onToggle"
    />
    <span>
      <strong>Email me a summary after each session</strong>
      <small v-if="mode === 'workspace'">
        Workspace-wide master switch — when off, nobody receives summaries.
      </small>
      <small v-else>
        Receive a concise recap in your inbox after every chat or voice call.
      </small>
    </span>
  </label>
</template>

<style scoped>
.email-summary-toggle {
  display: flex;
  gap: 0.75rem;
  align-items: flex-start;
  padding: 0.75rem 1rem;
  border: 1px solid var(--color-border, #e4e7eb);
  border-radius: 6px;
  cursor: pointer;
  background: var(--color-surface, #fff);
}
.email-summary-toggle input {
  margin-top: 0.25rem;
}
.email-summary-toggle small {
  display: block;
  margin-top: 0.25rem;
  color: var(--color-text-muted, #616e7c);
  font-size: 0.85rem;
}
</style>
