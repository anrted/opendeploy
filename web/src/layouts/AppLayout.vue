<template>
  <div class="min-h-screen flex bg-bg-base text-text-main transition-colors duration-200">
    <ToastHost />
    <div v-if="mobileOpen" class="fixed inset-0 z-40 bg-black/60 lg:hidden" @click="mobileOpen = false"></div>
    <!-- Sidebar -->
    <aside
      class="fixed inset-y-0 left-0 z-50 w-72 flex-shrink-0 flex flex-col border-r border-border-subtle bg-bg-card transition-transform duration-200 lg:static lg:z-auto lg:w-64 lg:translate-x-0"
      :class="mobileOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <!-- Logo -->
      <div class="p-6 border-b border-border-subtle">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div class="w-8 h-8 rounded-lg bg-gradient-to-br from-indigo-500 to-violet-600 flex items-center justify-center shadow-lg shadow-indigo-500/30">
              <svg class="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                      d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
              </svg>
            </div>
            <div>
              <div class="text-sm font-bold text-text-main">OpenDeploy</div>
              <div class="text-xs text-text-muted">v1.0.0</div>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <!-- Theme Toggle -->
            <button @click="themeStore.toggle()" class="text-text-muted hover:text-text-main transition-colors">
              <svg v-if="themeStore.isDark" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" /></svg>
              <svg v-else class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" /></svg>
            </button>
            <LanguageSwitcher />
            <button v-if="mobileOpen" class="ml-1 rounded-lg p-1 text-text-muted hover:text-text-main lg:hidden" :aria-label="$t('sidebar.menu')" @click="mobileOpen = false">
              <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
            </button>
          </div>
        </div>
      </div>

      <!-- Navigation -->
      <nav class="flex-1 p-4 space-y-1 overflow-y-auto">
        <router-link v-for="item in navItems" :key="item.name"
          :to="item.to"
          custom
          v-slot="{ isActive, navigate }">
          <button @click="navigate(); mobileOpen = false"
            :class="['nav-item w-full', isActive ? 'active' : '']">
            <component :is="item.icon" class="w-4 h-4 flex-shrink-0" />
            {{ $te('sidebar.' + item.name) ? $t('sidebar.' + item.name) : item.label }}
          </button>
        </router-link>
      </nav>

      <!-- User info -->
      <div class="p-4 border-t border-border-subtle">
        <div class="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-slate-100 dark:hover:bg-[#1e2535] cursor-pointer transition-colors"
             @click="handleLogout">
          <div class="w-8 h-8 rounded-full bg-gradient-to-br from-indigo-500 to-violet-600 flex items-center justify-center text-xs font-bold text-white">
            {{ userInitial }}
          </div>
          <div class="flex-1 min-w-0">
            <div class="text-sm font-medium text-text-main truncate">{{ auth.user?.username || 'User' }}</div>
            <div class="text-xs text-text-muted truncate capitalize">{{ auth.user?.role || 'admin' }}</div>
          </div>
          <svg class="w-4 h-4 text-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
          </svg>
        </div>
      </div>
    </aside>

    <!-- Main content -->
    <main class="min-w-0 flex-1 overflow-y-auto">
      <header class="sticky top-0 z-30 flex h-16 items-center justify-between border-b border-border-subtle bg-bg-card/95 px-4 backdrop-blur lg:hidden">
        <button class="rounded-lg p-2 text-text-main hover:bg-slate-100 dark:hover:bg-[#1e2535]" :aria-label="$t('sidebar.menu')" @click="mobileOpen = true">
          <svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"/></svg>
        </button>
        <span class="text-sm font-bold">OpenDeploy</span>
        <LanguageSwitcher />
      </header>
      <div class="p-4 sm:p-6 lg:p-8">
        <router-view />
      </div>
    </main>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'
import ToastHost from '@/components/ToastHost.vue'

const themeStore = useThemeStore()
const mobileOpen = ref(false)

// Icon components (inline SVGs as functional components)
const DashboardIcon = {
  template: `<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"/></svg>`
}
const ModulesIcon = {
  template: `<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>`
}
const SitesIcon = {
  template: `<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"/></svg>`
}
const ServicesIcon = {
  template: `<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"/></svg>`
}
const SettingsIcon = {
  template: `<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/></svg>`
}
const FirewallIcon = {
  template: `<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>`
}
const ProcessesIcon = {
  template: `<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h16"/></svg>`
}
const ServersIcon = {
  template: `<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="6" rx="2" stroke-width="2"/><rect x="3" y="14" width="18" height="6" rx="2" stroke-width="2"/><path stroke-linecap="round" stroke-width="2" d="M7 7h.01M7 17h.01M11 7h6M11 17h6"/></svg>`
}
const UsersIcon = {
  template: `<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a4 4 0 00-4-4h-1M9 20H2v-2a4 4 0 014-4h3m7-6a4 4 0 11-8 0 4 4 0 018 0z"/></svg>`
}
const TasksIcon = {
  template: `<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12h14V7a2 2 0 00-2-2h-2M9 5a3 3 0 006 0M9 12l2 2 4-4"/></svg>`
}
const CronIcon = {
  template: `<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9" stroke-width="2"/><path stroke-linecap="round" stroke-width="2" d="M12 7v5l3 2"/></svg>`
}
const LogsIcon = {
  template: `<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/></svg>`
}

const allNavItems = [
  { name: 'dashboard', label: 'Dashboard', to: '/', icon: DashboardIcon },
  { name: 'servers',   label: 'Servers',   to: '/servers', icon: ServersIcon },
  { name: 'modules',   label: 'Modules',   to: '/modules', icon: ModulesIcon },
  { name: 'sites',     label: 'Sites',     to: '/sites', icon: SitesIcon },
  { name: 'services',  label: 'Services',  to: '/services', icon: ServicesIcon },
  { name: 'tasks',     label: 'Tasks',     to: '/tasks', icon: TasksIcon },
  { name: 'cron',      label: 'Cron',      to: '/cron', icon: CronIcon },
  { name: 'processes', label: 'Processes', to: '/processes', icon: ProcessesIcon },
  { name: 'logs',      label: 'System Logs', to: '/logs', icon: LogsIcon },
  { name: 'users',     label: 'Users',     to: '/users', icon: UsersIcon, adminOnly: true },
  { name: 'firewall',  label: 'Firewall',  to: '/modules/firewall', icon: FirewallIcon },
  { name: 'settings',  label: 'Settings',  to: '/settings', icon: SettingsIcon },
]

const auth = useAuthStore()
const navItems = computed(() => allNavItems.filter(item => !item.adminOnly || auth.user?.role === 'admin'))
const router = useRouter()
watch(() => router.currentRoute.value.fullPath, () => { mobileOpen.value = false })

const userInitial = computed(() =>
  (auth.user?.username?.[0] || 'U').toUpperCase()
)

async function handleLogout() {
  await auth.logout()
  router.push('/login')
}
</script>
