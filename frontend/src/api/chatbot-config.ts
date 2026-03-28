import { useAuthStore } from '../stores/auth'

export interface ChatbotConfig {
  theme_color: string
  avatar_url: string
  welcome_text: string
  suggested_questions: string[]
  position: 'bottom-right' | 'bottom-left'
  widget_title: string
}

export type UpdateChatbotConfigRequest = Partial<ChatbotConfig>

// TODO: Replace with real API base URL when backend endpoints exist
const API_BASE = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

async function authFetch(path: string, init?: RequestInit): Promise<Response> {
  const auth = useAuthStore()
  return fetch(API_BASE + path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${auth.accessToken ?? ''}`,
      ...init?.headers,
    },
  })
}

// TODO: Remove mock data once backend chatbot-config endpoints are implemented
const mockConfig: ChatbotConfig = {
  theme_color: '#4f46e5',
  avatar_url: 'https://cdn.raven.example/avatar-default.svg',
  welcome_text: 'Hi there! How can I help you today?',
  suggested_questions: [
    'What is your return policy?',
    'How do I track my order?',
    'Can I speak with a human?',
  ],
  position: 'bottom-right',
  widget_title: 'Raven Chat',
}

export async function getChatbotConfig(): Promise<ChatbotConfig> {
  // TODO: Replace with real API call:
  // const res = await authFetch('/chatbot-config')
  // if (!res.ok) throw new Error(`getChatbotConfig failed: ${res.status}`)
  // return res.json()
  void authFetch // silence unused lint warning until real calls are wired
  return Promise.resolve({ ...mockConfig, suggested_questions: [...mockConfig.suggested_questions] })
}

export async function updateChatbotConfig(
  updates: UpdateChatbotConfigRequest,
): Promise<ChatbotConfig> {
  // TODO: Replace with real API call:
  // const res = await authFetch('/chatbot-config', {
  //   method: 'PUT',
  //   body: JSON.stringify(updates),
  // })
  // if (!res.ok) throw new Error(`updateChatbotConfig failed: ${res.status}`)
  // return res.json()
  if (updates.theme_color !== undefined) mockConfig.theme_color = updates.theme_color
  if (updates.avatar_url !== undefined) mockConfig.avatar_url = updates.avatar_url
  if (updates.welcome_text !== undefined) mockConfig.welcome_text = updates.welcome_text
  if (updates.suggested_questions !== undefined)
    mockConfig.suggested_questions = [...updates.suggested_questions]
  if (updates.position !== undefined) mockConfig.position = updates.position
  if (updates.widget_title !== undefined) mockConfig.widget_title = updates.widget_title
  return Promise.resolve({
    ...mockConfig,
    suggested_questions: [...mockConfig.suggested_questions],
  })
}
