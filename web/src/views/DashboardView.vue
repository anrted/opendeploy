<template>
  <div>
    <div v-if="errorMessage" class="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
      {{ errorMessage }}
      <button class="ml-2 underline" @click="loadOverview">{{ t('dashboard.retry') }}</button>
    </div>
    <!-- Header -->
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="page-title">{{ t('dashboard.title') }}</h1>
        <p class="page-subtitle">{{ t('dashboard.subtitle') }}</p>
      </div>
      <div class="flex items-center gap-2">
        <div :class="['flex items-center gap-1.5 text-xs', wsConnected ? 'text-emerald-400' : 'text-[#64748b]']">
          <span class="relative flex h-2 w-2">
            <span v-if="wsConnected" class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
            <span :class="['relative inline-flex rounded-full h-2 w-2', wsConnected ? 'bg-emerald-500' : 'bg-slate-600']"></span>
          </span>
          {{ wsConnected ? t('dashboard.live') : t('dashboard.connecting') }}
        </div>
      </div>
    </div>

    <!-- Stat tiles -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
      <StatTile
        :label="t('dashboard.cpu')"
        :value="formatPct(stats?.cpu?.usage_percent)"
        icon="cpu"
        :color="colorFromPct(stats?.cpu?.usage_percent)"
        :subtitle="t('dashboard.cores', { count: stats?.cpu?.cores || 0 })"
      />
      <StatTile
        :label="t('dashboard.memory')"
        :value="formatPct(stats?.memory?.used_percent)"
        icon="memory"
        :color="colorFromPct(stats?.memory?.used_percent)"
        :subtitle="formatBytes(stats?.memory?.used) + ' / ' + formatBytes(stats?.memory?.total)"
      />
      <StatTile
        :label="t('dashboard.disk')"
        :value="formatPct(stats?.disk?.[0]?.used_percent)"
        icon="disk"
        :color="colorFromPct(stats?.disk?.[0]?.used_percent)"
        :subtitle="formatBytes(stats?.disk?.[0]?.used) + ' / ' + formatBytes(stats?.disk?.[0]?.total)"
      />
      <StatTile
        :label="t('dashboard.load')"
        :value="(stats?.load_average?.[0] || 0).toFixed(2)"
        icon="activity"
        color="indigo"
        :subtitle="`5m: ${(stats?.load_average?.[1] || 0).toFixed(2)} · 15m: ${(stats?.load_average?.[2] || 0).toFixed(2)}`"
      />
    </div>

    <!-- Modules summary + Recent jobs -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-4 mb-6">
      <!-- Modules summary -->
      <div class="card">
        <h3 class="text-sm font-semibold text-[#94a3b8] mb-4">{{ t('dashboard.modules') }}</h3>
        <div class="grid grid-cols-3 gap-3">
          <div class="text-center">
            <div class="text-2xl font-bold text-emerald-400">{{ overview?.modules?.enabled || 0 }}</div>
            <div class="text-xs text-[#64748b] mt-0.5">{{ t('common.enabled') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-[#e2e8f0]">{{ overview?.modules?.total || 0 }}</div>
            <div class="text-xs text-[#64748b] mt-0.5">{{ t('dashboard.total') }}</div>
          </div>
          <div class="text-center">
            <div class="text-2xl font-bold text-red-400">{{ overview?.modules?.error || 0 }}</div>
            <div class="text-xs text-[#64748b] mt-0.5">{{ t('dashboard.errors') }}</div>
          </div>
        </div>
        <div class="mt-4 pt-4 border-t border-[#2d3748]">
          <div class="text-xs text-[#64748b]">{{ t('dashboard.uptime') }}: {{ formatUptime(stats?.uptime) }}</div>
        </div>
      </div>

      <!-- Recent jobs -->
      <div class="card lg:col-span-2">
        <h3 class="text-sm font-semibold text-[#94a3b8] mb-4">{{ t('dashboard.recentJobs') }}</h3>
        <div v-if="!overview?.recent_jobs?.length" class="text-sm text-[#4a5568] py-4 text-center">
          {{ t('dashboard.noJobs') }}
        </div>
        <div v-else class="space-y-2">
          <div v-for="job in (overview?.recent_jobs || []).slice(0, 5)" :key="job.id"
               class="flex items-center gap-3 text-sm py-1.5">
            <span :class="jobBadgeClass(job.state)">{{ job.state }}</span>
            <span class="text-[#94a3b8] flex-1 truncate">{{ formatJobType(job.type) }}</span>
            <span class="text-[#4a5568] text-xs">{{ timeAgo(job.created_at) }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Audit log -->
    <div class="card">
      <h3 class="text-sm font-semibold text-[#94a3b8] mb-4">{{ t('dashboard.recentActivity') }}</h3>
      <div v-if="!overview?.recent_audit?.length" class="text-sm text-[#4a5568] py-4 text-center">
        {{ t('dashboard.noActivity') }}
      </div>
      <div v-else class="divide-y divide-[#2d3748]">
        <div v-for="entry in (overview?.recent_audit || []).slice(0, 8)" :key="entry.created_at"
             class="flex items-center gap-3 py-2.5 text-sm">
          <span :class="entry.status === 'success' ? 'badge-success' : 'badge-danger'">
            {{ entry.status }}
          </span>
          <span class="text-[#94a3b8] flex-1">{{ entry.action }}</span>
          <span v-if="entry.resource" class="text-[#64748b] text-xs">{{ entry.resource }}</span>
          <span class="text-[#4a5568] text-xs">{{ timeAgo(entry.created_at) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import api, { apiErrorMessage } from '@/api/client'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
import StatTile from '@/components/StatTile.vue'

const overview = ref(null)
const stats = ref(null)
const wsConnected = ref(false)
const errorMessage = ref('')
let ws = null
let reconnectTimer = null

onMounted(async () => {
  await loadOverview()
  connectWebSocket()
})

onUnmounted(() => {
  ws?.close()
  clearTimeout(reconnectTimer)
})

async function loadOverview() {
  try {
    errorMessage.value = ''
    const { data } = await api.get('/dashboard')
    overview.value = data
    stats.value = data.server_stats
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, t('dashboard.loadFailed'))
  }
}

async function connectWebSocket() {
  let ticket
  try {
    const { data } = await api.post('/dashboard/ws-ticket')
    ticket = data.ticket
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, t('dashboard.liveFailed'))
    reconnectTimer = setTimeout(connectWebSocket, 5000)
    return
  }
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  ws = new WebSocket(`${proto}//${location.host}/api/v1/dashboard/ws?ticket=${encodeURIComponent(ticket)}`)
  ws.onopen = () => { wsConnected.value = true }
  ws.onclose = () => {
    wsConnected.value = false
    // Reconnect after 5s
    reconnectTimer = setTimeout(connectWebSocket, 5000)
  }
  ws.onmessage = (evt) => {
    try {
      const msg = JSON.parse(evt.data)
      if (msg.type === 'stats_update') {
        stats.value = msg.payload
      } else if (msg.type === 'site_changed') {
        loadOverview()
      }
    } catch { /* ignore */ }
  }
}

// ─── formatting ─────────────────────────────────────────────────────────────

function formatPct(v) {
  return v != null ? v.toFixed(1) + '%' : '—'
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i]
}

function formatUptime(secs) {
  if (!secs) return '—'
  const d = Math.floor(secs / 86400)
  const h = Math.floor((secs % 86400) / 3600)
  const m = Math.floor((secs % 3600) / 60)
  if (d > 0) return `${d}d ${h}h ${m}m`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function colorFromPct(pct) {
  if (pct == null) return 'muted'
  if (pct > 85) return 'red'
  if (pct > 65) return 'amber'
  return 'emerald'
}

function jobBadgeClass(state) {
  const map = {
    success: 'badge-success', error: 'badge-danger',
    running: 'badge-primary', pending: 'badge-muted',
  }
  return map[state] || 'badge-muted'
}

function formatJobType(type) {
  return type?.replace(/_/g, ' ') || type
}

function timeAgo(dateStr) {
  if (!dateStr) return ''
  const diff = (Date.now() - new Date(dateStr).getTime()) / 1000
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}
</script>
