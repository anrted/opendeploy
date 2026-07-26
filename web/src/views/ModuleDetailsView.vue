<template>
  <div class="h-full overflow-y-auto py-2 custom-scrollbar sm:py-4">
    <div v-if="loading" class="flex justify-center py-20">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-500"></div>
    </div>
    
    <div v-else-if="errorMessage" class="bg-red-500/10 border border-red-500/20 text-red-400 p-4 rounded-xl text-sm">
      {{ errorMessage }}
    </div>

    <div v-else-if="module">
      <!-- Header -->
      <div class="mb-8 flex flex-col items-start justify-between gap-5 lg:flex-row">
        <div class="flex min-w-0 items-center gap-4">
          <ModuleIcon :icon="module.icon || 'box'" size="lg" />
          <div>
            <div class="flex items-center gap-3 mb-1">
              <h1 class="text-2xl font-bold text-white">{{ module.name }}</h1>
              <span class="badge" :class="stateBadge(module.state)">
                {{ module.state }}
              </span>
            </div>
            <p class="text-[#94a3b8] text-sm">{{ module.description }}</p>
          </div>
        </div>
        
        <div class="flex flex-wrap justify-end gap-2 max-w-4xl">
          <template v-if="module.state === 'available'">
            <button @click="install" :disabled="actionLoading !== ''" class="btn btn-primary text-sm">
              <i v-if="actionLoading === 'install'" class="feather icon-loader animate-spin mr-2"></i>
              <i v-else class="feather icon-download mr-2"></i>
              {{ actionLoading === 'install' ? t('moduleDetails.installing') : t('moduleDetails.install') }}
            </button>
          </template>

          <template v-else-if="isInstalled(module.state)">
            
            <template v-if="generalActions.length">
              <button v-for="act in generalActions" :key="act.id"
                @click="executeDynamicAction(act)" 
                :disabled="actionLoading !== ''" 
                :title="act.description || act.title"
                class="btn text-sm"
                :class="'btn-' + act.color">
                <i v-if="actionLoading === act.id" class="feather icon-loader animate-spin mr-2"></i>
                <i v-else :class="'feather icon-' + act.icon + ' mr-2'"></i>
                {{ act.title }}
              </button>
            </template>

            <button v-if="module.capabilities.supports_service && module.state !== 'enabled'" @click="enable" :disabled="actionLoading !== ''" class="btn btn-success text-sm">
              <i v-if="actionLoading === 'enable'" class="feather icon-loader animate-spin mr-2"></i>
              <i v-else class="feather icon-play mr-2"></i>
              {{ t('moduleDetails.start') }}
            </button>
            
            <button v-if="module.capabilities.supports_service && module.state === 'enabled'" @click="disable" :disabled="actionLoading !== ''" class="btn btn-secondary text-sm">
              <i v-if="actionLoading === 'disable'" class="feather icon-loader animate-spin mr-2"></i>
              <i v-else class="feather icon-square mr-2"></i>
              {{ t('moduleDetails.stop') }}
            </button>
            
            <button @click="uninstall" :disabled="actionLoading !== ''" class="btn btn-danger text-sm">
              <i v-if="actionLoading === 'uninstall'" class="feather icon-loader animate-spin mr-2"></i>
              <i v-else class="feather icon-trash mr-2"></i>
              {{ t('moduleDetails.remove') }}
            </button>
          </template>
        </div>
      </div>

      <div v-if="presetGroups.length" class="mb-6">
        <div class="flex items-center justify-between mb-3">
          <div>
            <h2 class="text-lg font-semibold text-white">{{ t('moduleDetails.presets') }}</h2>
            <p class="text-sm text-[#94a3b8]">{{ t('moduleDetails.presetsDescription') }}</p>
          </div>
        </div>
        <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3">
          <div v-for="preset in presetGroups" :key="preset.id" class="bg-[#1e293b] border border-[#334155] rounded-xl p-4">
            <div class="flex items-start gap-3 mb-4">
              <div class="rounded-lg bg-indigo-500/10 text-indigo-400 p-2">
                <i class="feather icon-shield"></i>
              </div>
              <div>
                <h3 class="font-medium text-white">{{ preset.title }}</h3>
                <p class="text-xs text-[#94a3b8] mt-1">{{ preset.description }}</p>
              </div>
            </div>
            <div class="grid grid-cols-2 gap-2">
              <button @click="executeDynamicAction(preset.enable)" :disabled="actionLoading !== ''" class="btn btn-success text-xs">
                {{ actionLoading === preset.enable.id ? t('moduleDetails.applying') : t('moduleDetails.enable') }}
              </button>
              <button @click="executeDynamicAction(preset.disable)" :disabled="actionLoading !== ''" class="btn btn-secondary text-xs">
                {{ actionLoading === preset.disable.id ? t('moduleDetails.applying') : t('moduleDetails.disable') }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Dynamic Tabs -->
      <div v-if="module.pages && module.pages.length > 0" class="border-b border-[#334155] mb-6 flex overflow-x-auto">
        <button v-for="page in module.pages" :key="page.id" @click="activeTab = page.id"
          class="px-4 py-2 border-b-2 font-medium text-sm whitespace-nowrap transition-colors"
          :class="activeTab === page.id ? 'border-indigo-500 text-indigo-400' : 'border-transparent text-[#64748b] hover:text-[#e2e8f0]'">
          {{ page.title }}
        </button>
      </div>

      <!-- Tab Content -->
      <div class="rounded-xl border border-[#334155] bg-[#1e293b] p-4 sm:p-6">
        
        <!-- Overview Tab -->
        <div v-if="currentPage && currentPage.type === 'overview'" class="space-y-6">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <h3 class="text-sm font-medium text-[#94a3b8] mb-1">{{ t('moduleDetails.category') }}</h3>
              <p class="text-[#e2e8f0]">{{ t(`categories.${module.category || 'System'}`) }}</p>
            </div>
            <div>
              <h3 class="text-sm font-medium text-[#94a3b8] mb-1">{{ t('moduleDetails.softwareVersion') }}</h3>
              <p class="break-words text-[#e2e8f0]">{{ module.software_version || t('common.notInstalled') }}</p>
            </div>
            <div v-if="module.installed_at">
              <h3 class="text-sm font-medium text-[#94a3b8] mb-1">{{ t('moduleDetails.installedAt') }}</h3>
              <p class="text-[#e2e8f0]">{{ new Date(module.installed_at).toLocaleString() }}</p>
            </div>
          </div>
          
          <!-- Dynamic Status -->
          <div class="mt-8 border-t border-[#334155] pt-6">
            <div class="flex items-center justify-between mb-4">
              <h3 class="text-lg font-semibold text-white">{{ t('moduleDetails.runtimeStatus') }}</h3>
              <button @click="loadStatus" class="text-xs text-indigo-400 hover:text-indigo-300">{{ t('common.refresh') }}</button>
            </div>
            
            <div v-if="statusLoading" class="text-sm text-[#94a3b8]">{{ t('moduleDetails.checkingStatus') }}</div>
            <div v-else-if="runtimeStatus" class="grid grid-cols-1 sm:grid-cols-3 gap-4">
               <div class="bg-black/20 p-4 rounded-lg">
                 <div class="text-xs text-[#64748b] mb-1">{{ t('moduleDetails.packageState') }}</div>
                 <div class="font-medium text-[#e2e8f0]">{{ runtimeStatus.packageStatus }}</div>
               </div>
               <div v-if="module.capabilities.supports_service" class="bg-black/20 p-4 rounded-lg">
                 <div class="text-xs text-[#64748b] mb-1">{{ t('moduleDetails.serviceState') }}</div>
                 <div class="font-medium" :class="runtimeStatus.serviceStatus === 'running' ? 'text-green-400' : 'text-red-400'">
                   {{ runtimeStatus.serviceStatus || 'unknown' }}
                 </div>
               </div>
               <div class="bg-black/20 p-4 rounded-lg">
                 <div class="text-xs text-[#64748b] mb-1">{{ t('moduleDetails.health') }}</div>
                 <div class="font-medium text-[#e2e8f0]">{{ runtimeStatus.health || 'unknown' }}</div>
               </div>
               <div v-if="runtimeStatus.details" class="bg-black/20 p-4 rounded-lg col-span-full">
                 <div class="text-xs text-[#64748b] mb-1">{{ t('moduleDetails.details') }}</div>
                 <pre class="text-xs text-[#e2e8f0] whitespace-pre-wrap">{{ runtimeStatus.details }}</pre>
               </div>
            </div>
            
            <!-- Dynamic Properties Cards -->
            <div v-if="runtimeStatus && runtimeStatus.properties && runtimeStatus.properties.length > 0" class="mt-6">
              <!-- Group properties by group -->
              <div v-for="group in [...new Set(runtimeStatus.properties.map(p => p.group || 'General'))]" :key="group" class="mb-6">
                <h3 class="text-sm font-medium text-[#94a3b8] mb-3 uppercase tracking-wider">{{ group }}</h3>
                <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                  <div v-for="prop in runtimeStatus.properties.filter(p => (p.group || 'General') === group)" :key="prop.name" class="bg-[#0f172a] border border-[#334155] p-4 rounded-xl flex flex-col justify-center">
                    <div class="text-xs text-[#64748b] mb-1">{{ prop.name }}</div>
                    <div class="font-medium text-[#e2e8f0] truncate" :title="prop.value">{{ prop.value }}</div>
                  </div>
                </div>
              </div>
            </div>
            
            <div v-else-if="!statusLoading && !runtimeStatus" class="text-sm text-[#94a3b8]">{{ t('moduleDetails.statusUnavailable') }}</div>
          </div>
        </div>

        <!-- Dependencies Tab -->
        <div v-else-if="currentPage && currentPage.type === 'dependencies'" class="space-y-6">
          <div v-if="!module.dependencies || (!module.dependencies.required?.length && !module.dependencies.recommended?.length)">
            <p class="text-[#94a3b8]">{{ t('moduleDetails.noDependencies') }}</p>
          </div>
          <div v-else class="space-y-4">
            <div v-if="module.dependencies.required?.length">
              <h3 class="text-sm font-medium text-white mb-2">{{ t('moduleDetails.required') }}</h3>
              <ul class="list-disc list-inside text-sm text-[#94a3b8]">
                <li v-for="dep in module.dependencies.required" :key="dep">{{ dep }}</li>
              </ul>
            </div>
            <div v-if="module.dependencies.recommended?.length">
              <h3 class="text-sm font-medium text-white mb-2">{{ t('moduleDetails.recommended') }}</h3>
              <ul class="list-disc list-inside text-sm text-[#94a3b8]">
                <li v-for="dep in module.dependencies.recommended" :key="dep">{{ dep }}</li>
              </ul>
            </div>
            <div v-if="module.dependencies.conflicts?.length">
              <h3 class="text-sm font-medium text-red-400 mb-2">{{ t('moduleDetails.conflicts') }}</h3>
              <ul class="list-disc list-inside text-sm text-[#94a3b8]">
                <li v-for="dep in module.dependencies.conflicts" :key="dep">{{ dep }}</li>
              </ul>
            </div>
          </div>
        </div>

        <!-- Settings Tab -->
        <div v-else-if="currentPage && currentPage.type === 'settings'" class="space-y-6">
          <SettingsForm :moduleId="module.id" :schema="module.settings_schema" />
        </div>

        <!-- Logs Tab -->
        <div v-else-if="currentPage && currentPage.type === 'logs'" class="space-y-6">
          <LogsViewer :moduleId="module.id" :logs="module.logs" />
        </div>

        <!-- DataGrid Tab -->
        <div v-else-if="currentPage && currentPage.type === 'datagrid'" class="space-y-6">
          <DataGrid :module="module" :page="currentPage" />
        </div>
        
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import DataGrid from '@/components/DataGrid.vue'
import SettingsForm from '@/components/SettingsForm.vue'
import LogsViewer from '@/components/LogsViewer.vue'
import ModuleIcon from '@/components/ModuleIcon.vue'
import api, { apiErrorMessage } from '@/api/client'
import { useConfirmStore } from '@/stores/confirm'

const confirm = useConfirmStore()
const { t } = useI18n()

const route = useRoute()
const router = useRouter()
const id = route.params.id

const module = ref(null)
const loading = ref(true)
const statusLoading = ref(false)
const errorMessage = ref('')
const actionLoading = ref('')
const runtimeStatus = ref(null)

const activeTab = ref('overview')
const selectedLog = ref('')

const currentPage = computed(() => {
  if (!module.value || !module.value.pages) return null
  return module.value.pages.find(p => p.id === activeTab.value) || module.value.pages[0]
})

const generalActions = computed(() => {
  return (module.value?.actions || []).filter(action => !action.id.includes('_preset_'))
})

const presetGroups = computed(() => {
  const actions = module.value?.actions || []
  const labels = {
    sshd: 'SSH',
    nginx_scanners: 'Nginx Scanners',
    nginx_auth: 'Nginx HTTP Auth',
    php_probes: 'PHP Probes',
  }

  return Object.entries(labels).flatMap(([presetID, title]) => {
    const enable = actions.find(action => action.id === `enable_preset_${presetID}`)
    const disable = actions.find(action => action.id === `disable_preset_${presetID}`)
    if (!enable || !disable) return []
    return [{
      id: presetID,
      title,
      description: enable.description,
      enable,
      disable,
    }]
  })
})

onMounted(async () => {
  await loadModule()
  if (module.value && isInstalled(module.value.state)) {
    loadStatus()
    if (module.value.logs?.length) {
      selectedLog.value = module.value.logs[0].id
    }
  }
})

async function loadModule() {
  loading.value = true
  try {
    errorMessage.value = ''
    const { data } = await api.get(`/modules/${id}`)
    module.value = data
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, t('moduleDetails.loadError'))
  } finally {
    loading.value = false
  }
}

async function loadStatus() {
  statusLoading.value = true
  try {
    const { data } = await api.get(`/modules/${id}/status`)
    runtimeStatus.value = data
  } catch (e) {
    console.error('Failed to load status', e)
  } finally {
    statusLoading.value = false
  }
}

function isInstalled(state) {
  return ['installed', 'enabled', 'disabled', 'error'].includes(state)
}

function stateBadge(state) {
  const map = {
    enabled: 'badge-success', disabled: 'badge-muted',
    installing: 'badge-primary', error: 'badge-danger',
    available: 'badge-muted', installed: 'badge-warning',
    removing: 'badge-danger',
  }
  return map[state] || 'badge-muted'
}

async function executeDynamicAction(act) {
  if (act.requiresConfirmation || act.dangerous) {
    const confirmed = await confirm.require({
      title: act.title,
      message: t('moduleDetails.confirmAction', { action: act.title }),
      confirmText: act.title,
      type: act.dangerous ? 'danger' : 'warning'
    })
    if (!confirmed) return
  }
  
  actionLoading.value = act.id
  try {
    errorMessage.value = ''
    await api.post(`/modules/${id}/actions/${act.id}`)
    await loadModule()
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, t('moduleDetails.unableAction', { action: act.title }))
  } finally {
    actionLoading.value = ''
  }
}

async function action(endpoint, label) {
  if (['uninstall', 'disable', 'restart'].includes(endpoint)) {
    let msg = t('moduleDetails.confirmModuleAction', { action: endpoint, name: module.value.name })
    if (endpoint === 'uninstall' && module.value.dependencies?.required?.length) {
      msg += `\n\n${t('moduleDetails.dependencyWarning')}`
    }
    const confirmed = await confirm.require({
      title: `${endpoint.charAt(0).toUpperCase() + endpoint.slice(1)} Module`,
      message: msg,
      confirmText: endpoint.charAt(0).toUpperCase() + endpoint.slice(1),
      type: endpoint === 'uninstall' ? 'danger' : 'warning'
    })
    if (!confirmed) return
  }
  
  actionLoading.value = label
  try {
    errorMessage.value = ''
    await api.post(`/modules/${id}/${endpoint}`)
    await loadModule()
    if (isInstalled(module.value.state)) {
      await loadStatus()
    }
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, `Unable to ${endpoint} module`)
  } finally {
    actionLoading.value = ''
  }
}

const install   = () => action('install', 'install')
const uninstall = () => action('uninstall', 'uninstall')
const enable    = () => action('enable', 'enable')
const disable   = () => action('disable', 'disable')
</script>
