import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { useAuthStore } from './stores/auth'
import { posthogPlugin } from './plugins/posthog'
import App from './App.vue'
import router from './router'
import './assets/main.css'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)
app.use(posthogPlugin, { router })

// Initialise Keycloak before mounting.
// With onLoad: 'login-required', init() either:
//   a) redirects to Keycloak (never resolves on this page load), or
//   b) exchanges the code from the URL and resolves with authenticated=true.
// The app only mounts after auth succeeds — no router guard needed.
const authStore = useAuthStore()
authStore.init().then(() => {
  console.log('[main] auth ready, mounting app')
  app.mount('#app')
})
