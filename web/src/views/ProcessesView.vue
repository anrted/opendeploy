<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-text-main">Process Manager</h1>
        <p class="text-sm text-text-muted mt-1">Monitor and manage system processes</p>
      </div>
      <div class="flex gap-2">
        <button @click="fetchProcesses" class="px-4 py-2 bg-bg-card border border-border-subtle rounded-lg text-sm font-medium hover:bg-slate-50 dark:hover:bg-[#1e2535] transition-colors flex items-center gap-2">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
          Refresh
        </button>
      </div>
    </div>

    <!-- Search & Filter -->
    <div class="flex gap-4">
      <div class="flex-1 relative">
        <svg class="w-5 h-5 absolute left-3 top-2.5 text-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>
        <input type="text" v-model="searchQuery" placeholder="Search processes by name, user, or cmd..." class="w-full pl-10 pr-4 py-2 bg-bg-card border border-border-subtle rounded-lg text-sm focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-colors">
      </div>
    </div>

    <!-- Process Table -->
    <div class="bg-bg-card rounded-xl border border-border-subtle overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead class="bg-slate-50 dark:bg-[#1e2535] border-b border-border-subtle text-text-muted">
            <tr>
              <th class="px-6 py-3 font-medium cursor-pointer hover:text-text-main" @click="sortBy('pid')">
                PID <span v-if="sortKey === 'pid'">{{ sortAsc ? '↑' : '↓' }}</span>
              </th>
              <th class="px-6 py-3 font-medium cursor-pointer hover:text-text-main" @click="sortBy('user')">
                USER <span v-if="sortKey === 'user'">{{ sortAsc ? '↑' : '↓' }}</span>
              </th>
              <th class="px-6 py-3 font-medium cursor-pointer hover:text-text-main" @click="sortBy('cpu_percent')">
                CPU % <span v-if="sortKey === 'cpu_percent'">{{ sortAsc ? '↑' : '↓' }}</span>
              </th>
              <th class="px-6 py-3 font-medium cursor-pointer hover:text-text-main" @click="sortBy('mem_percent')">
                MEM % <span v-if="sortKey === 'mem_percent'">{{ sortAsc ? '↑' : '↓' }}</span>
              </th>
              <th class="px-6 py-3 font-medium cursor-pointer hover:text-text-main" @click="sortBy('mem_rss')">
                RSS <span v-if="sortKey === 'mem_rss'">{{ sortAsc ? '↑' : '↓' }}</span>
              </th>
              <th class="px-6 py-3 font-medium cursor-pointer hover:text-text-main" @click="sortBy('cmdline')">
                COMMAND <span v-if="sortKey === 'cmdline'">{{ sortAsc ? '↑' : '↓' }}</span>
              </th>
              <th class="px-6 py-3 font-medium text-right">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border-subtle">
            <tr v-if="loading" class="animate-pulse">
              <td colspan="7" class="px-6 py-4 text-center text-text-muted">Loading processes...</td>
            </tr>
            <tr v-else-if="filteredProcesses.length === 0">
              <td colspan="7" class="px-6 py-4 text-center text-text-muted">No processes found</td>
            </tr>
            <tr v-else v-for="proc in filteredProcesses" :key="proc.pid" class="hover:bg-slate-50 dark:hover:bg-[#1e2535]/50 transition-colors">
              <td class="px-6 py-3 font-medium text-text-main">{{ proc.pid }}</td>
              <td class="px-6 py-3 text-text-main">{{ proc.user }}</td>
              <td class="px-6 py-3 text-text-main">
                <div class="flex items-center gap-2">
                  <span :class="{'text-red-500': proc.cpu_percent > 80, 'text-yellow-500': proc.cpu_percent > 50 && proc.cpu_percent <= 80}">{{ proc.cpu_percent.toFixed(1) }}</span>
                  <div class="w-16 h-1.5 bg-slate-200 dark:bg-slate-700 rounded-full overflow-hidden">
                    <div class="h-full rounded-full transition-all duration-300" 
                         :class="proc.cpu_percent > 80 ? 'bg-red-500' : proc.cpu_percent > 50 ? 'bg-yellow-500' : 'bg-indigo-500'"
                         :style="{ width: Math.min(100, proc.cpu_percent) + '%' }"></div>
                  </div>
                </div>
              </td>
              <td class="px-6 py-3 text-text-main">{{ proc.mem_percent.toFixed(1) }}</td>
              <td class="px-6 py-3 text-text-muted">{{ formatBytes(proc.mem_rss) }}</td>
              <td class="px-6 py-3 text-text-muted font-mono text-xs truncate max-w-xs" :title="proc.cmdline">{{ proc.name }} {{ proc.cmdline }}</td>
              <td class="px-6 py-3 text-right">
                <button @click="killProcess(proc.pid, proc.name)" class="p-1.5 text-text-muted hover:text-red-500 rounded-md hover:bg-red-50 dark:hover:bg-red-500/10 transition-colors" title="Kill Process">
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { api } from '@/utils/api'

const processes = ref([])
const loading = ref(true)
const searchQuery = ref('')
const sortKey = ref('cpu_percent')
const sortAsc = ref(false)
let pollInterval = null

const fetchProcesses = async () => {
  try {
    const data = await api.get('/system/processes')
    processes.value = data || []
  } catch (error) {
    console.error('Failed to fetch processes:', error)
  } finally {
    loading.value = false
  }
}

const killProcess = async (pid, name) => {
  if (confirm(`Are you sure you want to kill process ${name} (PID: ${pid})?`)) {
    try {
      await api.post(`/system/processes/${pid}/kill`)
      // Refresh list immediately
      fetchProcesses()
    } catch (error) {
      alert(`Failed to kill process: ${error.message}`)
    }
  }
}

const sortBy = (key) => {
  if (sortKey.value === key) {
    sortAsc.value = !sortAsc.value
  } else {
    sortKey.value = key
    sortAsc.value = true
  }
}

const filteredProcesses = computed(() => {
  let result = processes.value
  
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    result = result.filter(p => 
      p.name?.toLowerCase().includes(q) || 
      p.user?.toLowerCase().includes(q) || 
      p.cmdline?.toLowerCase().includes(q) ||
      p.pid.toString().includes(q)
    )
  }
  
  result.sort((a, b) => {
    let aVal = a[sortKey.value]
    let bVal = b[sortKey.value]
    
    // Handle string comparisons
    if (typeof aVal === 'string') {
      aVal = aVal.toLowerCase()
      bVal = bVal.toLowerCase()
    }
    
    if (aVal < bVal) return sortAsc.value ? -1 : 1
    if (aVal > bVal) return sortAsc.value ? 1 : -1
    return 0
  })
  
  return result
})

const formatBytes = (bytes) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

onMounted(() => {
  fetchProcesses()
  // Poll every 5 seconds
  pollInterval = setInterval(fetchProcesses, 5000)
})

onUnmounted(() => {
  if (pollInterval) clearInterval(pollInterval)
})
</script>
