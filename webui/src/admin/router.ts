import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '@shared/stores/auth'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/login', component: () => import('@/pages/login/AdminLoginPage.vue'), meta: { public: true } },
    { path: '/bootstrap', component: () => import('@/pages/bootstrap/BootstrapPage.vue'), meta: { public: true } },
    { path: '/tenants', component: () => import('@/pages/admin/TenantsPage.vue') },
    { path: '/keys', component: () => import('@/pages/admin/ApiKeysPage.vue') },
    { path: '/audit', component: () => import('@/pages/admin/AuditLogsPage.vue') },
    { path: '/pricing', component: () => import('@/pages/admin/PricingPage.vue') },
    { path: '/sandbox', component: () => import('@/pages/admin/SandboxPage.vue') },
    { path: '/:pathMatch(.*)*', redirect: '/tenants' },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (to.meta.public) return true
  if (!auth.connected) {
    // Check bootstrap status
    try {
      const res = await fetch('/admin/v1/bootstrap/status')
      const data = await res.json() as { bootstrap_required: boolean }
      if (data.bootstrap_required) return '/bootstrap'
    } catch { /* ignore */ }
    return '/login'
  }
  return true
})

export default router
