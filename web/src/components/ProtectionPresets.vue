<template>
  <section class="mb-6" aria-labelledby="protection-presets-title">
    <div class="mb-3 flex items-center justify-between gap-4">
      <div>
        <h2 id="protection-presets-title" class="text-lg font-semibold text-white">{{ t('protectionPresets.title') }}</h2>
        <p class="text-sm text-[#94a3b8]">{{ t('protectionPresets.subtitle') }}</p>
      </div>
      <button class="btn btn-secondary text-xs" :disabled="loading" @click="load">
        <AppIcon name="refresh-cw" class="mr-2 h-4 w-4" />
        {{ t('common.refresh') }}
      </button>
    </div>

    <div v-if="notice" class="mb-3 rounded-lg border px-4 py-3 text-sm"
      :class="notice.type === 'error' ? 'border-red-500/30 bg-red-500/10 text-red-300' : 'border-emerald-500/30 bg-emerald-500/10 text-emerald-300'">
      {{ notice.message }}
    </div>

    <div v-if="loading" class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4" aria-busy="true">
      <div v-for="index in 4" :key="index" class="h-80 animate-pulse rounded-xl border border-[#334155] bg-[#1e293b] p-5">
        <div class="mb-5 h-10 w-10 rounded-lg bg-[#334155]"></div>
        <div class="mb-3 h-5 w-2/3 rounded bg-[#334155]"></div>
        <div class="mb-6 h-9 rounded bg-[#334155]"></div>
        <div class="space-y-3"><div v-for="row in 4" :key="row" class="h-4 rounded bg-[#273449]"></div></div>
      </div>
    </div>

    <div v-else class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
      <article v-for="preset in presets" :key="preset.id"
        class="flex min-h-[21rem] flex-col rounded-xl border bg-[#1e293b] p-5 transition-colors"
        :class="preset.enabled ? 'border-emerald-500/35' : 'border-[#334155]'">
        <div class="mb-3 flex items-start gap-3">
          <div class="rounded-lg p-2" :class="preset.enabled ? 'bg-emerald-500/10 text-emerald-400' : 'bg-slate-500/10 text-slate-400'">
            <AppIcon :name="preset.icon || 'shield'" class="h-5 w-5" />
          </div>
          <div class="min-w-0">
            <h3 class="font-semibold text-white">{{ preset.title }}</h3>
            <p class="mt-1 text-xs leading-5 text-[#94a3b8]">{{ preset.description }}</p>
          </div>
        </div>

        <div class="mb-4 flex items-center gap-2 text-sm font-medium" :class="preset.enabled ? 'text-emerald-400' : 'text-slate-400'">
          <span class="h-2.5 w-2.5 rounded-full" :class="preset.enabled ? 'bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,.6)]' : 'bg-slate-500'"></span>
          {{ preset.enabled ? t('protectionPresets.active') : t('protectionPresets.inactive') }}
          <span v-if="preset.needs_update" class="ml-auto rounded bg-amber-500/10 px-2 py-1 text-[10px] text-amber-300">
            {{ t('protectionPresets.updateAvailable') }}
          </span>
        </div>

        <dl class="mb-4 grid grid-cols-[auto_1fr] gap-x-3 gap-y-2 border-y border-[#334155] py-4 text-xs">
          <dt class="text-[#64748b]">{{ t('protectionPresets.jail') }}</dt>
          <dd class="truncate text-right font-mono text-[#e2e8f0]" :title="preset.jails.join(', ')">{{ preset.jails.join(', ') }}</dd>
          <dt class="text-[#64748b]">{{ t('protectionPresets.log') }}</dt>
          <dd class="truncate text-right font-mono text-[#e2e8f0]" :title="preset.log_paths.join(', ')">{{ shortPath(preset.log_paths[0]) }}</dd>
          <dt class="text-[#64748b]">{{ t('protectionPresets.rule') }}</dt>
          <dd class="text-right text-[#e2e8f0]">{{ ruleSummary(preset) }}</dd>
          <dt class="text-[#64748b]">{{ t('protectionPresets.ban') }}</dt>
          <dd class="text-right text-[#e2e8f0]">{{ preset.settings.bantime }}</dd>
          <dt class="text-[#64748b]">{{ t('protectionPresets.rules') }}</dt>
          <dd class="text-right text-[#e2e8f0]">{{ preset.rule_count }}</dd>
          <dt class="text-[#64748b]">{{ t('protectionPresets.modified') }}</dt>
          <dd class="text-right text-[#e2e8f0]">{{ formatDate(preset.last_modified) }}</dd>
        </dl>

        <div class="mt-auto grid grid-cols-2 gap-2">
          <button class="btn btn-secondary text-xs" @click="openDetails(preset)">{{ t('protectionPresets.details') }}</button>
          <button class="btn btn-secondary text-xs" @click="openSettings(preset)">{{ t('protectionPresets.configure') }}</button>
          <button class="btn col-span-2 text-xs" :class="preset.enabled ? 'btn-warning' : 'btn-success'"
            :disabled="busy === preset.id" @click="confirmToggle(preset)">
            <span v-if="busy === preset.id" class="mr-2 inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent"></span>
            {{ preset.enabled ? t('protectionPresets.disable') : t('protectionPresets.enable') }}
          </button>
        </div>
      </article>
    </div>

    <Teleport to="body">
      <div v-if="selected" class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4" @click.self="closeModal">
        <div class="max-h-[92vh] w-full max-w-3xl overflow-y-auto rounded-2xl border border-[#334155] bg-[#0f172a] shadow-2xl">
          <header class="sticky top-0 z-10 flex items-start justify-between border-b border-[#334155] bg-[#0f172a] p-5">
            <div>
              <h3 class="text-lg font-semibold text-white">{{ selected.title }}</h3>
              <p class="mt-1 text-sm text-[#94a3b8]">{{ mode === 'details' ? t('protectionPresets.detailsTitle') : t('protectionPresets.configureTitle') }}</p>
            </div>
            <button class="rounded-lg p-2 text-[#94a3b8] hover:bg-white/5 hover:text-white" :aria-label="t('common.close')" @click="closeModal">×</button>
          </header>

          <div v-if="mode === 'details'" class="space-y-5 p-5 text-sm">
            <p class="leading-6 text-[#cbd5e1]">{{ detailsText(selected.id) }}</p>
            <InfoList :label="t('protectionPresets.files')" :items="selected.files" mono />
            <InfoList :label="t('protectionPresets.jails')" :items="selected.jails" mono />
            <InfoList :label="t('protectionPresets.filters')" :items="selected.filters" mono />
            <InfoList :label="t('protectionPresets.logPaths')" :items="selected.log_paths" mono />
            <InfoList :label="t('protectionPresets.actions')" :items="selected.actions" mono />
            <InfoList :label="t('protectionPresets.blockedIPs')" :items="[selected.blocked_ips]" />
            <InfoList :label="t('protectionPresets.exceptions')" :items="[selected.exceptions]" />
          </div>

          <form v-else class="p-5" @submit.prevent="save">
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <label v-for="field in fields" :key="field.key" class="block">
                <span class="mb-1.5 block text-xs font-medium text-[#94a3b8]">{{ t(`protectionPresets.fields.${field.key}`) }}</span>
                <input v-if="field.type !== 'select' && field.type !== 'boolean'" v-model="form[field.key]"
                  :type="field.type" class="input w-full" :min="field.min" :max="field.max">
                <select v-else-if="field.type === 'select'" v-model="form[field.key]" class="input w-full">
                  <option v-for="option in field.options" :key="option" :value="option">{{ option }}</option>
                </select>
                <label v-else class="flex h-10 items-center gap-3 rounded-lg border border-[#334155] bg-[#1e293b] px-3">
                  <input v-model="form[field.key]" type="checkbox" class="h-4 w-4">
                  <span class="text-sm text-[#cbd5e1]">{{ form[field.key] ? t('common.enabled') : t('common.disabled') }}</span>
                </label>
              </label>
            </div>
            <p v-if="modalError" class="mt-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{{ modalError }}</p>

            <div v-if="preview" class="mt-5 rounded-xl border border-indigo-500/20 bg-indigo-500/5 p-4">
              <h4 class="mb-3 font-medium text-white">{{ t('protectionPresets.previewTitle') }}</h4>
              <div class="mb-3 grid gap-2 text-xs sm:grid-cols-2">
                <div><span class="text-[#64748b]">{{ t('protectionPresets.jails') }}:</span> <span class="text-[#cbd5e1]">{{ preview.jails.join(', ') }}</span></div>
                <div><span class="text-[#64748b]">{{ t('protectionPresets.services') }}:</span> <span class="text-[#cbd5e1]">{{ preview.services.join(', ') }}</span></div>
                <div class="sm:col-span-2"><span class="text-[#64748b]">{{ t('protectionPresets.files') }}:</span> <span class="font-mono text-[#cbd5e1]">{{ preview.files.join(', ') }}</span></div>
              </div>
              <pre class="max-h-52 overflow-auto whitespace-pre-wrap rounded-lg bg-black/30 p-3 text-xs text-[#cbd5e1]">{{ preview.configuration }}</pre>
            </div>

            <footer class="mt-6 flex flex-wrap justify-end gap-2 border-t border-[#334155] pt-4">
              <button type="button" class="btn btn-secondary" @click="closeModal">{{ t('common.cancel') }}</button>
              <button type="button" class="btn btn-secondary" :disabled="modalBusy" @click="reset">{{ t('protectionPresets.restoreDefaults') }}</button>
              <button type="button" class="btn btn-secondary" :disabled="modalBusy" @click="loadPreview">{{ t('protectionPresets.preview') }}</button>
              <button type="submit" class="btn btn-primary" :disabled="modalBusy">
                {{ modalBusy ? t('protectionPresets.applying') : t('common.save') }}
              </button>
            </footer>
          </form>
        </div>
      </div>
    </Teleport>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import api, { apiErrorMessage } from '@/api/client'
import { useConfirmStore } from '@/stores/confirm'
import AppIcon from '@/components/AppIcon.vue'
import InfoList from '@/components/InfoList.vue'

const props = defineProps({ moduleId: { type: String, required: true } })
const { t, locale } = useI18n()
const confirm = useConfirmStore()
const presets = ref([])
const loading = ref(true)
const busy = ref('')
const selected = ref(null)
const mode = ref('details')
const form = reactive({})
const preview = ref(null)
const modalError = ref('')
const modalBusy = ref(false)
const notice = ref(null)

const fields = [
  { key: 'bantime', type: 'text' }, { key: 'findtime', type: 'text' },
  { key: 'maxretry', type: 'number', min: 1, max: 1000 },
  { key: 'backend', type: 'select', options: ['auto', 'systemd', 'polling'] },
  { key: 'logpath', type: 'text' }, { key: 'port', type: 'text' },
  { key: 'ignoreip', type: 'text' }, { key: 'banaction', type: 'text' },
  { key: 'ipv6', type: 'boolean' }, { key: 'auto_reload', type: 'boolean' },
]

onMounted(load)

async function load() {
  loading.value = true
  try {
    const { data } = await api.get(`/modules/${props.moduleId}/presets`)
    presets.value = data
  } catch (error) {
    showNotice(apiErrorMessage(error, t('protectionPresets.loadFailed')), 'error')
  } finally {
    loading.value = false
  }
}

function ruleSummary(preset) {
  return t('protectionPresets.ruleSummary', {
    attempts: preset.settings.maxretry,
    duration: preset.settings.findtime,
  })
}

function shortPath(path) {
  if (!path) return '—'
  return path.split('/').pop()
}

function formatDate(value) {
  if (!value) return '—'
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'short', timeStyle: 'short' }).format(new Date(value))
}

function detailsText(id) {
  return t(`protectionPresets.presetDetails.${id}`)
}

function openDetails(preset) {
  selected.value = preset
  mode.value = 'details'
}

function openSettings(preset) {
  selected.value = preset
  mode.value = 'settings'
  preview.value = null
  modalError.value = ''
  Object.keys(form).forEach(key => delete form[key])
  Object.assign(form, preset.settings)
}

function closeModal() {
  selected.value = null
  preview.value = null
  modalError.value = ''
}

async function confirmToggle(preset) {
  const enabled = !preset.enabled
  const accepted = await confirm.require({
    title: enabled ? t('protectionPresets.enable') : t('protectionPresets.disable'),
    message: t(enabled ? 'protectionPresets.confirmEnable' : 'protectionPresets.confirmDisable', { name: preset.title }),
    confirmText: enabled ? t('protectionPresets.enable') : t('protectionPresets.disable'),
    type: enabled ? 'warning' : 'danger',
  })
  if (!accepted) return
  busy.value = preset.id
  try {
    await api.post(`/modules/${props.moduleId}/presets/${preset.id}/toggle`, { enabled })
    await load()
    showNotice(t(enabled ? 'protectionPresets.enabledToast' : 'protectionPresets.disabledToast', { name: preset.title }))
  } catch (error) {
    showNotice(apiErrorMessage(error, t('protectionPresets.operationFailed')), 'error')
  } finally {
    busy.value = ''
  }
}

async function loadPreview() {
  modalBusy.value = true
  modalError.value = ''
  try {
    const { data } = await api.post(`/modules/${props.moduleId}/presets/${selected.value.id}/preview`, form)
    preview.value = data
  } catch (error) {
    modalError.value = apiErrorMessage(error, t('protectionPresets.validationFailed'))
  } finally {
    modalBusy.value = false
  }
}

async function save() {
  modalBusy.value = true
  modalError.value = ''
  try {
    await api.put(`/modules/${props.moduleId}/presets/${selected.value.id}`, form)
    const name = selected.value.title
    closeModal()
    await load()
    showNotice(t('protectionPresets.savedToast', { name }))
  } catch (error) {
    modalError.value = apiErrorMessage(error, t('protectionPresets.saveFailed'))
  } finally {
    modalBusy.value = false
  }
}

async function reset() {
  const accepted = await confirm.require({
    title: t('protectionPresets.restoreDefaults'),
    message: t('protectionPresets.confirmReset', { name: selected.value.title }),
    confirmText: t('protectionPresets.restoreDefaults'),
    type: 'warning',
  })
  if (!accepted) return
  modalBusy.value = true
  try {
    const name = selected.value.title
    await api.post(`/modules/${props.moduleId}/presets/${selected.value.id}/reset`)
    closeModal()
    await load()
    showNotice(t('protectionPresets.resetToast', { name }))
  } catch (error) {
    modalError.value = apiErrorMessage(error, t('protectionPresets.saveFailed'))
  } finally {
    modalBusy.value = false
  }
}

function showNotice(message, type = 'success') {
  notice.value = { message, type }
  window.setTimeout(() => {
    if (notice.value?.message === message) notice.value = null
  }, 5000)
}
</script>
