import type { ProviderType } from '../../api/llm-providers'

export interface ProviderHelpEntry {
  keyHref?: string
  keyPrefix?: string
  helpText: string
  requiresKey: boolean
  defaultBaseUrl?: string
  baseUrlHelp?: string
}

// Per-provider onboarding guidance shown in the Add-Provider dialog.
// `keyHref` deep-links to the vendor's API-key console so the user can
// mint a key without leaving the flow; `keyPrefix` is shown in the
// placeholder so they know what shape the key should have; `helpText`
// is a one-liner above the API-key field. Keyed off ProviderType.
// `requiresKey=false` hides API-key inputs entirely (Edit dialog skips
// Rotate-key for these; today only Ollama). `defaultBaseUrl` auto-fills
// the Base URL input when the provider type flips; `baseUrlHelp` is a
// one-liner shown below the input.
export const PROVIDER_HELP: Record<ProviderType, ProviderHelpEntry> = {
  openai: {
    keyHref: 'https://platform.openai.com/api-keys',
    keyPrefix: 'sk-...',
    helpText: 'Generate a Secret Key in the OpenAI dashboard and paste it below.',
    requiresKey: true,
  },
  anthropic: {
    keyHref: 'https://console.anthropic.com/settings/keys',
    keyPrefix: 'sk-ant-...',
    helpText: 'Generate an API key in the Anthropic Console and paste it below.',
    requiresKey: true,
  },
  ollama: {
    helpText:
      "Ollama runs locally and needs no API key. Point Base URL at your Ollama daemon — http://localhost:11434 by default. The hosted demo can't reach a private LAN address, so this works for self-hosted / Tauri-desktop Raven, or expose your Ollama via a public tunnel.",
    requiresKey: false,
    defaultBaseUrl: 'http://localhost:11434',
    baseUrlHelp:
      'Address of the Ollama daemon (default port 11434). Must be reachable from wherever the Raven server runs.',
  },
  custom: {
    keyPrefix: "your provider's key",
    helpText:
      'For any OpenAI-compatible provider (Together, Groq, Fireworks, vLLM, etc.). Set Base URL to the provider root (Raven appends /v1/models, /v1/chat/completions, etc).',
    requiresKey: true,
  },
}

export const PROVIDER_TYPES: { value: ProviderType; label: string }[] = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'ollama', label: 'Ollama' },
  { value: 'custom', label: 'Custom' },
]

// Pretty-print provider type for the card body. Keep in lockstep with
// PROVIDER_TYPES above — single source of truth for label text.
export const PROVIDER_TYPE_LABEL: Record<ProviderType, string> = {
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  ollama: 'Ollama',
  custom: 'Custom',
}
