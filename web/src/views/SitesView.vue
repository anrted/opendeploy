<template>
  <div>
    <div v-if="errorMessage" class="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{{ errorMessage }}</div>
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="page-title">{{ t('sitesPage.title') }}</h1>
        <p class="page-subtitle">{{ t('sitesPage.subtitle') }}</p>
      </div>
      <button id="create-site-btn" class="btn-primary" @click="openCreateModal">
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
            <th>{{ t('sitesPage.domain') }}</th>
            <th>{{ t('sitesPage.root') }}</th>
            <th>{{ t('sitesPage.app') }}</th>
            <th>SSL</th>
            <th>{{ t('sitesPage.state') }}</th>
            <th>{{ t('sitesPage.module') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading">
            <td colspan="7" class="text-center py-10 text-[#4a5568]">{{ t('common.loading') }}</td>
          </tr>
          <tr v-else-if="!sites.length">
            <td colspan="7" class="p-0">
              <EmptyState title="No Sites" description="No sites configured yet. Add a new site to get started.">
                <template #action>
                  <button class="btn-primary" @click="openCreateModal">{{ t('sitesPage.add') }}</button>
                </template>
              </EmptyState>
            </td>
          </tr>
          <tr v-for="site in sites" :key="site.id">
            <td class="font-medium text-white">{{ primaryDomain(site) }}</td>
            <td class="font-mono text-xs">{{ site.root_path }}</td>
            <td>{{ appLabel(site) }}</td>
            <td>
              <span v-if="site.ssl" class="badge-success">SSL</span>
              <span v-else class="badge-muted">HTTP</span>
            </td>
            <td><span :class="siteBadge(site.state)">{{ site.state }}</span></td>
            <td class="text-[#64748b]">{{ site.module_id }}</td>
            <td>
              <div class="flex gap-2">
                <button v-if="site.state === 'disabled'" @click="enableSite(site.id)" class="btn-success text-xs px-2 py-1">{{ t('moduleDetails.enable') }}</button>
                <button v-if="site.state === 'active'" @click="disableSite(site.id)" class="btn-danger text-xs px-2 py-1">{{ t('moduleDetails.disable') }}</button>
                <button @click="openEditModal(site)" class="btn-secondary text-xs px-2 py-1">{{ t('common.edit') }}</button>
                <button @click="openFileManager(site)" class="btn-primary text-xs px-2 py-1">{{ t('sitesPage.files') }}</button>
                <button @click="deleteSite(site.id)" class="btn-danger text-xs px-2 py-1">{{ t('common.delete') }}</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Create/Edit site modal -->
    <div v-if="showModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div class="card w-full max-w-md mx-4">
        <h2 class="text-lg font-semibold text-white mb-4">{{ isEditing ? 'Edit Site' : 'Add Site' }}</h2>
        <form @submit.prevent="submitSite" class="space-y-4">
          <div>
            <label class="label">{{ t('sitesPage.domain') }}</label>
            <input id="site-domain" v-model="form.domain" class="input" placeholder="example.com" required :disabled="isEditing" />
          </div>
          <div>
            <label class="label">{{ t('sitesPage.root') }}</label>
            <input id="site-root" v-model="form.root_path" class="input opacity-75 cursor-not-allowed" placeholder="/var/www/example" required disabled />
          </div>
          <div>
            <input id="site-root" v-model="form.root_path" class="input opacity-75 cursor-not-allowed" placeholder="/var/www/example" required disabled />
          </div>
          <div>
            <label class="label">{{ t('sitesPage.webServer') }}</label>
            <select v-model="form.module_id" class="input" required :disabled="isEditing">
              <option value="nginx">Nginx</option>
              <option value="apache">Apache</option>
            </select>
          </div>
          <div>
            <label class="label">Mode</label>
            <select v-model="form.web_mode" class="input">
              <option value="static">Static files</option>
              <option value="proxy">Reverse Proxy</option>
            </select>
          </div>
          <template v-if="form.web_mode === 'proxy'">
            <div>
              <label class="label">Proxy host</label>
              <input v-model="form.proxy_host" class="input" placeholder="127.0.0.1" required />
            </div>
            <div>
              <label class="label">Proxy port</label>
              <input v-model.number="form.proxy_port" type="number" class="input" placeholder="8080" required min="1" max="65535" />
            </div>
          </template>
          <div v-else>
            <label class="label">{{ t('sitesPage.phpVersion') }}</label>
            <select v-model="form.php_version" class="input">
              <option value="">{{ t('sitesPage.static') }}</option>
              <option>8.1</option><option>8.2</option><option>8.3</option><option>8.4</option>
            </select>
          </div>
          <div class="flex items-center gap-2">
            <input id="site-ssl" v-model="form.ssl_enabled" type="checkbox" class="rounded" />
            <label for="site-ssl" class="text-sm text-[#e2e8f0] cursor-pointer">{{ t('sitesPage.enableSsl') }}</label>
          </div>
          <div v-if="form.ssl_enabled">
            <label class="label">{{ t('sitesPage.certificate') }}</label>
            <input v-model="form.ssl_cert" class="input opacity-75 cursor-not-allowed" readonly
              :placeholder="`/etc/letsencrypt/live/${form.domain || 'example.com'}/fullchain.pem`" required />
          </div>
          <div v-if="form.ssl_enabled">
            <label class="label">{{ t('sitesPage.privateKey') }}</label>
            <input v-model="form.ssl_key" class="input opacity-75 cursor-not-allowed" readonly
              :placeholder="`/etc/letsencrypt/live/${form.domain || 'example.com'}/privkey.pem`" required />
          </div>
          <div v-if="submitError" class="text-sm text-red-400">{{ submitError }}</div>
          <div class="flex gap-3 justify-end">
            <button type="button" class="btn-secondary" @click="showModal = false">{{ t('common.cancel') }}</button>
            <button id="create-site-submit" type="submit" class="btn-primary" :disabled="submitting">
              {{ submitting ? 'Saving…' : 'Save' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- File Manager -->
    <FileManager v-if="showFiles" :site="selectedSite" @close="showFiles = false" />
  </div>
</template>

<script setup>
import { ref, onMounted, reactive, watch } from 'vue'
import api, { apiErrorMessage } from '@/api/client'
import FileManager from '@/components/FileManager/index.vue'
import EmptyState from '@/components/EmptyState.vue'
import { useConfirmStore } from '@/stores/confirm'
import { useI18n } from 'vue-i18n'

const confirm = useConfirmStore()
const { t } = useI18n()

const sites = ref([])
const loading = ref(true)
const showModal = ref(false)
const isEditing = ref(false)
const submitting = ref(false)
const submitError = ref('')
const errorMessage = ref('')
const showFiles = ref(false)
const selectedSite = ref(null)

const form = reactive({
  domain: '', root_path: '', module_id: 'nginx', php_version: '', ssl_enabled: false, ssl_cert: '', ssl_key: '',
  web_mode: 'static', proxy_host: '127.0.0.1', proxy_port: 8080
})

// Helpers to extract data from new relational API format
function primaryDomain(site) {
  if (!site.domains || !site.domains.length) return site.name || '—'
  const primary = site.domains.find(d => d.type === 'primary') || site.domains[0]
  return primary.domain
}

function appLabel(site) {
  if (site.proxy_enabled) return 'Reverse Proxy'
  if (!site.app) return '—'
  if (site.app.app_type === 'php') return `PHP ${site.app.app_version || ''}`
  if (site.app.app_type === 'proxy') return 'Proxy'
  return 'Static'
}

watch(() => form.domain, (newDomain) => {
  if (!newDomain) return
  if (form.root_path === '' || form.root_path.startsWith('/var/www/')) {
    form.root_path = `/var/www/${newDomain}`
  }
  if (form.ssl_enabled) {
    if (form.ssl_cert === '' || form.ssl_cert.startsWith('/etc/letsencrypt/live/')) {
      form.ssl_cert = `/etc/letsencrypt/live/${newDomain}/fullchain.pem`
      form.ssl_key = `/etc/letsencrypt/live/${newDomain}/privkey.pem`
    }
  }
})

watch(() => form.ssl_enabled, (enabled) => {
  if (enabled && form.domain) {
    if (form.ssl_cert === '' || form.ssl_cert.startsWith('/etc/letsencrypt/live/')) {
      form.ssl_cert = `/etc/letsencrypt/live/${form.domain}/fullchain.pem`
      form.ssl_key = `/etc/letsencrypt/live/${form.domain}/privkey.pem`
    }
  }
})

onMounted(loadSites)

async function loadSites() {
  loading.value = true
  try {
    errorMessage.value = ''
    const { data } = await api.get('/sites')
    sites.value = data || []
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, t('sitesPage.loadFailed'))
  } finally {
    loading.value = false
  }
}

function siteBadge(state) {
  return { active: 'badge-success', disabled: 'badge-muted', error: 'badge-danger' }[state] || 'badge-muted'
}

function openCreateModal() {
  isEditing.value = false
  selectedSite.value = null
  form.domain = ''
  form.root_path = ''
  form.module_id = 'nginx'
  form.php_version = ''
  form.ssl_enabled = false
  form.ssl_cert = ''
  form.ssl_key = ''
  form.web_mode = 'static'
  form.proxy_host = '127.0.0.1'
  form.proxy_port = 8080
  submitError.value = ''
  showModal.value = true
}

function openEditModal(site) {
  isEditing.value = true
  selectedSite.value = site
  // Map from new relational API format back to form fields
  form.domain = primaryDomain(site)
  form.root_path = site.root_path
  form.module_id = site.module_id || 'nginx'
  form.php_version = site.app?.app_version || ''
  form.ssl_enabled = !!site.ssl
  form.ssl_cert = site.ssl?.cert_path || ''
  form.ssl_key = site.ssl?.key_path || ''
  form.web_mode = site.proxy_enabled ? 'proxy' : 'static'
  form.proxy_host = site.proxy_host || '127.0.0.1'
  form.proxy_port = site.proxy_port || 8080
  submitError.value = ''
  showModal.value = true
}

function openFileManager(site) {
  selectedSite.value = site
  showFiles.value = true
}

async function submitSite() {
  submitError.value = ''
  submitting.value = true
  try {
    const domain = form.domain
    // Auto-fill ssl paths from domain if still empty (e.g. user enabled SSL before typing domain)
    const sslCert = form.ssl_cert || (form.ssl_enabled && domain ? `/etc/letsencrypt/live/${domain}/fullchain.pem` : null)
    const sslKey = form.ssl_key || (form.ssl_enabled && domain ? `/etc/letsencrypt/live/${domain}/privkey.pem` : null)
    const payload = {
      domain,
      name: domain,
      root_path: form.root_path,
      module_id: form.module_id,
      app_type: form.web_mode === 'proxy' ? 'static' : (form.php_version ? 'php' : 'static'),
      app_version: form.web_mode === 'proxy' ? null : (form.php_version || null),
      ssl_enabled: form.ssl_enabled,
      ssl_cert: form.ssl_enabled ? sslCert : null,
      ssl_key: form.ssl_enabled ? sslKey : null,
      proxy_enabled: form.web_mode === 'proxy',
      proxy_host: form.web_mode === 'proxy' ? form.proxy_host : null,
      proxy_port: form.web_mode === 'proxy' ? form.proxy_port : null,
    }
    if (isEditing.value) {
      await api.put(`/sites/${selectedSite.value.id}`, payload)
    } else {
      await api.post('/sites', payload)
    }
    showModal.value = false
    form.domain = form.root_path = form.php_version = form.ssl_cert = form.ssl_key = ''
    form.ssl_enabled = false
    form.web_mode = 'static'
    form.proxy_host = '127.0.0.1'
    form.proxy_port = 8080
    await loadSites()
  } catch (e) {
    submitError.value = apiErrorMessage(e, isEditing.value ? t('sitesPage.updateFailed') : t('sitesPage.createFailed'))
}

async function enableSite(id) {
  await siteAction(id, 'enable')
}

async function disableSite(id) {
  const confirmed = await confirm.require({
    title: t('sitesPage.disableTitle'),
    message: t('sitesPage.disableMessage'),
    confirmText: t('moduleDetails.disable'),
    type: 'warning'
  })
  if (!confirmed) return
  await siteAction(id, 'disable')
}

async function deleteSite(id) {
  const confirmed = await confirm.require({
    title: t('sitesPage.deleteTitle'),
    message: t('sitesPage.deleteMessage'),
    confirmText: t('common.delete'),
    type: 'danger'
  })
  if (!confirmed) return
  try {
    errorMessage.value = ''
    await api.delete(`/sites/${id}`)
    await loadSites()
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, t('sitesPage.deleteFailed'))
  }
}

async function siteAction(id, action) {
  try {
    errorMessage.value = ''
    await api.post(`/sites/${id}/${action}`)
    await loadSites()
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, t('sitesPage.actionFailed', { action }))
  }
}
</script>
