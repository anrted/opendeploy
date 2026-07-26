<template>
  <div>
    <div v-if="errorMessage" class="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{{ errorMessage }}</div>
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="page-title">Services</h1>
        <p class="page-subtitle">Manage system services</p>
      </div>
      <button id="add-service-btn" class="btn-primary" @click="showAdd = true">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
        </svg>
        Add Service
      </button>
    </div>

    <div class="table-wrapper">
      <table class="table">
        <thead>
          <tr>
            <th>Name</th><th>Unit</th><th>State</th><th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading"><td colspan="4" class="text-center py-10 text-[#4a5568]">Loading…</td></tr>
          <tr v-else-if="!services.length">
            <td colspan="4" class="p-0">
              <EmptyState title="No Services" description="No system services tracked. Track a service to get started.">
                <template #action>
                  <button class="btn-primary" @click="showAdd = true">Add Service</button>
                </template>
              </EmptyState>
            </td>
          </tr>
          <tr v-for="svc in services" :key="svc.id">
            <td class="font-medium text-white">{{ svc.name }}</td>
            <td class="font-mono text-xs text-[#64748b]">{{ svc.unit }}</td>
            <td><span :class="stateBadge(svc.state)">{{ svc.state }}</span></td>
            <td>
              <div class="flex gap-2">
                <button v-if="svc.state !== 'running'" @click="startSvc(svc.id)" class="btn-success text-xs px-2 py-1">Start</button>
                <button v-if="svc.state === 'running'" @click="stopSvc(svc.id)" class="btn-danger text-xs px-2 py-1">Stop</button>
                <button @click="restartSvc(svc.id)" class="btn-secondary text-xs px-2 py-1">Restart</button>
                <button @click="openLogs(svc)" class="btn-primary text-xs px-2 py-1">Logs</button>
                <button @click="removeSvc(svc.id)" class="btn-danger text-xs px-2 py-1">Remove</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Add service modal -->
    <div v-if="showAdd" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="card w-full max-w-md mx-4">
        <h2 class="text-lg font-semibold text-white mb-4">Track Service</h2>
        <form @submit.prevent="addService" class="space-y-4">
          <div>
            <label class="label">Display Name</label>
            <input v-model="form.name" class="input" placeholder="Redis" required />
          </div>
          <div>
            <label class="label">systemd Unit</label>
            <input v-model="form.unit" class="input" placeholder="redis.service" required />
          </div>
          <div>
            <label class="label">Description (optional)</label>
            <input v-model="form.description" class="input" placeholder="In-memory data store" />
          </div>
          <div class="flex gap-3 justify-end">
            <button type="button" class="btn-secondary" @click="showAdd = false">Cancel</button>
            <button type="submit" class="btn-primary" :disabled="adding">
              {{ adding ? 'Adding…' : 'Add' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Live Logs modal -->
    <div v-if="showLogs" class="fixed inset-0 z-50 flex flex-col bg-black/80 backdrop-blur-sm">
      <div class="flex items-center justify-between p-4 bg-[#1a202c]">
        <h2 class="text-lg font-bold text-white tracking-wide font-mono">Logs: {{ activeLogSvc?.name }}</h2>
        <button @click="closeLogs" class="text-slate-400 hover:text-white transition-colors">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
        </button>
      </div>
      <div class="flex-1 overflow-auto p-4 bg-[#0d1117]" ref="logsContainer">
        <pre class="font-mono text-sm text-green-400 whitespace-pre-wrap leading-relaxed">{{ logContent }}</pre>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, reactive, nextTick, onBeforeUnmount } from 'vue'
import api, { apiErrorMessage } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import EmptyState from '@/components/EmptyState.vue'
import { useConfirmStore } from '@/stores/confirm'

const confirm = useConfirmStore()

const services = ref([])
const loading = ref(true)
const showAdd = ref(false)
const adding = ref(false)
const actionID = ref('')
const errorMessage = ref('')
const form = reactive({ name: '', unit: '', description: '' })

// Logs state
const showLogs = ref(false)
const activeLogSvc = ref(null)
const logContent = ref('')
const logsContainer = ref(null)
let ws = null

onMounted(load)

onBeforeUnmount(() => {
  if (ws) ws.close()
})

async function load() {
  loading.value = true
  try {
    errorMessage.value = ''
    const { data } = await api.get('/services')
    services.value = data || []
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'Unable to load services')
  } finally {
    loading.value = false
  }
}

function stateBadge(s) {
  return { running: 'badge-success', stopped: 'badge-muted', failed: 'badge-danger', unknown: 'badge-muted' }[s] || 'badge-muted'
}

async function addService() {
  adding.value = true
  try {
    errorMessage.value = ''
    await api.post('/services', form)
    showAdd.value = false
    form.name = form.unit = form.description = ''
    await load()
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'Unable to add service')
  } finally {
    adding.value = false
  }
}

async function serviceAction(id, action) {
  if (['stop', 'restart'].includes(action)) {
    const confirmed = await confirm.require({
      title: `${action.charAt(0).toUpperCase() + action.slice(1)} Service`,
      message: `Are you sure you want to ${action} this service?`,
      confirmText: action.charAt(0).toUpperCase() + action.slice(1),
      type: action === 'stop' ? 'danger' : 'warning'
    })
    if (!confirmed) return
  }
  actionID.value = id
  try {
    errorMessage.value = ''
    await api.post(`/services/${id}/${action}`)
    await load()
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, `Unable to ${action} service`)
  } finally {
    actionID.value = ''
  }
}
const startSvc   = (id) => serviceAction(id, 'start')
const stopSvc    = (id) => serviceAction(id, 'stop')
const restartSvc = (id) => serviceAction(id, 'restart')
const removeSvc  = async (id) => {
  const confirmed = await confirm.require({
    title: 'Remove Service',
    message: 'Are you sure you want to remove this service from tracking?',
    confirmText: 'Remove',
    type: 'danger'
  })
  if (!confirmed) return
  try {
    errorMessage.value = ''
    await api.delete(`/services/${id}`)
    await load()
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'Unable to remove service')
  }
}

function openLogs(svc) {
  activeLogSvc.value = svc
  logContent.value = 'Connecting to log stream...\n'
  showLogs.value = true
  
  if (ws) ws.close()
  
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  const auth = useAuthStore()
  
  ws = new WebSocket(`${protocol}//${host}/api/v1/services/${svc.id}/logs/stream?token=${auth.accessToken}`)
  
  ws.onmessage = (event) => {
    logContent.value += event.data + '\n'
    nextTick(() => {
      if (logsContainer.value) {
        logsContainer.value.scrollTop = logsContainer.value.scrollHeight
      }
    })
  }
  
  ws.onerror = () => {
    logContent.value += '\n[WebSocket Error]\n'
  }
  
  ws.onclose = () => {
    logContent.value += '\n[Connection Closed]\n'
  }
}

function closeLogs() {
  if (ws) {
    ws.close()
    ws = null
  }
  showLogs.value = false
  activeLogSvc.value = null
  logContent.value = ''
}
</script>
