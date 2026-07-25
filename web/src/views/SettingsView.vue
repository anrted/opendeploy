<template>
  <div>
    <div class="mb-6">
      <h1 class="page-title">{{ $t('settings.title') }}</h1>
      <p class="page-subtitle">{{ $t('settings.subtitle') }}</p>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <div class="card">
        <h2 class="text-base font-semibold text-white mb-4">{{ $t('settings.general') }}</h2>
        <form @submit.prevent="saveSettings" class="space-y-4">
          <div>
            <label class="label">{{ $t('settings.panelTitle') }}</label>
            <input id="setting-title" v-model="settings['core.panel_title']" class="input" placeholder="OpenDeploy" />
          </div>
          <div>
            <label class="label">{{ $t('settings.defaultPhp') }}</label>
            <select v-model="settings['core.default_php']" class="input">
              <option value="">None</option>
              <option v-for="v in phpVersions" :key="v" :value="v">{{ v }}</option>
            </select>
          </div>

          <div v-if="saved" class="text-sm text-emerald-400">{{ $t('settings.saved') }}</div>
          <div v-if="saveError" class="text-sm text-red-400">{{ saveError }}</div>

          <button id="save-settings-btn" type="submit" class="btn-primary" :disabled="saving">
            {{ saving ? $t('settings.saving') : $t('settings.saveSettings') }}
          </button>
        </form>
      </div>

      <div class="card">
        <h2 class="text-base font-semibold text-white mb-4">{{ $t('settings.about') }}</h2>
        <div class="space-y-3 text-sm">
          <div class="flex justify-between">
            <span class="text-[#64748b]">{{ $t('settings.version') }}</span>
            <span class="text-[#e2e8f0] font-mono">{{ updateStatus?.current_version || 'unknown' }}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-[#64748b]">{{ $t('settings.license') }}</span>
            <span class="text-[#e2e8f0]">MIT</span>
          </div>
          <div class="flex justify-between">
            <span class="text-[#64748b]">{{ $t('settings.github') }}</span>
            <a href="https://github.com/anrted/opendeploy" class="text-indigo-400 hover:text-indigo-300">
              anrted/opendeploy
            </a>
          </div>
        </div>
      </div>

      <div class="card lg:col-span-2">
        <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h2 class="text-base font-semibold text-white">{{ $t('settings.updates.title') }}</h2>
            <p class="mt-1 text-sm text-[#64748b]">{{ $t('settings.updates.desc') }}</p>
          </div>
          <button type="button" class="btn-secondary" :disabled="checkingUpdates" @click="checkUpdates">
            {{ checkingUpdates ? $t('settings.updates.checking') : $t('settings.updates.check') }}
          </button>
        </div>

        <div v-if="updateError" class="mt-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
          {{ updateError }}
        </div>
        <div v-else-if="updateStatus" class="mt-4 rounded-lg border border-white/10 bg-white/[0.02] p-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div class="text-sm text-[#94a3b8]">
                {{ $t('settings.updates.installed') }}: <span class="font-mono text-[#e2e8f0]">{{ updateStatus.current_version }}</span>
              </div>
              <div class="mt-1 text-sm text-[#94a3b8]">
                {{ $t('settings.updates.latest') }}: <span class="font-mono text-[#e2e8f0]">{{ updateStatus.latest_version }}</span>
              </div>
            </div>
            <span :class="updateStatus.update_available ? 'badge-warning' : 'badge-success'">
              {{ updateStatus.update_available ? $t('settings.updates.available') : $t('settings.updates.upToDate') }}
            </span>
          </div>
          <div class="mt-4 flex flex-wrap gap-3">
            <button v-if="updateStatus.update_available" type="button" class="btn-primary" :disabled="applyingUpdate" @click="applyUpdate('stable')">
              {{ applyingUpdate ? $t('settings.updates.updating') : 'Обновить до стабильной версии' }}
            </button>
            <button type="button" class="btn-secondary" :disabled="applyingUpdate" @click="applyUpdate('dev')">
              Обновить до тестовой сборки
            </button>
            <a :href="updateStatus.update_url || updateStatus.release_url" target="_blank" rel="noopener noreferrer" class="btn-secondary inline-flex">
              {{ $t('settings.updates.viewChanges') }}
            </a>
            <button v-if="applyingUpdate" type="button" class="btn-secondary" @click="cancelUpdate">
              {{ $t('settings.updates.cancel') }}
            </button>
          </div>
          <div v-if="updateMessage" class="mt-4 text-sm text-emerald-400">{{ updateMessage }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import api, { apiErrorMessage } from '@/api/client'

const { t } = useI18n()

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
const applyingUpdate = ref(false)
const updateMessage = ref('')

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

async function applyUpdate(updateType = 'stable') {
  if (updateType === 'dev') {
    if (!confirm('Warning: Dev builds may be unstable. Continue?')) return
  } else {
    if (!confirm('Update OpenDeploy from the trusted GitHub repository? Services will restart.')) return
  }
  applyingUpdate.value = true
  updateError.value = ''
  updateMessage.value = ''
  updateCancel.value = false
  const targetCommit = updateStatus.value?.latest_commit
  try {
    await api.post('/updates/apply', { type: updateType })
    updateMessage.value = t('settings.updates.started')
    await waitForUpdatedCore(targetCommit)
  } catch (e) {
    updateError.value = apiErrorMessage(e, 'Unable to start update')
    applyingUpdate.value = false
  }
}

const updateCancel = ref(false)
function cancelUpdate() {
  updateCancel.value = true
}

async function waitForUpdatedCore(targetCommit) {
  const deadline = Date.now() + 20 * 60 * 1000
  const checkInterval = 3000
  let checks = 0
  
  while (Date.now() < deadline) {
    if (updateCancel.value) {
      applyingUpdate.value = false
      updateMessage.value = ''
      updateError.value = 'Update cancelled by user.'
      return
    }

    await new Promise(resolve => setTimeout(resolve, checkInterval))
    checks++
    try {
      const { data } = await api.get('/updates')
      if (targetCommit && data.current_commit === targetCommit) {
        location.reload()
        return
      }
      
      // If we made several checks (e.g. 15 seconds) and the API is still responding 
      // without updating, it means the update system service didn't restart the backend.
      if (checks > 5) {
        applyingUpdate.value = false
        updateMessage.value = ''
        updateError.value = t('settings.updates.timeout')
        return
      }
    } catch {
      // A short outage is expected while Core and Agent restart.
      // Reset checks because the backend actually went offline!
      checks = 0
    }
  }
  applyingUpdate.value = false
  updateError.value = 'Update did not finish within 20 minutes. Check the opendeploy-update service logs.'
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
