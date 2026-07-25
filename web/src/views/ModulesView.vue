<template>
  <div>
    <div v-if="errorMessage" class="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{{ errorMessage }}</div>
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="page-title">Modules</h1>
        <p class="page-subtitle">Install and manage server modules</p>
      </div>
    </div>

    <div v-if="loading" class="flex justify-center py-20">
      <svg class="w-6 h-6 text-indigo-400 animate-spin" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
      </svg>
    </div>

    <div v-else class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
      <div v-for="mod in modules" :key="mod.id" class="card flex flex-col gap-4">
        <!-- Header -->
        <div class="flex items-start justify-between">
          <div>
            <h3 class="font-semibold text-[#e2e8f0]">{{ mod.name }}</h3>
            <p class="text-xs text-[#64748b] mt-0.5">{{ mod.description }}</p>
          </div>
          <span :class="stateBadge(mod.state)">{{ mod.state }}</span>
        </div>

        <!-- Version -->
        <div v-if="mod.installed_version" class="text-xs text-[#64748b]">
          Version: <span class="text-[#94a3b8]">{{ mod.installed_version }}</span>
        </div>

        <!-- Actions -->
        <div class="flex flex-wrap gap-2 mt-auto">
          <button v-if="!isInstalled(mod.state)" @click="install(mod.id)"
            class="btn-primary text-xs px-3 py-1.5" :disabled="actionLoading[mod.id]">
            {{ actionLoading[mod.id] === 'install' ? 'Installing…' : 'Install' }}
          </button>
          <button v-if="isInstalled(mod.state) && mod.state !== 'enabled'" @click="enable(mod.id)"
            class="btn-success text-xs px-3 py-1.5" :disabled="actionLoading[mod.id]">
            Enable
          </button>
          <button v-if="mod.state === 'enabled'" @click="restart(mod.id)"
            class="btn-secondary text-xs px-3 py-1.5" :disabled="actionLoading[mod.id]">
            Restart
          </button>
          <button v-if="mod.state === 'enabled'" @click="disable(mod.id)"
            class="btn-danger text-xs px-3 py-1.5" :disabled="actionLoading[mod.id]">
            Disable
          </button>
          <button v-if="isInstalled(mod.state)" @click="uninstall(mod.id)"
            class="btn-danger text-xs px-3 py-1.5" :disabled="actionLoading[mod.id]">
            {{ actionLoading[mod.id] === 'uninstall' ? 'Removing…' : 'Remove' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api, { apiErrorMessage } from '@/api/client'

const modules = ref([])
const loading = ref(true)
const actionLoading = ref({})
const errorMessage = ref('')

onMounted(loadModules)

async function loadModules() {
  loading.value = true
  try {
    errorMessage.value = ''
    const { data } = await api.get('/modules')
    modules.value = data || []
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'Unable to load modules')
  } finally {
    loading.value = false
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
  }
  return map[state] || 'badge-muted'
}

async function action(id, endpoint, label) {
  if (['uninstall', 'disable', 'restart'].includes(endpoint) &&
      !confirm(`${endpoint[0].toUpperCase() + endpoint.slice(1)} module ${id}?`)) return
  actionLoading.value[id] = label
  try {
    errorMessage.value = ''
    await api.post(`/modules/${id}/${endpoint}`)
    await loadModules()
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, `Unable to ${endpoint} module`)
  } finally {
    delete actionLoading.value[id]
  }
}

const install   = (id) => action(id, 'install', 'install')
const uninstall = (id) => action(id, 'uninstall', 'uninstall')
const enable    = (id) => action(id, 'enable', 'enable')
const disable   = (id) => action(id, 'disable', 'disable')
const restart   = (id) => action(id, 'restart', 'restart')
</script>
