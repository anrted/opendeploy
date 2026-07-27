<template>
  <div class="logs-viewer space-y-6">
    <div v-if="logs && logs.length > 0">
      <div class="flex flex-col md:flex-row gap-4 mb-4 items-center justify-between">
        <!-- Log Selector -->
        <div class="flex items-center space-x-3 w-full md:w-auto">
          <label class="text-sm font-medium text-[#94a3b8]">Select Log:</label>
          <select v-model="selectedLogId" class="input w-full md:w-64" @change="fetchLogs">
            <option v-for="log in logs" :key="log.id" :value="log.id">
              {{ log.name }}
            </option>
          </select>
        </div>

        <!-- Controls -->
        <div class="flex items-center space-x-3">
          <select v-model.number="lineLimit" class="input w-28" @change="fetchLogs()">
            <option :value="100">100 lines</option><option :value="500">500 lines</option><option :value="1000">1,000 lines</option><option :value="5000">5,000 lines</option>
          </select>
          <select v-model="levelFilter" class="input w-28">
            <option value="">All levels</option><option value="error">Error</option><option value="warn">Warning</option><option value="info">Info</option><option value="debug">Debug</option>
          </select>
          <div class="relative">
            <input 
              type="text" 
              v-model="searchQuery" 
              placeholder="Search in logs..." 
              class="input pl-9 w-full md:w-64"
            />
            <i class="feather icon-search absolute left-3 top-2.5 text-[#64748b]"></i>
          </div>
          <button @click="autoRefresh = !autoRefresh" :class="['btn', autoRefresh ? 'btn-primary' : 'bg-[#1e293b] border border-[#334155] text-[#94a3b8]']" title="Auto-refresh">
            <i class="feather icon-refresh-cw" :class="{'animate-spin-slow': autoRefresh}"></i>
          </button>
          <button @click="clearLog" class="btn btn-danger" title="Clear Log" :disabled="clearing">
            <i v-if="clearing" class="feather icon-loader animate-spin"></i>
            <i v-else class="feather icon-trash-2"></i>
          </button>
          <button @click="downloadLog" class="btn bg-[#1e293b] border border-[#334155] text-[#94a3b8] hover:text-white" title="Download Log">
            <i class="feather icon-download"></i>
          </button>
        </div>
      </div>
      <div v-if="errorMessage" class="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{{ errorMessage }}</div>
      <div v-if="successMessage" class="mb-4 rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-3 text-sm text-emerald-300">{{ successMessage }}</div>

      <!-- Log Terminal -->
      <div class="relative bg-[#0f172a] rounded-xl border border-[#334155] overflow-hidden">
        <div v-if="loading && !lines.length" class="absolute inset-0 z-10 flex items-center justify-center bg-[#0f172a]/80 backdrop-blur-sm">
          <div class="flex flex-col items-center">
            <i class="feather icon-loader animate-spin text-3xl text-indigo-500 mb-2"></i>
            <span class="text-[#94a3b8] text-sm">Loading logs...</span>
          </div>
        </div>
        
        <div 
          ref="logContainer" 
          class="p-4 h-[600px] overflow-y-auto font-mono text-xs sm:text-sm text-[#e2e8f0] leading-relaxed"
          @scroll="handleScroll"
        >
          <div v-if="filteredLines.length === 0" class="text-[#64748b] text-center py-10">
            {{ searchQuery ? 'No lines match your search.' : 'Log is empty.' }}
          </div>
          <div v-else>
            <div v-for="(line, idx) in filteredLines" :key="idx" :class="['hover:bg-white/5 px-2 py-0.5 rounded break-all whitespace-pre-wrap', lineClass(line)]">
              {{ line }}
            </div>
          </div>
        </div>

        <!-- Scroll to bottom button -->
        <button 
          v-show="!isAtBottom" 
          @click="scrollToBottom" 
          class="absolute bottom-4 right-6 bg-[#3b82f6]/20 text-[#3b82f6] hover:bg-[#3b82f6] hover:text-white border border-[#3b82f6]/50 rounded-full w-10 h-10 flex items-center justify-center transition-all shadow-lg"
        >
          <i class="feather icon-arrow-down"></i>
        </button>
      </div>
    </div>
    
    <div v-else class="py-10 text-center text-[#64748b]">
      <i class="feather icon-file-text text-4xl mb-4 text-[#475569]"></i>
      <h3 class="text-lg font-medium text-white mb-2">No Logs Available</h3>
      <p>This module does not expose any logs.</p>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import api, { apiErrorMessage } from '@/api/client'
import { useConfirmStore } from '@/stores/confirm'

const confirm = useConfirmStore()

const props = defineProps({
  moduleId: {
    type: String,
    required: true
  },
  logs: {
    type: Array,
    default: () => []
  }
})

const selectedLogId = ref('')
const lines = ref([])
const loading = ref(false)
const clearing = ref(false)
const searchQuery = ref('')
const autoRefresh = ref(true)
const lineLimit = ref(500)
const levelFilter = ref('')
const errorMessage = ref('')
const successMessage = ref('')
const logContainer = ref(null)
const isAtBottom = ref(true)
let refreshInterval = null

const filteredLines = computed(() => {
  let result = lines.value
  if (levelFilter.value) {
    const level = levelFilter.value.toLowerCase()
    result = result.filter(line => line.toLowerCase().includes(level))
  }
  if (!searchQuery.value) return result
  const q = searchQuery.value.toLowerCase()
  return result.filter(line => line.toLowerCase().includes(q))
})

onMounted(() => {
  if (props.logs && props.logs.length > 0) {
    selectedLogId.value = props.logs[0].id
    fetchLogs()
  }
  
  refreshInterval = setInterval(() => {
    if (autoRefresh.value && !loading.value) {
      fetchLogs(true)
    }
  }, 3000)
})

onUnmounted(() => {
  if (refreshInterval) clearInterval(refreshInterval)
})

watch(() => props.logs, (newLogs) => {
  if (newLogs && newLogs.length > 0 && !selectedLogId.value) {
    selectedLogId.value = newLogs[0].id
    fetchLogs()
  }
})

async function fetchLogs(isBackground = false) {
  if (!selectedLogId.value) return
  
  if (!isBackground) loading.value = true
  
  try {
    errorMessage.value = ''
    const res = await api.get(`/modules/${props.moduleId}/logs/${selectedLogId.value}/read?lines=${lineLimit.value}`)
    lines.value = res.data || []
    
    // If we are at the bottom, stay at the bottom after new lines arrive
    if (isAtBottom.value) {
      nextTick(() => {
        scrollToBottom()
      })
    }
  } catch (err) {
    errorMessage.value = apiErrorMessage(err, 'Failed to fetch logs')
  } finally {
    if (!isBackground) loading.value = false
  }
}

async function clearLog() {
  if (!selectedLogId.value) return
  const confirmed = await confirm.require({
    title: 'Clear Log',
    message: 'Are you sure you want to clear this log? This action cannot be undone.',
    confirmText: 'Clear',
    type: 'danger'
  })
  if (!confirmed) return
  
  clearing.value = true
  try {
    await api.post(`/modules/${props.moduleId}/logs/${selectedLogId.value}/clear`)
    lines.value = []
    successMessage.value = 'Log cleared.'
  } catch (err) {
    errorMessage.value = apiErrorMessage(err, 'Failed to clear log')
  } finally {
    clearing.value = false
  }
}

function lineClass(line) {
  const value = line.toLowerCase()
  if (value.includes('error') || value.includes('fatal') || value.includes('panic')) return 'text-red-400'
  if (value.includes('warn')) return 'text-amber-400'
  if (value.includes('debug') || value.includes('trace')) return 'text-slate-500'
  if (value.includes('info')) return 'text-emerald-300'
  return 'text-[#e2e8f0]'
}

function downloadLog() {
  const text = lines.value.join('\n')
  const blob = new Blob([text], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${props.moduleId}_${selectedLogId.value}.log`
  a.click()
  URL.revokeObjectURL(url)
}

function handleScroll() {
  if (!logContainer.value) return
  const { scrollTop, scrollHeight, clientHeight } = logContainer.value
  // If we are within 50px of the bottom, consider it "at bottom"
  isAtBottom.value = scrollHeight - scrollTop - clientHeight < 50
}

function scrollToBottom() {
  if (!logContainer.value) return
  logContainer.value.scrollTop = logContainer.value.scrollHeight
  isAtBottom.value = true
}
</script>

<style scoped>
.animate-spin-slow {
  animation: spin 3s linear infinite;
}
</style>
