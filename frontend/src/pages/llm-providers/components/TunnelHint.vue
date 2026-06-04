<script setup lang="ts">
import { ref } from 'vue'

defineProps<{
  ravenHost: string
  baseUrl: string
}>()

// Tunnel suggestions shown in the Ollama branch so a user on the
// hosted demo (whose Vultr box can't reach `localhost`) has a one-liner
// to expose their local daemon publicly. Cloudflare's `trycloudflare`
// mode is the recommended first option — no account, no install of
// anything Raven-side, free, auto-TLS. ngrok is the fallback for users
// who already have it set up.
//
// `install` carries platform-specific install one-liners + a download
// docs URL for users who don't already have the binary. Detecting the
// host OS via navigator.userAgent picks the matching command; the docs
// link is always shown as the universal fallback.
const ollamaTunnels: {
  label: string
  command: string
  note?: string
  install: { docsHref: string; macos: string; windows: string; linux: string }
}[] = [
  {
    label: 'Cloudflare Tunnel (recommended)',
    // --http-host-header localhost is critical: Ollama's HTTP server rejects
    // any request whose Host header isn't 127.0.0.1 / localhost (security
    // hardening). cloudflared's default forwards the public tunnel hostname
    // as Host, which Ollama returns 403 to with an empty body. The flag
    // tells cloudflared to rewrite Host to "localhost" on the upstream hop
    // so Ollama treats it as a local call.
    command: 'cloudflared tunnel --url http://localhost:11434 --http-host-header localhost',
    note: 'Prints an https://<random>.trycloudflare.com URL. No account needed. The --http-host-header flag is mandatory — Ollama 403s any other Host. Paste the printed URL into Base URL above.',
    install: {
      docsHref:
        'https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/',
      macos: 'brew install cloudflared',
      windows: 'winget install --id Cloudflare.cloudflared',
      linux: "See the download page for your distro's package or static binary.",
    },
  },
  {
    label: 'ngrok',
    // --host-header=rewrite serves the same purpose as cloudflared's
    // --http-host-header: rewrites the Host on the upstream hop to
    // 127.0.0.1:11434 so Ollama's Host-allowlist check accepts the call.
    // Without it, ngrok forwards Host: <subdomain>.ngrok.app and Ollama 403s.
    command: 'ngrok http 11434 --host-header=rewrite',
    note: 'Free account required for stable URLs. The --host-header=rewrite flag is mandatory — Ollama 403s any other Host. Use the https Forwarding address.',
    install: {
      docsHref: 'https://ngrok.com/download',
      macos: 'brew install ngrok',
      windows: 'winget install ngrok.ngrok',
      linux:
        'curl -sSL https://ngrok-agent.s3.amazonaws.com/ngrok.asc | sudo tee /etc/apt/trusted.gpg.d/ngrok.asc >/dev/null && echo "deb https://ngrok-agent.s3.amazonaws.com buster main" | sudo tee /etc/apt/sources.list.d/ngrok.list && sudo apt update && sudo apt install ngrok',
    },
  },
]

// Best-effort platform detection — userAgent is fine here because we
// only use it to pick a default install one-liner, not for anything
// security-sensitive. Falls back to the docs URL if unsure.
function detectOS(): 'macos' | 'windows' | 'linux' | 'unknown' {
  const ua = navigator.userAgent.toLowerCase()
  if (ua.includes('mac')) return 'macos'
  if (ua.includes('win')) return 'windows'
  if (ua.includes('linux')) return 'linux'
  return 'unknown'
}
const userOS = detectOS()
function installCommandForOS(t: (typeof ollamaTunnels)[number]) {
  if (userOS === 'macos') return t.install.macos
  if (userOS === 'windows') return t.install.windows
  if (userOS === 'linux') return t.install.linux
  return null
}
function installLabelForOS(): string {
  if (userOS === 'macos') return 'Install on macOS'
  if (userOS === 'windows') return 'Install on Windows'
  if (userOS === 'linux') return 'Install on Linux'
  return 'Install'
}

const copied = ref<string | null>(null)
async function copyTunnel(command: string) {
  try {
    await navigator.clipboard.writeText(command)
    copied.value = command
    setTimeout(() => (copied.value === command ? (copied.value = null) : null), 1500)
  } catch {
    // Clipboard access can fail under restrictive permissions / older
    // browsers — surface the command in the UI and let the user copy
    // it manually instead of throwing.
    copied.value = null
  }
}
</script>

<template>
  <div class="mt-3 rounded-md border border-gray-200 bg-gray-50 p-3">
    <p class="text-xs font-medium text-gray-700">
      {{ ravenHost }} can't reach <code class="font-mono">{{ baseUrl }}</code> on your machine. Expose Ollama temporarily:
    </p>
    <ul class="mt-2 space-y-3">
      <li v-for="t in ollamaTunnels" :key="t.label" class="text-xs">
        <div class="font-medium text-gray-600">{{ t.label }}</div>
        <!-- Install one-liner for the user's detected OS, with a
             fallback link to the upstream download docs that
             cover Linux distros + manual installs. -->
        <div v-if="installCommandForOS(t)" class="mt-0.5">
          <div class="text-[11px] text-gray-500">{{ installLabelForOS() }}:</div>
          <div class="mt-0.5 flex items-center gap-1.5">
            <code class="flex-1 truncate rounded bg-gray-800 px-2 py-1 text-[11px] text-gray-100">{{ installCommandForOS(t) }}</code>
            <button
              type="button"
              class="shrink-0 rounded border border-gray-300 bg-white px-2 py-1 text-[11px] text-gray-700 hover:bg-gray-100"
              @click="copyTunnel(installCommandForOS(t) ?? '')"
            >
              {{ copied === installCommandForOS(t) ? 'Copied' : 'Copy' }}
            </button>
          </div>
        </div>
        <a
          :href="t.install.docsHref"
          target="_blank"
          rel="noopener noreferrer"
          class="mt-1 inline-flex items-center text-[11px] text-indigo-600 underline hover:text-indigo-800"
        >
          Other install options
          <svg class="ml-0.5 h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><path d="M15 3h6v6"/><path d="M10 14L21 3"/></svg>
        </a>
        <div class="mt-1.5">
          <div class="text-[11px] text-gray-500">Then run:</div>
          <div class="mt-0.5 flex items-center gap-1.5">
            <code class="flex-1 truncate rounded bg-gray-900 px-2 py-1 text-[11px] text-gray-100">{{ t.command }}</code>
            <button
              type="button"
              class="shrink-0 rounded border border-gray-300 bg-white px-2 py-1 text-[11px] text-gray-700 hover:bg-gray-100"
              @click="copyTunnel(t.command)"
            >
              {{ copied === t.command ? 'Copied' : 'Copy' }}
            </button>
          </div>
        </div>
        <p v-if="t.note" class="mt-0.5 text-gray-500">{{ t.note }}</p>
      </li>
    </ul>
    <p class="mt-2 text-[11px] text-amber-700">
      <strong>Heads up:</strong> a public tunnel exposes your local Ollama to anyone with the URL. Close the tunnel when you're done, or restrict by IP / auth.
    </p>
  </div>
</template>
