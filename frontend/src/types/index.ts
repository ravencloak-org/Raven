export interface User {
  id: string
  email: string
  displayName: string
  orgId: string
  orgRole: 'owner' | 'admin' | 'member'
}

export interface Organization {
  id: string
  name: string
  slug: string
  status: 'active' | 'suspended'
}

export interface Workspace {
  id: string
  orgId: string
  name: string
  slug: string
}

export interface KnowledgeBase {
  id: string
  workspaceId: string
  name: string
  description: string
  documentCount: number
}
