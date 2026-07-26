<template>
  <div class="px-6 py-8 h-full overflow-y-auto custom-scrollbar">
    <div v-if="loading" class="flex justify-center py-20">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-500"></div>
    </div>
    
    <div v-else-if="errorMessage" class="bg-red-500/10 border border-red-500/20 text-red-400 p-4 rounded-xl text-sm">
      {{ errorMessage }}
    </div>

    <div v-else-if="module">
      <!-- Header -->
      <div class="flex items-start justify-between mb-8">
        <div class="flex items-center gap-4">
          <div class="w-16 h-16 rounded-2xl bg-black/20 flex items-center justify-center border border-[#334155]">
            <i class="feather text-2xl text-indigo-400" :class="'icon-' + module.icon"></i>
          </div>
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
        
        <div class="flex gap-2">
          <template v-if="module.state === 'available'">
            <button @click="install" :disabled="actionLoading !== ''" class="btn btn-primary text-sm">
              <i v-if="actionLoading === 'install'" class="feather icon-loader animate-spin mr-2"></i>
              <i v-else class="feather icon-download mr-2"></i>
              {{ actionLoading === 'install' ? 'Installing…' : 'Install' }}
            </button>
          </template>

          <template v-else-if="isInstalled(module.state)">
            
            <template v-if="module.actions && module.actions.length">
              <button v-for="act in module.actions" :key="act.id" 
                @click="executeDynamicAction(act)" 
                :disabled="actionLoading !== ''" 
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
              Start
            </button>
            
            <button v-if="module.capabilities.supports_service && module.state === 'enabled'" @click="disable" :disabled="actionLoading !== ''" class="btn btn-secondary text-sm">
              <i v-if="actionLoading === 'disable'" class="feather icon-loader animate-spin mr-2"></i>
              <i v-else class="feather icon-square mr-2"></i>
              Stop
            </button>
            
            <button @click="uninstall" :disabled="actionLoading !== ''" class="btn btn-danger text-sm">
              <i v-if="actionLoading === 'uninstall'" class="feather icon-loader animate-spin mr-2"></i>
              <i v-else class="feather icon-trash mr-2"></i>
              Remove
            </button>
          </template>
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
      <div class="bg-[#1e293b] border border-[#334155] rounded-xl p-6">
        
        <!-- Overview Tab -->
        <div v-if="currentPage && currentPage.type === 'overview'" class="space-y-6">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <h3 class="text-sm font-medium text-[#94a3b8] mb-1">Category</h3>
              <p class="text-[#e2e8f0]">{{ module.category || 'System' }}</p>
            </div>
            <div>
              <h3 class="text-sm font-medium text-[#94a3b8] mb-1">Software Version</h3>
              <p class="text-[#e2e8f0]">{{ module.software_version || 'Not Installed' }}</p>
            </div>
            <div v-if="module.installed_at">
              <h3 class="text-sm font-medium text-[#94a3b8] mb-1">Installed At</h3>
              <p class="text-[#e2e8f0]">{{ new Date(module.installed_at).toLocaleString() }}</p>
            </div>
          </div>
          
          <!-- Dynamic Status -->
          <div class="mt-8 border-t border-[#334155] pt-6">
            <div class="flex items-center justify-between mb-4">
              <h3 class="text-lg font-semibold text-white">Runtime Status</h3>
              <button @click="loadStatus" class="text-xs text-indigo-400 hover:text-indigo-300">Refresh</button>
            </div>
            
            <div v-if="statusLoading" class="text-sm text-[#94a3b8]">Checking system status...</div>
            <div v-else-if="runtimeStatus" class="grid grid-cols-1 sm:grid-cols-3 gap-4">
               <div class="bg-black/20 p-4 rounded-lg">
                 <div class="text-xs text-[#64748b] mb-1">Package State</div>
                 <div class="font-medium text-[#e2e8f0]">{{ runtimeStatus.packageStatus }}</div>
               </div>
               <div v-if="module.capabilities.supports_service" class="bg-black/20 p-4 rounded-lg">
                 <div class="text-xs text-[#64748b] mb-1">Service State</div>
                 <div class="font-medium" :class="runtimeStatus.serviceStatus === 'running' ? 'text-green-400' : 'text-red-400'">
                   {{ runtimeStatus.serviceStatus || 'unknown' }}
                 </div>
               </div>
               <div class="bg-black/20 p-4 rounded-lg">
                 <div class="text-xs text-[#64748b] mb-1">Health</div>
                 <div class="font-medium text-[#e2e8f0]">{{ runtimeStatus.health || 'unknown' }}</div>
               </div>
               <div v-if="runtimeStatus.details" class="bg-black/20 p-4 rounded-lg col-span-full">
                 <div class="text-xs text-[#64748b] mb-1">Details</div>
                 <pre class="text-xs text-[#e2e8f0] whitespace-pre-wrap">{{ runtimeStatus.details }}</pre>
               </div>
            </div>
            <div v-else class="text-sm text-[#94a3b8]">Status not available.</div>
          </div>
        </div>

        <!-- Dependencies Tab -->
        <div v-else-if="currentPage && currentPage.type === 'dependencies'" class="space-y-6">
          <div v-if="!module.dependencies || (!module.dependencies.required?.length && !module.dependencies.recommended?.length)">
            <p class="text-[#94a3b8]">No specific dependencies defined.</p>
          </div>
          <div v-else class="space-y-4">
            <div v-if="module.dependencies.required?.length">
              <h3 class="text-sm font-medium text-white mb-2">Required</h3>
              <ul class="list-disc list-inside text-sm text-[#94a3b8]">
                <li v-for="dep in module.dependencies.required" :key="dep">{{ dep }}</li>
              </ul>
            </div>
            <div v-if="module.dependencies.recommended?.length">
              <h3 class="text-sm font-medium text-white mb-2">Recommended</h3>
              <ul class="list-disc list-inside text-sm text-[#94a3b8]">
                <li v-for="dep in module.dependencies.recommended" :key="dep">{{ dep }}</li>
              </ul>
            </div>
            <div v-if="module.dependencies.conflicts?.length">
              <h3 class="text-sm font-medium text-red-400 mb-2">Conflicts</h3>
              <ul class="list-disc list-inside text-sm text-[#94a3b8]">
                <li v-for="dep in module.dependencies.conflicts" :key="dep">{{ dep }}</li>
              </ul>
            </div>
          </div>
        </div>

        <!-- Settings Tab -->
        <div v-else-if="currentPage && currentPage.type === 'settings'" class="space-y-6">
          <div v-if="module.settings_schema && module.settings_schema.length">
            <div v-for="setting in module.settings_schema" :key="setting.id" class="mb-4">
              <label class="block text-sm font-medium text-[#94a3b8] mb-1">{{ setting.label }}</label>
              
              <select v-if="setting.type === 'select'" class="input w-full md:w-1/2">
                <option v-for="opt in setting.options" :key="opt" :value="opt" :selected="opt === setting.value">{{ opt }}</option>
              </select>
              
              <input v-else-if="setting.type === 'boolean'" type="checkbox" :checked="setting.value" class="mt-2" />
              
              <input v-else type="text" :value="setting.value" class="input w-full md:w-1/2" />
              
              <p class="text-xs text-[#64748b] mt-1">{{ setting.description }}</p>
            </div>
            <button class="btn btn-primary mt-4">Save Settings</button>
          </div>
          <div v-else class="py-10 text-center text-[#64748b]">
            <p>Settings schema is empty.</p>
          </div>
        </div>

        <!-- Logs Tab -->
        <div v-else-if="currentPage && currentPage.type === 'logs'" class="space-y-6">
          <div v-if="module.logs && module.logs.length">
            <div class="mb-4">
              <label class="block text-sm font-medium text-[#94a3b8] mb-2">Select Log Source</label>
              <select class="input w-full md:w-1/3" v-model="selectedLog">
                <option v-for="log in module.logs" :key="log.id" :value="log.id">{{ log.name }} ({{ log.type }})</option>
              </select>
            </div>
            <div class="bg-black/50 border border-[#334155] rounded-xl p-4 font-mono text-xs text-[#e2e8f0] h-96 overflow-y-auto custom-scrollbar whitespace-pre-wrap">
              This is a placeholder for real-time logs tailing... 
              <br><br>
              <span class="text-indigo-400">Selected Log: {{ selectedLog }}</span>
            </div>
          </div>
          <div v-else class="py-10 text-center text-[#64748b]">
            <p>No logs defined for this module.</p>
          </div>
        </div>

        <!-- DataGrid Placeholder Tab -->
        <div v-else-if="currentPage && currentPage.type === 'datagrid'" class="space-y-6">
          <div class="py-10 text-center text-[#64748b]">
            <i class="feather icon-table text-4xl mb-4 text-[#475569]"></i>
            <h3 class="text-lg font-medium text-white mb-2">{{ currentPage.title }}</h3>
            <p>This dynamic grid is currently under development.</p>
            <p class="text-xs mt-2 text-[#94a3b8]">(Will load metadata for {{ module.id }}/{{ currentPage.id }})</p>
          </div>
        </div>
        
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import api, { apiErrorMessage } from '@/api/client'

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
    errorMessage.value = apiErrorMessage(e, 'Unable to load module details')
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
    const msg = `Are you sure you want to ${act.title}?`
    if (!confirm(msg)) return
  }
  
  actionLoading.value = act.id
  try {
    errorMessage.value = ''
    await api.post(`/modules/${id}/actions/${act.id}`)
    await loadModule()
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, `Unable to ${act.title}`)
  } finally {
    actionLoading.value = ''
  }
}

async function action(endpoint, label) {
  if (['uninstall', 'disable', 'restart'].includes(endpoint)) {
    let msg = `${endpoint[0].toUpperCase() + endpoint.slice(1)} module ${module.value.name}?`
    if (endpoint === 'uninstall' && module.value.dependencies?.required?.length) {
      msg += `\n\nWarning: This may affect dependencies.`
    }
    if (!confirm(msg)) return
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
