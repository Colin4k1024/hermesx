import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes = [
  {
    path: '/connect',
    component: () => import('@/pages/ConnectPage.vue'),
    meta: { public: true },
  },
  {
    path: '/chat',
    component: () => import('@/pages/ChatPage.vue'),
  },
  {
    path: '/memories',
    component: () => import('@/pages/MemoriesPage.vue'),
  },
  {
    path: '/skills',
    component: () => import('@/pages/SkillsPage.vue'),
  },
  {
    path: '/admin/skills',
    component: () => import('@/pages/admin/AdminSkillsPage.vue'),
    meta: { requiresAdmin: true },
  },
  {
    path: '/admin/tenants',
    component: () => import('@/pages/admin/AdminTenantsPage.vue'),
    meta: { requiresAdmin: true },
  },
  {
    path: '/admin/keys',
    component: () => import('@/pages/admin/AdminKeysPage.vue'),
    meta: { requiresAdmin: true },
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/connect',
  },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.public) {
    if (auth.connected && to.path === '/connect') return '/chat'
    return true
  }
  if (!auth.connected) return '/connect'
  if (to.meta.requiresAdmin && !auth.isAdmin) return '/chat'
  return true
})

export default router
