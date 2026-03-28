import { defineStore } from 'pinia'
import { ref } from 'vue'
import { filter } from 'remeda'
import {
  listWorkspaces,
  getWorkspace,
  createWorkspace,
  deleteWorkspace,
  addMember,
  type Workspace,
  type WorkspaceMember,
} from '../api/workspaces'

export const useWorkspacesStore = defineStore('workspaces', () => {
  const workspaces = ref<Workspace[]>([])
  const currentWorkspace = ref<Workspace | null>(null)
  const total = ref(0)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const members = ref<WorkspaceMember[]>([])

  async function fetchWorkspaces(orgId: string, offset = 0, limit = 20) {
    loading.value = true
    error.value = null
    try {
      const res = await listWorkspaces(orgId, offset, limit)
      workspaces.value = res.items
      total.value = res.total
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function fetchWorkspace(orgId: string, wsId: string) {
    loading.value = true
    error.value = null
    try {
      currentWorkspace.value = await getWorkspace(orgId, wsId)
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      loading.value = false
    }
  }

  async function create(orgId: string, name: string): Promise<Workspace> {
    const ws = await createWorkspace(orgId, name)
    workspaces.value.push(ws)
    total.value += 1
    return ws
  }

  async function remove(orgId: string, wsId: string): Promise<void> {
    await deleteWorkspace(orgId, wsId)
    workspaces.value = filter(workspaces.value, (w) => w.id !== wsId)
    total.value -= 1
    if (currentWorkspace.value?.id === wsId) {
      currentWorkspace.value = null
    }
  }

  async function addWorkspaceMember(
    orgId: string,
    wsId: string,
    userId: string,
    role: string,
  ): Promise<WorkspaceMember> {
    const member = await addMember(orgId, wsId, userId, role)
    members.value.push(member)
    return member
  }

  return {
    workspaces,
    currentWorkspace,
    total,
    loading,
    error,
    members,
    fetchWorkspaces,
    fetchWorkspace,
    create,
    remove,
    addWorkspaceMember,
  }
})
