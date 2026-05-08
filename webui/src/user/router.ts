import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '@shared/stores/auth'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/login', component: () => import('@/pages/login/LoginPage.vue'), meta: { public: true } },
    { path: '/chat', component: () => import('@/pages/portal/ChatPage.vue') },
    { path: '/memories', component: () => import('@/pages/portal/MemoriesPage.vue') },
    { path: '/skills', component: () => import('@/pages/portal/SkillsPage.vue') },
    { path: '/usage', component: () => import('@/pages/portal/UsagePage.vue') },
    { path: '/:pathMatch(.*)*', redirect: '/login' },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.public) return true
  if (!auth.connected) return '/login'
  return true
})

export default router
