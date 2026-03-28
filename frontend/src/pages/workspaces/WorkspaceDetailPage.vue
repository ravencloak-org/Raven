<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useWorkspacesStore } from '../../stores/workspaces'

const route = useRoute()
const router = useRouter()
const store = useWorkspacesStore()

const orgId = route.params.orgId as string
const wsId = route.params.wsId as string

const memberUserId = ref('')
const memberRole = ref('member')
const addingMember = ref(false)

onMounted(() => store.fetchWorkspace(orgId, wsId))

async function handleAddMember() {
  const userId = memberUserId.value.trim()
  if (!userId) return
  addingMember.value = true
  try {
    await store.addWorkspaceMember(orgId, wsId, userId, memberRole.value)
    memberUserId.value = ''
    memberRole.value = 'member'
  } catch {
    /* error surfaced via store.error */
  } finally {
    addingMember.value = false
  }
}

async function handleDelete() {
  try {
    await store.remove(orgId, wsId)
    router.push(`/orgs/${orgId}/workspaces`)
  } catch {
    /* error surfaced via store.error */
  }
}
</script>

<template>
  <div class="p-6 max-w-4xl mx-auto">
    <!-- Loading state -->
    <div v-if="store.loading" class="text-gray-500">Loading...</div>

    <!-- Error state -->
    <div v-else-if="store.error" class="text-red-600">{{ store.error }}</div>

    <!-- Workspace detail -->
    <div v-else-if="store.currentWorkspace">
      <div class="flex items-center justify-between mb-6">
        <div>
          <h1 class="text-2xl font-bold">{{ store.currentWorkspace.name }}</h1>
          <p class="text-sm text-gray-500">{{ store.currentWorkspace.slug }}</p>
        </div>
        <button
          class="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700"
          @click="handleDelete"
        >
          Delete Workspace
        </button>
      </div>

      <dl class="grid grid-cols-2 gap-4 mb-8 text-sm">
        <div>
          <dt class="text-gray-500">Created</dt>
          <dd>{{ new Date(store.currentWorkspace.created_at).toLocaleString() }}</dd>
        </div>
        <div>
          <dt class="text-gray-500">Updated</dt>
          <dd>{{ new Date(store.currentWorkspace.updated_at).toLocaleString() }}</dd>
        </div>
      </dl>

      <!-- Members section -->
      <section>
        <h2 class="text-lg font-semibold mb-4">Members</h2>

        <!-- Add member form -->
        <form class="flex gap-3 mb-6" @submit.prevent="handleAddMember">
          <input
            v-model="memberUserId"
            type="text"
            placeholder="User ID"
            class="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
          <select
            v-model="memberRole"
            class="rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          >
            <option value="member">Member</option>
            <option value="admin">Admin</option>
          </select>
          <button
            type="submit"
            :disabled="addingMember || !memberUserId.trim()"
            class="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {{ addingMember ? 'Adding...' : 'Add Member' }}
          </button>
        </form>

        <!-- Member list -->
        <div v-if="store.members.length === 0" class="text-sm text-gray-500">
          No members added yet.
        </div>
        <table v-else class="w-full border-collapse">
          <thead>
            <tr class="border-b border-gray-200 text-left text-sm font-medium text-gray-500">
              <th class="pb-3 pr-4">User ID</th>
              <th class="pb-3">Role</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="member in store.members"
              :key="member.user_id"
              class="border-b border-gray-100"
            >
              <td class="py-3 pr-4 text-sm">{{ member.user_id }}</td>
              <td class="py-3">
                <span
                  class="inline-block rounded px-2 py-0.5 text-xs"
                  :class="
                    member.role === 'admin'
                      ? 'bg-purple-100 text-purple-800'
                      : 'bg-gray-100 text-gray-800'
                  "
                >
                  {{ member.role }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </section>
    </div>
  </div>
</template>
