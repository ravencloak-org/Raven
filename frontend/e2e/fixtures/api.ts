import { type APIRequestContext } from '@playwright/test'

export class APIClient {
  constructor(
    private req: APIRequestContext,
    private baseURL: string,
  ) {}

  async createKB(workspaceId: string, name: string) {
    const resp = await this.req.post(`${this.baseURL}/api/v1/knowledge-bases`, {
      data: { workspace_id: workspaceId, name },
    })
    return resp.json()
  }

  async createAPIKey(workspaceId: string, kbId?: string) {
    const resp = await this.req.post(`${this.baseURL}/api/v1/api-keys`, {
      data: { workspace_id: workspaceId, kb_id: kbId ?? null },
    })
    return resp.json()
  }

  async uploadDocument(kbId: string, content: Buffer, filename: string) {
    const resp = await this.req.post(`${this.baseURL}/api/v1/documents`, {
      multipart: {
        kb_id: kbId,
        file: { name: filename, mimeType: 'text/plain', buffer: content },
      },
    })
    return resp.json()
  }
}
