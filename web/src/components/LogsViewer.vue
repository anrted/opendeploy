<template>
  <div class="logs-viewer space-y-6">
    <div v-if="logs && logs.length > 0">
      <div class="flex flex-col md:flex-row gap-4 mb-4 items-center justify-between">
        <!-- Log Selector -->
        <div class="flex items-center space-x-3 w-full md:w-auto">
          <label class="text-sm font-medium text-[#94a3b8]">{{ t('logs.select') }}</label>
          <select v-model="selectedLogId" class="input w-full md:w-64" @change="fetchLogs">
            <option v-for="log in logs" :key="log.id" :value="log.id">
              {{ log.name }}
            </option>
          </select>
        </div>

        <!-- Controls -->
        <div class="flex items-center space-x-3">
          <select v-model.number="lineLimit" class="input w-28" @change="fetchLogs()">
            <option v-for="count in [100, 500, 1000, 5000]" :key="count" :value="count">{{ t('logs.lines', { count: count.toLocaleString() }) }}</option>
          </select>
          <select v-model="levelFilter" class="input w-28">
            <option value="">{{ t('logs.allLevels') }}</option><option value="error">{{ t('logs.levels.error') }}</option><option value="warn">{{ t('logs.levels.warning') }}</option><option value="info">{{ t('logs.levels.info') }}</option><option value="debug">{{ t('logs.levels.debug') }}</option>
          </select>
          <div class="relative">
            <input 
              type="text" 
              v-model="searchQuery" 
              :placeholder="t('logs.search')"
              class="input pl-9 w-full md:w-64"
            />
            <AppIcon name="search" class="absolute left-3 top-2.5 h-4 w-4 text-[#64748b]" />
          </div>
          <button @click="autoRefresh = !autoRefresh" :class="['btn', autoRefresh ? 'btn-primary' : 'bg-[#1e293b] border border-[#334155] text-[#94a3b8]']" :title="t('logs.autoRefresh')">
            <AppIcon name="refresh-cw" class="h-4 w-4" :class="{'animate-spin-slow': autoRefresh}" />
          </button>
          <button @click="clearLog" class="btn btn-danger" :title="t('logs.clear')" :disabled="clearing">
            <span v-if="clearing" class="h-4 w-4 animate-spin rounded-full border-2 border-current border-r-transparent"></span>
            <AppIcon v-else name="trash-2" class="h-4 w-4" />
          </button>
          <button @click="downloadLog" class="btn bg-[#1e293b] border border-[#334155] text-[#94a3b8] hover:text-white" :title="t('logs.download')">
            <AppIcon name="download" class="h-4 w-4" />
          </button>
        </div>
      </div>
      <div v-if="errorMessage" class="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{{ errorMessage }}</div>
      <div v-if="successMessage" class="mb-4 rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-3 text-sm text-emerald-300">{{ successMessage }}</div>

      <!-- Log Terminal -->
      <div class="relative bg-[#0f172a] rounded-xl border border-[#334155] overflow-hidden">
        <div v-if="loading && !lines.length" class="absolute inset-0 z-10 flex items-center justify-center bg-[#0f172a]/80 backdrop-blur-sm">
          <div class="flex flex-col items-center">
            <span class="mb-2 h-8 w-8 animate-spin rounded-full border-2 border-indigo-500 border-r-transparent"></span>
            <span class="text-[#94a3b8] text-sm">{{ t('logs.loading') }}</span>
          </div>
        </div>
        
        <div 
          ref="logContainer" 
          class="p-4 h-[600px] overflow-y-auto font-mono text-xs sm:text-sm text-[#e2e8f0] leading-relaxed"
          @scroll="handleScroll"
        >
          <div v-if="filteredLines.length === 0" class="text-[#64748b] text-center py-10">
            {{ searchQuery ? t('logs.noMatch') : t('logs.empty') }}
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
          <AppIcon name="arrow-down" class="h-5 w-5" />
        </button>
      </div>
    </div>
    
    <div v-else class="py-10 text-center text-[#64748b]">
      <AppIcon name="file-text" class="mx-auto mb-4 h-10 w-10 text-[#475569]" />
      <h3 class="text-lg font-medium text-white mb-2">{{ t('logs.unavailable') }}</h3>
      <p>{{ t('logs.unavailableDescription') }}</p>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import api, { apiErrorMessage } from '@/api/client'
import { useConfirmStore } from '@/stores/confirm'
import AppIcon from '@/components/AppIcon.vue'
import { useI18n } from 'vue-i18n'

const confirm = useConfirmStore()
const { t } = useI18n()

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
    errorMessage.value = apiErrorMessage(err, t('logs.loadFailed'))
  } finally {
    if (!isBackground) loading.value = false
  }
}

async function clearLog() {
  if (!selectedLogId.value) return
  const confirmed = await confirm.require({
    title: t('logs.clearTitle'),
    message: t('logs.clearMessage'),
    confirmText: t('logs.clear'),
    type: 'danger'
  })
  if (!confirmed) return
  
  clearing.value = true
  try {
    await api.post(`/modules/${props.moduleId}/logs/${selectedLogId.value}/clear`)
    lines.value = []
    successMessage.value = t('logs.cleared')
  } catch (err) {
    errorMessage.value = apiErrorMessage(err, t('logs.clearFailed'))
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
