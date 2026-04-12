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

// Initialise Keycloak before mounting so the auth store is ready when the
// router guard evaluates.  With check-sso the app mounts regardless of
// whether the user is logged in — the router guard handles redirects.
const authStore = useAuthStore()
authStore.init().then(() => app.mount('#app'))
