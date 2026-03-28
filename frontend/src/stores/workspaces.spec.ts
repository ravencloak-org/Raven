import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useWorkspacesStore } from './workspaces'
import type { Workspace, WorkspaceListResponse, WorkspaceMember } from '../api/workspaces'

vi.mock('../api/workspaces', () => ({
  listWorkspaces: vi.fn(),
  getWorkspace: vi.fn(),
  createWorkspace: vi.fn(),
  deleteWorkspace: vi.fn(),
  addMember: vi.fn(),
}))

import {
  listWorkspaces,
  getWorkspace,
  createWorkspace,
  deleteWorkspace,
  addMember,
} from '../api/workspaces'

const mockedListWorkspaces = vi.mocked(listWorkspaces)
const mockedGetWorkspace = vi.mocked(getWorkspace)
const mockedCreateWorkspace = vi.mocked(createWorkspace)
const mockedDeleteWorkspace = vi.mocked(deleteWorkspace)
const mockedAddMember = vi.mocked(addMember)

const ORG_ID = 'org-1'

function fakeWorkspace(overrides: Partial<Workspace> = {}): Workspace {
  return {
    id: 'ws-1',
    org_id: ORG_ID,
    name: 'My Workspace',
    slug: 'my-workspace',
    settings: {},
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('useWorkspacesStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('fetchWorkspaces', () => {
    it('populates workspaces and total on success', async () => {
      const ws = fakeWorkspace()
      const response: WorkspaceListResponse = {
        items: [ws],
        total: 1,
        offset: 0,
        limit: 20,
      }
      mockedListWorkspaces.mockResolvedValue(response)

      const store = useWorkspacesStore()
      await store.fetchWorkspaces(ORG_ID)

      expect(mockedListWorkspaces).toHaveBeenCalledWith(ORG_ID, 0, 20)
      expect(store.workspaces).toEqual([ws])
      expect(store.total).toBe(1)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('sets error on failure', async () => {
      mockedListWorkspaces.mockRejectedValue(new Error('listWorkspaces failed: 500'))

      const store = useWorkspacesStore()
      await store.fetchWorkspaces(ORG_ID)

      expect(store.workspaces).toEqual([])
      expect(store.error).toBe('listWorkspaces failed: 500')
      expect(store.loading).toBe(false)
    })

    it('passes offset and limit through', async () => {
      mockedListWorkspaces.mockResolvedValue({
        items: [],
        total: 0,
        offset: 10,
        limit: 5,
      })

      const store = useWorkspacesStore()
      await store.fetchWorkspaces(ORG_ID, 10, 5)

      expect(mockedListWorkspaces).toHaveBeenCalledWith(ORG_ID, 10, 5)
    })
  })

  describe('fetchWorkspace', () => {
    it('sets currentWorkspace on success', async () => {
      const ws = fakeWorkspace()
      mockedGetWorkspace.mockResolvedValue(ws)

      const store = useWorkspacesStore()
      await store.fetchWorkspace(ORG_ID, 'ws-1')

      expect(mockedGetWorkspace).toHaveBeenCalledWith(ORG_ID, 'ws-1')
      expect(store.currentWorkspace).toEqual(ws)
      expect(store.loading).toBe(false)
    })

    it('sets error on failure', async () => {
      mockedGetWorkspace.mockRejectedValue(new Error('getWorkspace failed: 404'))

      const store = useWorkspacesStore()
      await store.fetchWorkspace(ORG_ID, 'ws-1')

      expect(store.currentWorkspace).toBeNull()
      expect(store.error).toBe('getWorkspace failed: 404')
    })
  })

  describe('create', () => {
    it('appends new workspace and increments total', async () => {
      const ws = fakeWorkspace({ id: 'ws-new', name: 'New WS' })
      mockedCreateWorkspace.mockResolvedValue(ws)

      const store = useWorkspacesStore()
      store.total = 3

      const result = await store.create(ORG_ID, 'New WS')

      expect(mockedCreateWorkspace).toHaveBeenCalledWith(ORG_ID, 'New WS')
      expect(result).toEqual(ws)
      expect(store.workspaces).toContainEqual(ws)
      expect(store.total).toBe(4)
    })

    it('propagates API errors', async () => {
      mockedCreateWorkspace.mockRejectedValue(new Error('createWorkspace failed: 400'))

      const store = useWorkspacesStore()

      await expect(store.create(ORG_ID, '')).rejects.toThrow('createWorkspace failed: 400')
    })
  })

  describe('remove', () => {
    it('removes workspace from list and decrements total', async () => {
      mockedDeleteWorkspace.mockResolvedValue(undefined)

      const store = useWorkspacesStore()
      const ws = fakeWorkspace()
      store.workspaces = [ws]
      store.total = 1

      await store.remove(ORG_ID, 'ws-1')

      expect(mockedDeleteWorkspace).toHaveBeenCalledWith(ORG_ID, 'ws-1')
      expect(store.workspaces).toEqual([])
      expect(store.total).toBe(0)
    })

    it('clears currentWorkspace if it was the deleted one', async () => {
      mockedDeleteWorkspace.mockResolvedValue(undefined)

      const store = useWorkspacesStore()
      const ws = fakeWorkspace()
      store.currentWorkspace = ws
      store.workspaces = [ws]
      store.total = 1

      await store.remove(ORG_ID, 'ws-1')

      expect(store.currentWorkspace).toBeNull()
    })

    it('propagates API errors', async () => {
      mockedDeleteWorkspace.mockRejectedValue(new Error('deleteWorkspace failed: 403'))

      const store = useWorkspacesStore()

      await expect(store.remove(ORG_ID, 'ws-1')).rejects.toThrow(
        'deleteWorkspace failed: 403',
      )
    })
  })

  describe('addWorkspaceMember', () => {
    it('adds member to the members list', async () => {
      const member: WorkspaceMember = { user_id: 'u-1', role: 'editor' }
      mockedAddMember.mockResolvedValue(member)

      const store = useWorkspacesStore()
      const result = await store.addWorkspaceMember(ORG_ID, 'ws-1', 'u-1', 'editor')

      expect(mockedAddMember).toHaveBeenCalledWith(ORG_ID, 'ws-1', 'u-1', 'editor')
      expect(result).toEqual(member)
      expect(store.members).toContainEqual(member)
    })

    it('propagates API errors', async () => {
      mockedAddMember.mockRejectedValue(new Error('addMember failed: 409'))

      const store = useWorkspacesStore()

      await expect(
        store.addWorkspaceMember(ORG_ID, 'ws-1', 'u-1', 'editor'),
      ).rejects.toThrow('addMember failed: 409')
    })
  })
})
