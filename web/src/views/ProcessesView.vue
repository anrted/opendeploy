<template>
  <div class="space-y-6">
    <div class="flex flex-col items-start justify-between gap-4 sm:flex-row sm:items-center">
      <div>
        <h1 class="text-2xl font-bold text-text-main">{{ t('processes.title') }}</h1>
        <p class="mt-1 text-sm text-text-muted">{{ t('processes.subtitle') }}</p>
      </div>
      <button @click="fetchProcesses" class="flex items-center gap-2 rounded-lg border border-border-subtle bg-bg-card px-4 py-2 text-sm font-medium transition-colors hover:bg-slate-50 dark:hover:bg-[#1e2535]">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-width="2" d="M4 4v5h5M20 20v-5h-5M5 9a8 8 0 0114.5 2M19 15A8 8 0 014.5 13"/></svg>
        {{ t('common.refresh') }}
      </button>
    </div>

    <div class="relative">
      <svg class="absolute left-3 top-2.5 h-5 w-5 text-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>
      <input v-model="searchQuery" :placeholder="t('processes.search')" class="w-full rounded-lg border border-border-subtle bg-bg-card py-2 pl-10 pr-4 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500">
    </div>

    <div class="overflow-hidden rounded-xl border border-border-subtle bg-bg-card">
      <div class="overflow-x-auto">
        <table class="w-full min-w-[760px] text-left text-sm">
          <thead class="border-b border-border-subtle bg-slate-50 text-text-muted dark:bg-[#1e2535]">
            <tr>
              <th v-for="column in columns" :key="column.key" class="cursor-pointer px-4 py-3 font-medium hover:text-text-main" @click="sortBy(column.key)">
                {{ column.label }} <span v-if="sortKey === column.key">{{ sortAsc ? '↑' : '↓' }}</span>
              </th>
              <th class="px-4 py-3 text-right font-medium">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border-subtle">
            <tr v-if="loading"><td colspan="7" class="px-4 py-6 text-center text-text-muted">{{ t('processes.loading') }}</td></tr>
            <tr v-else-if="filteredProcesses.length === 0"><td colspan="7" class="px-4 py-6 text-center text-text-muted">{{ t('processes.empty') }}</td></tr>
            <tr v-for="proc in filteredProcesses" v-else :key="proc.pid" class="transition-colors hover:bg-slate-50 dark:hover:bg-[#1e2535]/50">
              <td class="px-4 py-3 font-medium text-text-main">{{ proc.pid }}</td>
              <td class="px-4 py-3 text-text-main">{{ proc.user }}</td>
              <td class="px-4 py-3 text-text-main">{{ number(proc.cpu_percent) }}</td>
              <td class="px-4 py-3 text-text-main">{{ number(proc.mem_percent) }}</td>
              <td class="px-4 py-3 text-text-muted">{{ formatBytes(proc.mem_rss) }}</td>
              <td class="max-w-xs truncate px-4 py-3 font-mono text-xs text-text-muted" :title="proc.cmdline">{{ proc.name }} {{ proc.cmdline }}</td>
              <td class="px-4 py-3 text-right">
                <button @click="killProcess(proc.pid, proc.name)" class="rounded-md p-1.5 text-text-muted transition-colors hover:bg-red-500/10 hover:text-red-500" :title="t('processes.kill')">
                  <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
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
import { useI18n } from 'vue-i18n'
import api from '@/api/client'

const { t } = useI18n()
const processes = ref([])
const loading = ref(true)
const searchQuery = ref('')
const sortKey = ref('cpu_percent')
const sortAsc = ref(false)
let pollInterval

const columns = computed(() => [
  { key: 'pid', label: t('processes.pid') },
  { key: 'user', label: t('processes.user') },
  { key: 'cpu_percent', label: 'CPU %' },
  { key: 'mem_percent', label: 'MEM %' },
  { key: 'mem_rss', label: 'RSS' },
  { key: 'cmdline', label: t('processes.command') },
])

async function fetchProcesses() {
  try {
    const { data } = await api.get('/system/processes')
    processes.value = Array.isArray(data) ? data : []
  } catch (error) {
    console.error('Failed to fetch processes:', error)
  } finally {
    loading.value = false
  }
}

async function killProcess(pid, name) {
  if (!confirm(t('processes.confirmKill', { name, pid }))) return
  try {
    await api.post(`/system/processes/${pid}/kill`)
    fetchProcesses()
  } catch (error) {
    alert(t('processes.killFailed', { error: error.message }))
  }
}

function sortBy(key) {
  if (sortKey.value === key) sortAsc.value = !sortAsc.value
  else {
    sortKey.value = key
    sortAsc.value = true
  }
}

const filteredProcesses = computed(() => {
  const q = searchQuery.value.toLowerCase()
  const result = processes.value.filter((proc) => !q || [proc.name, proc.user, proc.cmdline, proc.pid].some((value) => String(value ?? '').toLowerCase().includes(q)))
  return result.sort((a, b) => {
    const left = typeof a[sortKey.value] === 'string' ? a[sortKey.value].toLowerCase() : (a[sortKey.value] ?? 0)
    const right = typeof b[sortKey.value] === 'string' ? b[sortKey.value].toLowerCase() : (b[sortKey.value] ?? 0)
    return (left < right ? -1 : left > right ? 1 : 0) * (sortAsc.value ? 1 : -1)
  })
})

const number = (value) => Number(value || 0).toFixed(1)
function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / 1024 ** index).toFixed(1)} ${units[index]}`
}

onMounted(() => {
  fetchProcesses()
  pollInterval = setInterval(fetchProcesses, 5000)
})
onUnmounted(() => clearInterval(pollInterval))
</script>
