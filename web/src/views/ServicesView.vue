<template>
  <div>
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
          <tr v-else-if="!services.length"><td colspan="4" class="text-center py-10 text-[#4a5568]">No services tracked</td></tr>
          <tr v-for="svc in services" :key="svc.id">
            <td class="font-medium text-white">{{ svc.name }}</td>
            <td class="font-mono text-xs text-[#64748b]">{{ svc.unit }}</td>
            <td><span :class="stateBadge(svc.state)">{{ svc.state }}</span></td>
            <td>
              <div class="flex gap-2">
                <button v-if="svc.state !== 'running'" @click="startSvc(svc.id)" class="btn-success text-xs px-2 py-1">Start</button>
                <button v-if="svc.state === 'running'" @click="stopSvc(svc.id)" class="btn-danger text-xs px-2 py-1">Stop</button>
                <button @click="restartSvc(svc.id)" class="btn-secondary text-xs px-2 py-1">Restart</button>
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
  </div>
</template>

<script setup>
import { ref, onMounted, reactive } from 'vue'
import api from '@/api/client'

const services = ref([])
const loading = ref(true)
const showAdd = ref(false)
const adding = ref(false)
const form = reactive({ name: '', unit: '', description: '' })

onMounted(load)

async function load() {
  loading.value = true
  try {
    const { data } = await api.get('/services')
    services.value = data || []
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
    await api.post('/services', form)
    showAdd.value = false
    form.name = form.unit = form.description = ''
    await load()
  } finally {
    adding.value = false
  }
}

const startSvc   = async (id) => { await api.post(`/services/${id}/start`).catch(console.error); await load() }
const stopSvc    = async (id) => { await api.post(`/services/${id}/stop`).catch(console.error); await load() }
const restartSvc = async (id) => { await api.post(`/services/${id}/restart`).catch(console.error); await load() }
const removeSvc  = async (id) => {
  if (!confirm('Remove this service?')) return
  await api.delete(`/services/${id}`).catch(console.error)
  await load()
}
</script>
