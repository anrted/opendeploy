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
            <span class="text-[#e2e8f0] font-mono">v0.1.0-alpha</span>
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
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, reactive } from 'vue'
import api from '@/api/client'

const settings = reactive({
  'core.panel_title': '',
  'core.default_php': '',
})

const phpVersions = ['8.1', '8.2', '8.3', '8.4']
const saving = ref(false)
const saved = ref(false)
const saveError = ref('')

onMounted(async () => {
  try {
    const { data } = await api.get('/settings?ns=core')
    if (Array.isArray(data)) {
      data.forEach(s => { settings[s.key] = s.value })
    }
  } catch (e) {
    console.error(e)
  }
})

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
