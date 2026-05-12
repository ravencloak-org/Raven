import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { useAuthStore } from './stores/auth'
import { useServerConfigStore } from './stores/server-config'
import { usePrecheckStore } from './stores/precheck'
import { posthogPlugin } from './plugins/posthog'
import { initSuperTokens } from './plugins/supertokens'
import App from './App.vue'
import router from './router'
import './assets/main.css'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)
app.use(posthogPlugin, { router })

// Fetch server feature flags before mounting so the router guard has them.
const serverConfig = useServerConfigStore()
serverConfig.load().then(() => {
  // In single-user (Raven AI) mode SuperTokens is not running on the server;
  // skip SDK initialisation to avoid unnecessary network requests to /auth/*.
  if (!serverConfig.singleUser) {
    initSuperTokens()
  }

  const authStore = useAuthStore()
  if (serverConfig.singleUser) {
    // Single-user mode: no real session — treat as always authenticated.
    authStore.setLocalMode()
    // Subscribe to the Tauri shell's precheck:result event so the model-picker
    // step has the host's RAM tier ready. No-op outside a Tauri context.
    usePrecheckStore().subscribeToTauri()
    app.mount('#app')
  } else {
    // Multi-user mode: initialise session check before mounting.
    authStore.init().then(() => app.mount('#app'))
  }
})
