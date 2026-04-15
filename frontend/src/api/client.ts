import { useBillingStore } from '../stores/billing'

const BASE_URL = import.meta.env.VITE_API_URL || '/api'

interface RequestOptions {
  method?: string
  body?: unknown
  headers?: Record<string, string>
}

async function request<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, headers = {} } = options

  const response = await fetch(`${BASE_URL}${endpoint}`, {
    method,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...headers,
    },
    body: body ? JSON.stringify(body) : undefined,
  })

  if (response.status === 402) {
    // Payment required — show upgrade prompt via billing store
    try {
      const billingStore = useBillingStore()
      billingStore.showUpgradePrompt()
    } catch {
      // Store may not be available outside Pinia context; ignore
    }
    throw Object.assign(new Error('Payment required'), { status: 402 })
  }

  if (!response.ok) {
    const error = await response.json().catch(() => ({ message: response.statusText }))
    throw new Error(error.message || `Request failed with status ${response.status}`)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return response.json()
}

export const api = {
  get<T>(endpoint: string, headers?: Record<string, string>): Promise<T> {
    return request<T>(endpoint, { method: 'GET', headers })
  },

  post<T>(endpoint: string, body?: unknown, headers?: Record<string, string>): Promise<T> {
    return request<T>(endpoint, { method: 'POST', body, headers })
  },

  put<T>(endpoint: string, body?: unknown, headers?: Record<string, string>): Promise<T> {
    return request<T>(endpoint, { method: 'PUT', body, headers })
  },

  patch<T>(endpoint: string, body?: unknown, headers?: Record<string, string>): Promise<T> {
    return request<T>(endpoint, { method: 'PATCH', body, headers })
  },

  delete<T>(endpoint: string, headers?: Record<string, string>): Promise<T> {
    return request<T>(endpoint, { method: 'DELETE', headers })
  },
}
