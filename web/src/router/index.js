import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { public: true },
  },
  {
    path: '/',
    component: () => import('@/layouts/AppLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: 'users',
        name: 'users',
        component: () => import('@/views/UsersView.vue'),
        meta: { adminOnly: true },
      },
      {
        path: '',
        name: 'dashboard',
        component: () => import('@/views/DashboardView.vue'),
      },
      {
        path: 'servers',
        name: 'servers',
        component: () => import('@/views/ServersView.vue'),
      },
      {
        path: 'servers/:id',
        name: 'server_details',
        component: () => import('@/views/ServerDetailsView.vue'),
      },
      {
        path: 'modules',
        name: 'modules',
        component: () => import('@/views/ModulesView.vue'),
      },
      {
        path: 'modules/:id',
        name: 'module_details',
        component: () => import('@/views/ModuleDetailsView.vue'),
      },
      {
        path: 'modules/firewall',
        name: 'module_firewall',
        component: () => import('@/views/modules/FirewallView.vue'),
      },
      {
        path: 'sites',
        name: 'sites',
        component: () => import('@/views/SitesView.vue'),
      },
      {
        path: 'services',
        name: 'services',
        component: () => import('@/views/ServicesView.vue'),
      },
      {
        path: 'settings',
        name: 'settings',
        component: () => import('@/views/SettingsView.vue'),
      },
      {
        path: 'tasks',
        name: 'tasks',
        component: () => import('@/views/TasksView.vue'),
      },
      {
        path: 'cron',
        name: 'cron',
        component: () => import('@/views/CronView.vue'),
      },
      {
        path: 'processes',
        name: 'processes',
        component: () => import('@/views/ProcessesView.vue'),
      },
      {
        path: 'logs',
        name: 'logs',
        component: () => import('@/views/LogsView.vue'),
      },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Navigation guard: redirect to login if not authenticated
router.beforeEach((to) => {
  const auth = useAuthStore()
  if (!to.meta.public && !auth.isAuthenticated) {
    return { name: 'login' }
  }
  if (to.name === 'login' && auth.isAuthenticated) {
    return { name: 'dashboard' }
  }
  if (to.meta.adminOnly && auth.user?.role !== 'admin') {
    return { name: 'dashboard' }
  }
})

export default router
