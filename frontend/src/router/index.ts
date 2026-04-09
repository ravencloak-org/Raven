import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/dashboard',
    },
    {
      path: '/login',
      component: () => import('../layouts/AuthLayout.vue'),
      children: [
        {
          path: '',
          name: 'login',
          component: () => import('../pages/LoginPage.vue'),
        },
      ],
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
  if (to.meta.requiresAuth) {
    const auth = useAuthStore()
    if (!auth.isAuthenticated) {
      auth.login()
      return false
    }
  }

  if (to.name === 'login') {
    const auth = useAuthStore()
    if (auth.isAuthenticated) {
      return { name: 'dashboard' }
    }
  }
})

export default router
