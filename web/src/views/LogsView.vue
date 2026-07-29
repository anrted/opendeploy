<template>
  <div class="h-[calc(100vh-4rem)] flex flex-col p-4">
    <!-- Header -->
    <div class="mb-4">
      <h1 class="text-2xl font-bold text-text-main">{{ $t('sidebar.logs') }}</h1>
      <p class="text-text-muted mt-1">Search and view system and application logs</p>
    </div>

    <!-- Filters -->
    <div class="bg-bg-card border border-border-subtle rounded-xl p-4 shadow-sm mb-4">
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <!-- Search Query -->
        <div>
          <label class="block text-xs font-medium text-text-muted mb-1">Search Message</label>
          <div class="relative">
            <svg class="w-4 h-4 absolute left-3 top-2.5 text-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>
            <input v-model="filters.query" type="text" placeholder="Search..."
                   class="pl-9 w-full bg-bg-base border border-border-subtle rounded-lg px-3 py-2 text-sm text-text-main focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-colors"
                   @keyup.enter="applyFilters">
          </div>
        </div>

        <!-- Level -->
        <div>
          <label class="block text-xs font-medium text-text-muted mb-1">Level</label>
          <select v-model="filters.level" class="w-full bg-bg-base border border-border-subtle rounded-lg px-3 py-2 text-sm text-text-main focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-colors" @change="applyFilters">
            <option value="">All</option>
            <option value="INFO">INFO</option>
            <option value="WARN">WARN</option>
            <option value="ERROR">ERROR</option>
          </select>
        </div>

        <!-- Module -->
        <div>
          <label class="block text-xs font-medium text-text-muted mb-1">Module</label>
          <input v-model="filters.module" type="text" placeholder="E.g., fail2ban"
                 class="w-full bg-bg-base border border-border-subtle rounded-lg px-3 py-2 text-sm text-text-main focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-colors"
                 @keyup.enter="applyFilters">
        </div>

        <!-- Error ID -->
        <div>
          <label class="block text-xs font-medium text-text-muted mb-1">Error ID</label>
          <input v-model="filters.error_id" type="text" placeholder="Search by Error ID..."
                 class="w-full bg-bg-base border border-border-subtle rounded-lg px-3 py-2 text-sm text-text-main focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-colors"
                 @keyup.enter="applyFilters">
        </div>
      </div>
      
      <div class="mt-4 flex justify-end gap-2">
        <button @click="resetFilters" class="px-4 py-2 text-sm font-medium text-text-muted bg-bg-base border border-border-subtle rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-bg-base focus:ring-indigo-500">
          Reset
        </button>
        <button @click="applyFilters" class="px-4 py-2 text-sm font-medium text-white bg-indigo-600 rounded-lg hover:bg-indigo-700 transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-bg-base focus:ring-indigo-500 shadow-sm shadow-indigo-500/30">
          Search
        </button>
      </div>
    </div>

    <!-- Data Table -->
    <div class="flex-1 bg-bg-card border border-border-subtle rounded-xl shadow-sm overflow-hidden flex flex-col min-h-0 relative">
      <div v-if="loading && logs.length === 0" class="absolute inset-0 z-10 flex items-center justify-center bg-bg-card/50 backdrop-blur-sm">
        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-500"></div>
      </div>

      <div class="flex-1 overflow-auto">
        <table class="w-full text-left text-sm whitespace-nowrap">
          <thead class="sticky top-0 bg-bg-card border-b border-border-subtle text-xs uppercase text-text-muted z-10">
            <tr>
              <th class="px-4 py-3 font-medium">Time</th>
              <th class="px-4 py-3 font-medium">Level</th>
              <th class="px-4 py-3 font-medium">Module / Comp</th>
              <th class="px-4 py-3 font-medium w-full">Message</th>
              <th class="px-4 py-3 font-medium">Error ID</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border-subtle text-text-main">
            <tr v-for="log in logs" :key="log.id" class="hover:bg-slate-50 dark:hover:bg-slate-800/50 cursor-pointer transition-colors" @click="viewLogDetails(log)">
              <td class="px-4 py-3 text-text-muted">
                {{ formatTime(log.timestamp) }}
              </td>
              <td class="px-4 py-3">
                <span :class="[
                  'px-2 py-0.5 rounded text-xs font-medium',
                  log.level === 'ERROR' ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400' :
                  log.level === 'WARN'  ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' :
                  'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
                ]">
                  {{ log.level }}
                </span>
              </td>
              <td class="px-4 py-3 text-text-muted">
                {{ log.module || log.component || '-' }}
              </td>
              <td class="px-4 py-3 truncate max-w-[200px] lg:max-w-[400px]">
                {{ log.message }}
              </td>
              <td class="px-4 py-3 font-mono text-xs text-text-muted">
                {{ log.error_id || '-' }}
              </td>
            </tr>
            <tr v-if="!loading && logs.length === 0">
              <td colspan="5" class="px-4 py-8 text-center text-text-muted">
                No logs found.
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Pagination -->
      <div class="bg-bg-base border-t border-border-subtle px-4 py-3 flex items-center justify-between mt-auto shrink-0">
        <div class="text-sm text-text-muted">
          Showing <span class="font-medium text-text-main">{{ logs.length > 0 ? (currentPage - 1) * pageSize + 1 : 0 }}</span> to <span class="font-medium text-text-main">{{ Math.min(currentPage * pageSize, totalItems) }}</span> of <span class="font-medium text-text-main">{{ totalItems }}</span> logs
        </div>
        <div class="flex gap-2">
          <button @click="prevPage" :disabled="currentPage === 1"
                  class="px-3 py-1 bg-bg-card border border-border-subtle rounded hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-50 transition-colors text-sm font-medium text-text-main focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-bg-base focus:ring-indigo-500">
            Previous
          </button>
          <button @click="nextPage" :disabled="currentPage * pageSize >= totalItems"
                  class="px-3 py-1 bg-bg-card border border-border-subtle rounded hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-50 transition-colors text-sm font-medium text-text-main focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-bg-base focus:ring-indigo-500">
            Next
          </button>
        </div>
      </div>
    </div>

    <!-- Log Details Modal -->
    <div v-if="selectedLog" class="fixed inset-0 z-50 overflow-hidden flex" aria-labelledby="slide-over-title" role="dialog" aria-modal="true">
      <div class="absolute inset-0 bg-black/60 transition-opacity" @click="selectedLog = null"></div>
      
      <div class="absolute inset-y-0 right-0 max-w-full flex w-full sm:w-[600px] xl:w-[800px]">
        <div class="h-full w-full bg-bg-card shadow-2xl flex flex-col border-l border-border-subtle transform transition-transform">
          <!-- Modal Header -->
          <div class="px-6 py-4 border-b border-border-subtle flex items-center justify-between bg-bg-base shrink-0">
            <h2 class="text-lg font-bold text-text-main" id="slide-over-title">Log Details</h2>
            <button @click="selectedLog = null" class="text-text-muted hover:text-text-main transition-colors rounded-lg p-1 hover:bg-slate-100 dark:hover:bg-slate-800">
              <span class="sr-only">Close panel</span>
              <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          
          <!-- Modal Body -->
          <div class="flex-1 overflow-y-auto p-6">
            <div class="space-y-6">
              <!-- Basic Info -->
              <div>
                <h3 class="text-sm font-semibold text-text-muted uppercase tracking-wider mb-3">General Information</h3>
                <div class="bg-bg-base rounded-xl border border-border-subtle overflow-hidden">
                  <dl class="divide-y divide-border-subtle text-sm">
                    <div class="px-4 py-3 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                      <dt class="font-medium text-text-muted">Timestamp</dt>
                      <dd class="mt-1 text-text-main sm:mt-0 sm:col-span-2">{{ formatDateTime(selectedLog.timestamp) }}</dd>
                    </div>
                    <div class="px-4 py-3 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                      <dt class="font-medium text-text-muted">Level</dt>
                      <dd class="mt-1 sm:mt-0 sm:col-span-2">
                        <span :class="[
                          'px-2 py-0.5 rounded text-xs font-medium',
                          selectedLog.level === 'ERROR' ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400' :
                          selectedLog.level === 'WARN'  ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' :
                          'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
                        ]">
                          {{ selectedLog.level }}
                        </span>
                      </dd>
                    </div>
                    <div class="px-4 py-3 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                      <dt class="font-medium text-text-muted">Message</dt>
                      <dd class="mt-1 text-text-main font-medium sm:mt-0 sm:col-span-2">{{ selectedLog.message }}</dd>
                    </div>
                    <div v-if="selectedLog.error_id" class="px-4 py-3 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                      <dt class="font-medium text-text-muted">Error ID</dt>
                      <dd class="mt-1 text-text-main font-mono text-sm sm:mt-0 sm:col-span-2">{{ selectedLog.error_id }}</dd>
                    </div>
                    <div v-if="selectedLog.request_id" class="px-4 py-3 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                      <dt class="font-medium text-text-muted">Request ID</dt>
                      <dd class="mt-1 text-text-main font-mono text-sm sm:mt-0 sm:col-span-2">{{ selectedLog.request_id }}</dd>
                    </div>
                  </dl>
                </div>
              </div>
              
              <!-- Context -->
              <div v-if="selectedLog.module || selectedLog.component || selectedLog.method || selectedLog.endpoint || selectedLog.ip">
                <h3 class="text-sm font-semibold text-text-muted uppercase tracking-wider mb-3">Context</h3>
                <div class="bg-bg-base rounded-xl border border-border-subtle overflow-hidden">
                  <dl class="divide-y divide-border-subtle text-sm">
                    <div v-if="selectedLog.module" class="px-4 py-3 flex justify-between">
                      <dt class="font-medium text-text-muted">Module</dt>
                      <dd class="text-text-main font-mono">{{ selectedLog.module }}</dd>
                    </div>
                    <div v-if="selectedLog.component" class="px-4 py-3 flex justify-between">
                      <dt class="font-medium text-text-muted">Component</dt>
                      <dd class="text-text-main font-mono">{{ selectedLog.component }}</dd>
                    </div>
                    <div v-if="selectedLog.method || selectedLog.endpoint" class="px-4 py-3 flex justify-between">
                      <dt class="font-medium text-text-muted">HTTP Request</dt>
                      <dd class="text-text-main font-mono text-right">
                        <span v-if="selectedLog.method" class="text-indigo-500 font-bold mr-2">{{ selectedLog.method }}</span>
                        {{ selectedLog.endpoint }}
                      </dd>
                    </div>
                    <div v-if="selectedLog.ip" class="px-4 py-3 flex justify-between">
                      <dt class="font-medium text-text-muted">Client IP</dt>
                      <dd class="text-text-main font-mono">{{ selectedLog.ip }}</dd>
                    </div>
                    <div v-if="selectedLog.duration_ms > 0" class="px-4 py-3 flex justify-between">
                      <dt class="font-medium text-text-muted">Duration</dt>
                      <dd class="text-text-main font-mono">{{ selectedLog.duration_ms }} ms</dd>
                    </div>
                  </dl>
                </div>
              </div>

              <!-- Attributes (JSON) -->
              <div v-if="selectedLog.attributes && selectedLog.attributes !== '{}'">
                <h3 class="text-sm font-semibold text-text-muted uppercase tracking-wider mb-3">Attributes</h3>
                <div class="bg-[#1e1e1e] rounded-xl overflow-hidden border border-[#333]">
                  <pre class="p-4 text-xs font-mono text-[#d4d4d4] overflow-x-auto m-0 leading-relaxed">{{ formatAttributes(selectedLog.attributes) }}</pre>
                </div>
              </div>

              <!-- Stack Trace -->
              <div v-if="selectedLog.stack_trace">
                <h3 class="text-sm font-semibold text-red-500 uppercase tracking-wider mb-3">Stack Trace</h3>
                <div class="bg-[#1e1e1e] rounded-xl overflow-hidden border border-[#333]">
                  <pre class="p-4 text-xs font-mono text-[#d4d4d4] overflow-x-auto m-0 leading-relaxed">{{ selectedLog.stack_trace }}</pre>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script setup>
import { ref, onMounted, reactive } from 'vue'
import api from '@/api/client'

const logs = ref([])
const totalItems = ref(0)
const loading = ref(false)
const currentPage = ref(1)
const pageSize = 100

const filters = reactive({
  query: '',
  level: '',
  module: '',
  error_id: '',
})

const selectedLog = ref(null)

const fetchLogs = async () => {
  loading.value = true
  try {
    const params = new URLSearchParams({
      limit: pageSize.toString(),
      offset: ((currentPage.value - 1) * pageSize).toString(),
    })
    
    if (filters.query) params.append('query', filters.query)
    if (filters.level) params.append('level', filters.level)
    if (filters.module) params.append('module', filters.module)
    if (filters.error_id) params.append('error_id', filters.error_id)

    const response = await api.get(`/logs?${params.toString()}`)
    const data = response.data
    
    logs.value = data.data || []
    totalItems.value = data.total || 0
  } catch (error) {
    window.dispatchEvent(new CustomEvent('opendeploy:toast', {
      detail: { message: 'Failed to fetch logs' }
    }))
    console.error(error)
  } finally {
    loading.value = false
  }
}

const applyFilters = () => {
  currentPage.value = 1
  fetchLogs()
}

const resetFilters = () => {
  filters.query = ''
  filters.level = ''
  filters.module = ''
  filters.error_id = ''
  applyFilters()
}

const prevPage = () => {
  if (currentPage.value > 1) {
    currentPage.value--
    fetchLogs()
  }
}

const nextPage = () => {
  if (currentPage.value * pageSize < totalItems.value) {
    currentPage.value++
    fetchLogs()
  }
}

const viewLogDetails = (log) => {
  selectedLog.value = log
}

const formatTime = (ts) => {
  if (!ts) return ''
  const d = new Date(ts)
  return d.toLocaleTimeString([], { hour12: false })
}

const formatDateTime = (ts) => {
  if (!ts) return ''
  const d = new Date(ts)
  return d.toLocaleString()
}

const formatAttributes = (attrsStr) => {
  try {
    const obj = JSON.parse(attrsStr)
    return JSON.stringify(obj, null, 2)
  } catch (e) {
    return attrsStr
  }
}

onMounted(() => {
  fetchLogs()
})
</script>
