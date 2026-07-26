<template>
  <div>
    <div class="mb-6">
      <h1 class="page-title">{{ $t('settings.title') }}</h1>
      <p class="page-subtitle">{{ $t('settings.subtitle') }}</p>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <div class="card">
        <h2 class="text-base font-semibold text-white mb-4">{{ $t('settings.general') }}</h2>
        
        <div v-if="loadingSpecs" class="flex justify-center py-10">
          <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-indigo-500"></div>
        </div>
        
        <form v-else @submit.prevent="saveSettings" class="space-y-4">
          <div v-for="spec in coreSpecs" :key="spec.key" class="space-y-1">
            <label :for="spec.key" class="label">{{ spec.label }}</label>
            <p v-if="spec.description" class="text-xs text-[#94a3b8] mb-2">{{ spec.description }}</p>
            
            <template v-if="spec.type === 'string'">
              <input 
                :id="spec.key" 
                :type="spec.secret ? 'password' : 'text'"
                v-model="settings[spec.key]" 
                class="input" 
                :placeholder="spec.default_value"
                @input="validateField(spec)"
                @blur="validateField(spec)"
              />
            </template>
            
            <template v-else-if="spec.type === 'select'">
              <select :id="spec.key" v-model="settings[spec.key]" class="input" @change="validateField(spec)">
                <option v-for="opt in spec.options" :key="opt" :value="opt">{{ opt || 'None' }}</option>
              </select>
            </template>
            
            <template v-else-if="spec.type === 'int'">
              <input 
                :id="spec.key" 
                type="number"
                v-model="settings[spec.key]" 
                class="input" 
                :placeholder="spec.default_value"
                @input="validateField(spec)"
                @blur="validateField(spec)"
              />
            </template>
            
            <template v-else-if="spec.type === 'bool'">
              <label class="flex items-center gap-2 cursor-pointer">
                <input 
                  type="checkbox" 
                  :checked="settings[spec.key] === 'true'"
                  @change="e => { settings[spec.key] = e.target.checked ? 'true' : 'false'; validateField(spec) }"
                  class="rounded bg-[#1e293b] border-[#334155] text-indigo-500 focus:ring-indigo-500/30 w-4 h-4" 
                />
                <span class="text-sm text-[#e2e8f0]">{{ spec.label }}</span>
              </label>
            </template>
            
            <div v-if="validationErrors[spec.key]" class="text-xs text-red-400 mt-1">
              {{ validationErrors[spec.key] }}
            </div>
          </div>

          <div v-if="saved" class="text-sm text-emerald-400 mt-4">{{ $t('settings.saved') }}</div>
          <div v-if="saveError" class="text-sm text-red-400 mt-4">{{ saveError }}</div>

          <button id="save-settings-btn" type="submit" class="btn-primary mt-4" :disabled="saving || hasErrors">
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
import { ref, onMounted, reactive, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import api, { apiErrorMessage } from '@/api/client'
import { useConfirmStore } from '@/stores/confirm'

const { t } = useI18n()
const confirm = useConfirmStore()

const settings = reactive({})
const validationErrors = reactive({})
const specs = ref([])
const loadingSpecs = ref(true)

const coreSpecs = computed(() => {
  return specs.value.filter(s => s.key.startsWith('core.'))
})

const hasErrors = computed(() => {
  return Object.values(validationErrors).some(err => err !== '')
})

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
  await loadSpecs()
  await loadSettings()
})

async function loadSpecs() {
  loadingSpecs.value = true
  try {
    const { data } = await api.get('/settings/specs')
    if (Array.isArray(data)) {
      specs.value = data
      // Initialize settings object with empty/default values
      data.forEach(s => {
        if (settings[s.key] === undefined) {
          settings[s.key] = s.default_value || ''
        }
      })
    }
  } catch (e) {
    console.error('Failed to load specs', e)
  } finally {
    loadingSpecs.value = false
  }
}

async function loadSettings() {
  try {
    const { data } = await api.get('/settings?ns=core')
    if (Array.isArray(data)) {
      data.forEach(s => { 
        settings[s.key] = s.value 
      })
    }
    // Validate initially
    coreSpecs.value.forEach(spec => validateField(spec))
  } catch (e) {
    console.error(e)
  }
}

function validateField(spec) {
  const val = settings[spec.key]
  
  if (spec.required && !val) {
    validationErrors[spec.key] = 'This field is required.'
    return
  }
  
  if (val && spec.validation_regex) {
    const regex = new RegExp(spec.validation_regex)
    if (!regex.test(val)) {
      validationErrors[spec.key] = spec.validation_msg || 'Invalid format.'
      return
    }
  }
  
  validationErrors[spec.key] = ''
}

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
    const confirmed = await confirm.require({
      title: 'Developer Update',
      message: 'Warning: Dev builds may be unstable. Continue?',
      confirmText: 'Continue',
      type: 'warning'
    })
    if (!confirmed) return
  } else {
    const confirmed = await confirm.require({
      title: 'System Update',
      message: 'Update OpenDeploy from the trusted GitHub repository? Services will restart.',
      confirmText: 'Update',
      type: 'warning'
    })
    if (!confirmed) return
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
  // Final validation before save
  coreSpecs.value.forEach(spec => validateField(spec))
  if (hasErrors.value) return
  
  saving.value = true
  saved.value = false
  saveError.value = ''
  try {
    const payload = {}
    coreSpecs.value.forEach(s => {
      payload[s.key] = settings[s.key]
    })
    
    await api.put('/settings', payload)
    saved.value = true
    setTimeout(() => { saved.value = false }, 3000)
  } catch (e) {
    saveError.value = e.response?.data?.error?.message || 'Save failed'
  } finally {
    saving.value = false
  }
}
</script>
