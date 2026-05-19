import { createRouter, createWebHistory } from 'vue-router'
import { isVoiceEnabled } from '../lib/featureFlags'
import { useAuthStore } from '../stores/auth'
import { useServerConfigStore } from '../stores/server-config'
import { useOnboardingStore } from '../stores/onboarding'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      redirect: () => {
        const serverConfig = useServerConfigStore()
        if (serverConfig.singleUser) return '/dashboard'
        const auth = useAuthStore()
        return auth.isAuthenticated ? '/dashboard' : '/login'
      },
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('../pages/LoginPage.vue'),
    },
    {
      path: '/callback',
      name: 'callback',
      component: () => import('../pages/callback/CallbackPage.vue'),
    },
    {
      path: '/onboarding',
      name: 'onboarding',
      component: () => import('../pages/onboarding/OnboardingWizard.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/',
      component: () => import('../layouts/DefaultLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        {
          path: 'dashboard',
          name: 'dashboard',
          component: () => import('../pages/DashboardPage.vue'),
        },
        {
          path: 'orgs/:orgId',
          name: 'org-detail',
          component: () => import('../pages/orgs/OrgDetailPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'orgs/:orgId/workspaces',
          name: 'workspace-list',
          component: () => import('../pages/workspaces/WorkspaceListPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'orgs/:orgId/workspaces/:wsId',
          name: 'workspace-detail',
          component: () => import('../pages/workspaces/WorkspaceDetailPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'orgs/:orgId/workspaces/:wsId/knowledge-bases',
          name: 'kb-list',
          component: () => import('../pages/knowledge-bases/KBListPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'orgs/:orgId/workspaces/:wsId/knowledge-bases/:kbId',
          name: 'kb-detail',
          component: () => import('../pages/knowledge-bases/KBDetailPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'analytics',
          name: 'analytics',
          component: () => import('../pages/analytics/AnalyticsDashboardPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'api-keys',
          name: 'api-keys',
          component: () => import('../pages/apikeys/ApiKeyListPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'llm-providers',
          name: 'llm-providers',
          component: () => import('../pages/llm-providers/LlmProviderListPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'settings',
          name: 'settings',
          component: () => import('../pages/settings/SettingsPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'chatbot-config',
          name: 'chatbot-config',
          component: () => import('../pages/chatbot/ChatbotConfiguratorPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'sandbox',
          name: 'test-sandbox',
          component: () => import('../pages/sandbox/TestSandboxPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'orgs/:orgId/voice',
          name: 'voice-session-list',
          component: () => import('../pages/voice/VoiceSessionListPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'orgs/:orgId/voice/:sessionId',
          name: 'voice-session-detail',
          component: () =>
            import('../pages/voice/VoiceSessionDetailPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'orgs/:orgId/whatsapp/phone-numbers',
          name: 'whatsapp-phone-numbers',
          component: () => import('../pages/whatsapp/PhoneNumbersPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'orgs/:orgId/whatsapp/calls',
          name: 'whatsapp-calls',
          component: () => import('../pages/whatsapp/CallsPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'orgs/:orgId/billing',
          name: 'billing',
          component: () => import('../pages/billing/BillingPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'orgs/:orgId/billing/upgrade',
          name: 'billing-checkout',
          component: () => import('../pages/billing/CheckoutPage.vue'),
          meta: { requiresAuth: true },
        },
        {
          path: 'orgs/:orgId/billing/plans',
          name: 'billing-plans',
          component: () => import('../pages/billing/PlansPage.vue'),
          meta: { requiresAuth: true },
        },
      ],
    },
    {
      path: '/legal',
      component: () => import('../layouts/AuthLayout.vue'),
      children: [
        {
          path: 'privacy',
          name: 'privacy-policy',
          component: () => import('../pages/legal/PrivacyPolicyPage.vue'),
        },
        {
          path: 'terms',
          name: 'terms-of-service',
          component: () => import('../pages/legal/TermsOfServicePage.vue'),
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('../pages/NotFoundPage.vue'),
    },
  ],
})

router.beforeEach(async (to) => {
  // Public-demo gate: voice routes redirect to the dashboard when voice
  // is disabled. The demo runs behind Cloudflare Tunnel which can't carry
  // WebRTC's UDP, so the voice UI is hidden end-to-end.
  if (!isVoiceEnabled() && to.name && String(to.name).startsWith('voice-')) {
    return '/dashboard'
  }

  const serverConfig = useServerConfigStore()
  const auth = useAuthStore()

  // In single-user (Raven AI) mode there is no login flow — the app always
  // boots directly to the dashboard. Skip the login/callback pages entirely.
  // Allow /onboarding through when the first-run wizard hasn't been completed
  // yet; once complete the onboarding store will redirect to /dashboard itself.
  if (serverConfig.singleUser) {
    if (to.path === '/login' || to.path === '/callback') {
      return '/dashboard'
    }
    if (to.path === '/onboarding') {
      const onboarding = useOnboardingStore()
      if (onboarding.completed) return '/dashboard'
      return // let the wizard through
    }
    return
  }

  if (to.path === '/login' || to.path === '/callback') return
  if (to.path.startsWith('/legal/')) return

  if (!auth.isAuthenticated) {
    await auth.init()
  }

  if (to.meta.requiresAuth === true && !auth.isAuthenticated) {
    return '/login'
  }

  if (auth.isAuthenticated && !auth.hasOrg && to.path !== '/onboarding') {
    return '/onboarding'
  }
})

export default router
