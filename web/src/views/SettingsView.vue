<template>
  <div>
    <div class="mb-6">
      <h1 class="page-title">Settings</h1>
      <p class="page-subtitle">Application configuration</p>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <div class="card">
        <h2 class="text-base font-semibold text-white mb-4">General</h2>
        <form @submit.prevent="saveSettings" class="space-y-4">
          <div>
            <label class="label">Panel Title</label>
            <input id="setting-title" v-model="settings['core.panel_title']" class="input" placeholder="OpenDeploy" />
          </div>
          <div>
            <label class="label">Default PHP Version</label>
            <select v-model="settings['core.default_php']" class="input">
              <option value="">None</option>
              <option v-for="v in phpVersions" :key="v" :value="v">{{ v }}</option>
            </select>
          </div>

          <div v-if="saved" class="text-sm text-emerald-400">Settings saved!</div>
          <div v-if="saveError" class="text-sm text-red-400">{{ saveError }}</div>

          <button id="save-settings-btn" type="submit" class="btn-primary" :disabled="saving">
            {{ saving ? 'Saving…' : 'Save Settings' }}
          </button>
        </form>
      </div>

      <div class="card">
        <h2 class="text-base font-semibold text-white mb-4">About</h2>
        <div class="space-y-3 text-sm">
          <div class="flex justify-between">
            <span class="text-[#64748b]">Version</span>
            <span class="text-[#e2e8f0] font-mono">{{ updateStatus?.current_version || 'unknown' }}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-[#64748b]">License</span>
            <span class="text-[#e2e8f0]">MIT</span>
          </div>
          <div class="flex justify-between">
            <span class="text-[#64748b]">GitHub</span>
            <a href="https://github.com/anrted/opendeploy" class="text-indigo-400 hover:text-indigo-300">
              anrted/opendeploy
            </a>
          </div>
        </div>
      </div>

      <div class="card lg:col-span-2">
        <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h2 class="text-base font-semibold text-white">Project updates</h2>
            <p class="mt-1 text-sm text-[#64748b]">Check published releases from anrted/opendeploy on GitHub.</p>
          </div>
          <button type="button" class="btn-secondary" :disabled="checkingUpdates" @click="checkUpdates">
            {{ checkingUpdates ? 'Checking…' : 'Check for updates' }}
          </button>
        </div>

        <div v-if="updateError" class="mt-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
          {{ updateError }}
        </div>
        <div v-else-if="updateStatus" class="mt-4 rounded-lg border border-white/10 bg-white/[0.02] p-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div class="text-sm text-[#94a3b8]">
                Installed: <span class="font-mono text-[#e2e8f0]">{{ updateStatus.current_version }}</span>
              </div>
              <div class="mt-1 text-sm text-[#94a3b8]">
                Latest release: <span class="font-mono text-[#e2e8f0]">{{ updateStatus.latest_version }}</span>
              </div>
            </div>
            <span :class="updateStatus.update_available ? 'badge-warning' : 'badge-success'">
              {{ updateStatus.update_available ? 'Update available' : 'Up to date' }}
            </span>
          </div>
          <a v-if="updateStatus.update_available" :href="updateStatus.release_url" target="_blank" rel="noopener noreferrer"
            class="btn-primary mt-4 inline-flex">
            Open release
          </a>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, reactive } from 'vue'
import api, { apiErrorMessage } from '@/api/client'

const settings = reactive({
  'core.panel_title': '',
  'core.default_php': '',
})

const phpVersions = ['8.1', '8.2', '8.3', '8.4']
const saving = ref(false)
const saved = ref(false)
const saveError = ref('')
const updateStatus = ref(null)
const checkingUpdates = ref(false)
const updateError = ref('')

onMounted(async () => {
  checkUpdates()
  try {
    const { data } = await api.get('/settings?ns=core')
    if (Array.isArray(data)) {
      data.forEach(s => { settings[s.key] = s.value })
    }
  } catch (e) {
    console.error(e)
  }
})

async function checkUpdates() {
  checkingUpdates.value = true
  updateError.value = ''
  try {
    const { data } = await api.get('/updates')
    updateStatus.value = data
  } catch (e) {
    updateError.value = apiErrorMessage(e, 'Unable to check GitHub releases')
  } finally {
    checkingUpdates.value = false
  }
}

async function saveSettings() {
  saving.value = true
  saved.value = false
  saveError.value = ''
  try {
    await api.put('/settings', { ...settings })
    saved.value = true
    setTimeout(() => { saved.value = false }, 3000)
  } catch (e) {
    saveError.value = e.response?.data?.error?.message || 'Save failed'
  } finally {
    saving.value = false
  }
}
</script>
