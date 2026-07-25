<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="page-title">Sites</h1>
        <p class="page-subtitle">Manage virtual hosts</p>
      </div>
      <button id="create-site-btn" class="btn-primary" @click="showCreate = true">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
        </svg>
        Add Site
      </button>
    </div>

    <!-- Sites table -->
    <div class="table-wrapper">
      <table class="table">
        <thead>
          <tr>
            <th>Domain</th>
            <th>Root Path</th>
            <th>PHP</th>
            <th>SSL</th>
            <th>State</th>
            <th>Module</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading">
            <td colspan="7" class="text-center py-10 text-[#4a5568]">Loading…</td>
          </tr>
          <tr v-else-if="!sites.length">
            <td colspan="7" class="text-center py-10 text-[#4a5568]">No sites configured yet</td>
          </tr>
          <tr v-for="site in sites" :key="site.id">
            <td class="font-medium text-white">{{ site.domain }}</td>
            <td class="font-mono text-xs">{{ site.root_path }}</td>
            <td>{{ site.php_version || '—' }}</td>
            <td>
              <span v-if="site.ssl_enabled" class="badge-success">SSL</span>
              <span v-else class="badge-muted">HTTP</span>
            </td>
            <td><span :class="siteBadge(site.state)">{{ site.state }}</span></td>
            <td class="text-[#64748b]">{{ site.module_id }}</td>
            <td>
              <div class="flex gap-2">
                <button v-if="site.state === 'disabled'" @click="enableSite(site.id)" class="btn-success text-xs px-2 py-1">Enable</button>
                <button v-if="site.state === 'active'" @click="disableSite(site.id)" class="btn-danger text-xs px-2 py-1">Disable</button>
                <button @click="deleteSite(site.id)" class="btn-danger text-xs px-2 py-1">Delete</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Create site modal -->
    <div v-if="showCreate" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="card w-full max-w-md mx-4">
        <h2 class="text-lg font-semibold text-white mb-4">Add Site</h2>
        <form @submit.prevent="createSite" class="space-y-4">
          <div>
            <label class="label">Domain</label>
            <input id="site-domain" v-model="form.domain" class="input" placeholder="example.com" required />
          </div>
          <div>
            <label class="label">Root Path</label>
            <input id="site-root" v-model="form.root_path" class="input" placeholder="/var/www/example" required />
          </div>
          <div>
            <label class="label">PHP Version (optional)</label>
            <select v-model="form.php_version" class="input">
              <option value="">None</option>
              <option>8.1</option><option>8.2</option><option>8.3</option><option>8.4</option>
            </select>
          </div>
          <div class="flex items-center gap-2">
            <input id="site-ssl" v-model="form.ssl_enabled" type="checkbox" class="rounded" />
            <label for="site-ssl" class="text-sm text-[#e2e8f0] cursor-pointer">Enable SSL</label>
          </div>
          <div v-if="createError" class="text-sm text-red-400">{{ createError }}</div>
          <div class="flex gap-3 justify-end">
            <button type="button" class="btn-secondary" @click="showCreate = false">Cancel</button>
            <button id="create-site-submit" type="submit" class="btn-primary" :disabled="creating">
              {{ creating ? 'Creating…' : 'Create' }}
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

const sites = ref([])
const loading = ref(true)
const showCreate = ref(false)
const creating = ref(false)
const createError = ref('')

const form = reactive({
  domain: '', root_path: '', php_version: '', ssl_enabled: false,
})

onMounted(loadSites)

async function loadSites() {
  loading.value = true
  try {
    const { data } = await api.get('/sites')
    sites.value = data || []
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

function siteBadge(state) {
  return { active: 'badge-success', disabled: 'badge-muted', error: 'badge-danger' }[state] || 'badge-muted'
}

async function createSite() {
  createError.value = ''
  creating.value = true
  try {
    await api.post('/sites', {
      ...form,
      php_version: form.php_version || null,
      module_id: 'nginx',
    })
    showCreate.value = false
    form.domain = form.root_path = form.php_version = ''
    form.ssl_enabled = false
    await loadSites()
  } catch (e) {
    createError.value = e.response?.data?.error?.message || 'Error creating site'
  } finally {
    creating.value = false
  }
}

async function enableSite(id) {
  await api.post(`/sites/${id}/enable`).catch(console.error)
  await loadSites()
}

async function disableSite(id) {
  await api.post(`/sites/${id}/disable`).catch(console.error)
  await loadSites()
}

async function deleteSite(id) {
  if (!confirm('Delete this site?')) return
  await api.delete(`/sites/${id}`).catch(console.error)
  await loadSites()
}
</script>
