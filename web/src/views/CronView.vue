<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-bold text-text-main">Cron Scheduler</h1>
        <p class="mt-1 text-sm text-text-muted">Manage recurring tasks and inspect execution history.</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button class="btn-secondary" @click="exportJobs('json')">Export</button>
        <label class="btn-secondary cursor-pointer">
          Import
          <input class="hidden" type="file" accept=".json,.yaml,.yml,.txt" @change="importJobs" />
        </label>
        <button class="btn-primary" @click="openCreate()">New Cron Job</button>
      </div>
    </div>

    <div v-if="notice" class="rounded-lg border px-4 py-3 text-sm" :class="noticeType === 'error' ? 'border-red-500/40 bg-red-500/10 text-red-400' : 'border-emerald-500/40 bg-emerald-500/10 text-emerald-400'">
      {{ notice }}
    </div>

    <div class="card flex flex-col gap-3 p-4 lg:flex-row">
      <input v-model="search" class="input flex-1" placeholder="Search name, command, user…" />
      <select v-model="statusFilter" class="input lg:w-44">
        <option value="">All types & statuses</option>
        <option value="enabled">Enabled</option>
        <option value="disabled">Disabled</option>
        <option value="SYSTEM">System</option>
        <option value="PACKAGE">Package</option>
        <option value="OPENDEPLOY">OpenDeploy</option>
        <option value="USER">User</option>
      </select>
      <select v-model="sortBy" class="input lg:w-48">
        <option value="name">Sort by name</option>
        <option value="user">Sort by user</option>
        <option value="expression">Sort by schedule</option>
        <option value="updated_at">Sort by updated</option>
      </select>
      <button class="btn-secondary" :disabled="loading" @click="load">{{ loading ? 'Refreshing…' : 'Refresh' }}</button>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="index in 5" :key="index" class="h-20 animate-pulse rounded-xl bg-bg-card"></div>
    </div>

    <div v-else-if="!filteredJobs.length" class="card p-12 text-center">
      <div class="text-4xl">◷</div>
      <h2 class="mt-4 text-lg font-semibold text-text-main">No Cron jobs found</h2>
      <p class="mt-1 text-sm text-text-muted">Create a job or import an existing crontab.</p>
      <button class="btn-primary mt-5" @click="openCreate()">Create first job</button>
    </div>

    <div v-else class="card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full min-w-[1050px] text-left text-sm">
          <thead class="border-b border-border-subtle text-xs uppercase text-text-muted">
            <tr>
              <th class="px-4 py-3">Name</th>
              <th class="px-4 py-3">Command</th>
              <th class="px-4 py-3">User</th>
              <th class="px-4 py-3">Schedule</th>
              <th class="px-4 py-3">Status</th>
              <th class="px-4 py-3">Source</th>
              <th class="px-4 py-3">Updated</th>
              <th class="px-4 py-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border-subtle">
            <tr v-for="job in pagedJobs" :key="job.id" class="hover:bg-white/[0.02]">
              <td class="px-4 py-3">
                <div class="font-medium text-text-main flex items-center gap-1">
                  <span v-if="job.type === 'USER'" title="User Job">🟢</span>
                  <span v-else-if="job.type === 'OPENDEPLOY'" title="OpenDeploy Job">🔵</span>
                  <span v-else-if="job.type === 'PACKAGE'" title="Package Job">🟡</span>
                  <span v-else-if="job.type === 'SYSTEM'" title="System Job">🔴</span>
                  {{ job.name }}
                  <span v-if="job.is_protected" title="Protected" class="text-xs">🔒</span>
                </div>
                <div class="max-w-52 truncate text-xs text-text-muted">{{ job.description || job.id }}</div>
              </td>
              <td class="max-w-xs px-4 py-3"><code class="block truncate text-xs text-indigo-300">{{ job.command }}</code></td>
              <td class="px-4 py-3 text-text-muted">{{ job.user }}</td>
              <td class="px-4 py-3">
                <code class="text-text-main">{{ job.expression }}</code>
                <div class="text-xs text-text-muted">{{ job.timezone || 'Server timezone' }}</div>
              </td>
              <td class="px-4 py-3"><span class="badge" :class="job.enabled ? 'badge-success' : 'badge-muted'">{{ job.enabled ? 'Enabled' : 'Disabled' }}</span></td>
              <td class="px-4 py-3">
                <div class="flex flex-col gap-1">
                  <span class="badge badge-muted">{{ job.source || 'OpenDeploy' }}</span>
                  <span v-if="job.package_name" class="text-[10px] text-text-muted">pkg: {{ job.package_name }}</span>
                </div>
              </td>
              <td class="px-4 py-3 text-text-muted">
                {{ formatDate(job.updated_at) }}
                <div v-if="job.lock_reason" class="text-[10px] text-orange-300 truncate max-w-xs" :title="job.lock_reason">{{ job.lock_reason }}</div>
              </td>
              <td class="px-4 py-3">
                <div class="flex justify-end gap-1">
                  <button class="btn-icon" title="Run now" :disabled="busy === job.id || !job.can_edit" @click="run(job)">▶</button>
                  <button class="btn-icon" title="History" @click="showHistory(job)">☷</button>
                  <button class="btn-icon" title="Edit" :disabled="!job.can_edit" @click="openEdit(job)">✎</button>
                  <button class="btn-icon" :disabled="!job.can_edit" :title="job.enabled ? 'Disable' : 'Enable'" @click="toggle(job)">{{ job.enabled ? 'Ⅱ' : '●' }}</button>
                  <button class="btn-icon" title="Duplicate" :disabled="!job.can_edit" @click="duplicate(job)">⧉</button>
                  <button class="btn-icon text-red-400" title="Delete" :disabled="!job.can_delete" @click="remove(job)">×</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="flex items-center justify-between border-t border-border-subtle px-4 py-3 text-sm text-text-muted">
        <span>{{ filteredJobs.length }} jobs</span>
        <div class="flex items-center gap-2">
          <button class="btn-secondary" :disabled="page === 1" @click="page--">Previous</button>
          <span>{{ page }} / {{ pageCount }}</span>
          <button class="btn-secondary" :disabled="page === pageCount" @click="page++">Next</button>
        </div>
      </div>
    </div>

    <div v-if="editorOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4" @click.self="closeEditor">
      <form class="card max-h-[92vh] w-full max-w-3xl overflow-y-auto p-6" @submit.prevent="save">
        <div class="flex items-start justify-between">
          <div><h2 class="text-xl font-semibold text-text-main">{{ editing ? 'Edit Cron Job' : 'Create Cron Job' }}</h2><p class="text-sm text-text-muted">Changes are validated and written atomically.</p></div>
          <button type="button" class="btn-icon" @click="closeEditor">×</button>
        </div>

        <div v-if="!editing" class="mt-5">
          <label class="label">Template</label>
          <select class="input w-full" @change="applyTemplate($event.target.value)">
            <option value="">Custom job</option>
            <option v-for="template in templates" :key="template.id" :value="template.id">{{ template.name }}</option>
          </select>
        </div>

        <div class="mt-5 grid gap-4 sm:grid-cols-2">
          <label><span class="label">Name *</span><input v-model.trim="form.name" required maxlength="128" class="input w-full" /></label>
          <label><span class="label">ID *</span><input v-model.trim="form.id" required pattern="[A-Za-z0-9][A-Za-z0-9_-]{0,63}" :disabled="editing" class="input w-full" /></label>
          <label class="sm:col-span-2"><span class="label">Description</span><input v-model.trim="form.description" class="input w-full" /></label>
          <label class="sm:col-span-2"><span class="label">Command *</span><textarea v-model="form.command" required rows="3" class="input w-full font-mono"></textarea></label>
          <label><span class="label">Working directory</span><input v-model.trim="form.working_dir" placeholder="/var/www/example" class="input w-full" /></label>
          <label><span class="label">User *</span><input v-model.trim="form.user" required placeholder="www-data" class="input w-full" /></label>
        </div>

        <div class="mt-5 rounded-xl border border-border-subtle p-4">
          <div class="flex flex-wrap items-end gap-3">
            <label class="flex-1"><span class="label">Schedule builder</span><select v-model="preset" class="input w-full" @change="applyPreset">
              <option value="custom">Custom</option><option value="minute">Every minute</option><option value="hour">Every hour</option>
              <option value="day">Every day</option><option value="week">Every week</option><option value="month">Every month</option><option value="year">Every year</option>
            </select></label>
            <label v-if="['hour','day','week','month','year'].includes(preset)"><span class="label">Minute</span><input v-model.number="builder.minute" min="0" max="59" type="number" class="input w-24" @input="applyPreset" /></label>
            <label v-if="['day','week','month','year'].includes(preset)"><span class="label">Hour</span><input v-model.number="builder.hour" min="0" max="23" type="number" class="input w-24" @input="applyPreset" /></label>
          </div>
          <label class="mt-4 block"><span class="label">Cron expression *</span><input v-model.trim="form.expression" required class="input w-full font-mono text-lg" placeholder="0 3 * * *" /></label>
          <p class="mt-2 text-xs text-text-muted">Minute · Hour · Day of month · Month · Day of week</p>
        </div>

        <div class="mt-5 grid gap-4 sm:grid-cols-2">
          <label><span class="label">Timezone</span><input v-model.trim="form.timezone" placeholder="Europe/Moscow" class="input w-full" /></label>
          <label><span class="label">Environment (KEY=value per line)</span><textarea v-model="environmentText" rows="3" class="input w-full font-mono"></textarea></label>
        </div>
        <label class="mt-4 flex items-center gap-2 text-sm text-text-main"><input v-model="form.enabled" type="checkbox" /> Enabled</label>
        <label v-if="!editing" class="mt-2 flex items-center gap-2 text-sm text-text-main"><input v-model="runAfterCreate" type="checkbox" /> Run after creation</label>

        <div v-if="validation" class="mt-4 rounded-lg border p-3 text-sm" :class="validation.valid ? 'border-emerald-500/40 text-emerald-400' : 'border-red-500/40 text-red-400'">
          {{ validation.valid ? 'Validation passed.' : validation.error }}
          <div v-for="warning in validation.warnings" :key="warning">Warning: {{ warning }}</div>
        </div>
        <div class="mt-6 flex justify-end gap-2">
          <button type="button" class="btn-secondary" @click="validate">Validate</button>
          <button type="button" class="btn-secondary" @click="closeEditor">Cancel</button>
          <button class="btn-primary" :disabled="saving">{{ saving ? 'Saving…' : 'Save job' }}</button>
        </div>
      </form>
    </div>

    <div v-if="historyOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4" @click.self="historyOpen = false">
      <div class="card max-h-[85vh] w-full max-w-4xl overflow-y-auto p-6">
        <div class="flex justify-between"><div><h2 class="text-xl font-semibold">{{ selectedJob?.name }} history</h2><p class="text-sm text-text-muted">Automatically refreshed every 5 seconds.</p></div><button class="btn-icon" @click="historyOpen = false">×</button></div>
        <div v-if="!history.length" class="py-12 text-center text-text-muted">No executions recorded.</div>
        <div v-for="runItem in history" :key="runItem.id" class="mt-4 rounded-xl border border-border-subtle p-4">
          <div class="flex flex-wrap justify-between gap-2"><span class="badge" :class="runItem.exit_code === 0 ? 'badge-success' : 'badge-danger'">{{ runItem.exit_code === 0 ? 'Success' : 'Failed' }}</span><span class="text-xs text-text-muted">{{ formatDate(runItem.started_at) }} · {{ runItem.duration / 1000000 }} ms · {{ runItem.triggered }}</span></div>
          <pre v-if="runItem.stdout" class="mt-3 max-h-48 overflow-auto rounded-lg bg-black/30 p-3 text-xs text-emerald-300">{{ runItem.stdout }}</pre>
          <pre v-if="runItem.stderr" class="mt-3 max-h-48 overflow-auto rounded-lg bg-black/30 p-3 text-xs text-red-300">{{ runItem.stderr }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import api, { apiErrorMessage } from '@/api/client'
import { useConfirmStore } from '@/stores/confirm'

const confirm = useConfirmStore()
const jobs = ref([]), templates = ref([]), history = ref([])
const loading = ref(true), saving = ref(false), busy = ref('')
const search = ref(''), statusFilter = ref(''), sortBy = ref('name'), page = ref(1)
const pageSize = 10
const notice = ref(''), noticeType = ref('success')
const editorOpen = ref(false), editing = ref(false), historyOpen = ref(false), selectedJob = ref(null)
const validation = ref(null), runAfterCreate = ref(false), environmentText = ref('')
const preset = ref('custom'), builder = ref({ minute: 0, hour: 3 })
let historyTimer

const blank = () => ({ id: '', name: '', description: '', command: '', working_dir: '', user: 'www-data', environment: {}, expression: '0 3 * * *', timezone: '', enabled: true })
const form = ref(blank())
const filteredJobs = computed(() => jobs.value.filter(job => {
  const text = `${job.name} ${job.command} ${job.user} ${job.description} ${job.type} ${job.package_name}`.toLowerCase()
  let matchStatus = true;
  if (statusFilter.value === 'enabled') matchStatus = job.enabled;
  else if (statusFilter.value === 'disabled') matchStatus = !job.enabled;
  else if (statusFilter.value) matchStatus = job.type === statusFilter.value;
  return text.includes(search.value.toLowerCase()) && matchStatus;
}).sort((a, b) => String(a[sortBy.value] || '').localeCompare(String(b[sortBy.value] || ''))))
const pageCount = computed(() => Math.max(1, Math.ceil(filteredJobs.value.length / pageSize)))
const pagedJobs = computed(() => filteredJobs.value.slice((page.value - 1) * pageSize, page.value * pageSize))
watch([search, statusFilter, sortBy], () => { page.value = 1 })
watch(pageCount, value => { if (page.value > value) page.value = value })

onMounted(async () => { await Promise.all([load(), loadTemplates()]) })
onBeforeUnmount(() => clearInterval(historyTimer))

async function load() { loading.value = true; try { jobs.value = (await api.get('/modules/cron/jobs')).data || [] } catch (e) { show(apiErrorMessage(e), 'error') } finally { loading.value = false } }
async function loadTemplates() { try { templates.value = (await api.get('/modules/cron/templates')).data || [] } catch { templates.value = [] } }
function openCreate() { editing.value = false; form.value = blank(); environmentText.value = ''; validation.value = null; runAfterCreate.value = false; preset.value = 'custom'; editorOpen.value = true }
function openEdit(job) { editing.value = true; form.value = JSON.parse(JSON.stringify(job)); environmentText.value = Object.entries(job.environment || {}).map(([key, value]) => `${key}=${value}`).join('\n'); validation.value = null; preset.value = 'custom'; editorOpen.value = true }
function closeEditor() { editorOpen.value = false }
function applyTemplate(id) { const item = templates.value.find(value => value.id === id); if (!item) return; Object.assign(form.value, { id: item.id, name: item.name, description: item.description, command: item.command, expression: item.expression, user: item.user }); preset.value = 'custom' }
function applyPreset() { const m = builder.value.minute, h = builder.value.hour; const map = { minute: '* * * * *', hour: `${m} * * * *`, day: `${m} ${h} * * *`, week: `${m} ${h} * * 0`, month: `${m} ${h} 1 * *`, year: `${m} ${h} 1 1 *` }; if (map[preset.value]) form.value.expression = map[preset.value] }
function payload() { const environment = {}; environmentText.value.split('\n').forEach(line => { const index = line.indexOf('='); if (index > 0) environment[line.slice(0, index).trim()] = line.slice(index + 1).trim() }); return { ...form.value, environment } }
async function validate() { try { validation.value = (await api.post('/modules/cron/validate', payload())).data } catch (e) { validation.value = { valid: false, error: apiErrorMessage(e) } } }
async function save() { saving.value = true; try { await validate(); if (!validation.value?.valid) return; const data = payload(); if (editing.value) await api.put(`/modules/cron/jobs/${data.id}`, data); else { await api.post('/modules/cron/jobs', data); if (runAfterCreate.value) await api.post(`/modules/cron/jobs/${data.id}/run`) } closeEditor(); show('Cron job saved.'); await load() } catch (e) { show(apiErrorMessage(e), 'error') } finally { saving.value = false } }
async function run(job) { if (!await confirm.require({ title: 'Run Cron job', message: `Run “${job.name}” now?`, confirmText: 'Run', type: 'warning' })) return; busy.value = job.id; try { await api.post(`/modules/cron/jobs/${job.id}/run`); show('Job completed.'); await load() } catch (e) { show(apiErrorMessage(e), 'error') } finally { busy.value = '' } }
async function toggle(job) { try { await api.post(`/modules/cron/jobs/${job.id}/${job.enabled ? 'disable' : 'enable'}`); show(`Job ${job.enabled ? 'disabled' : 'enabled'}.`); await load() } catch (e) { show(apiErrorMessage(e), 'error') } }
async function duplicate(job) { try { await api.post(`/modules/cron/jobs/${job.id}/duplicate`); show('Job duplicated.'); await load() } catch (e) { show(apiErrorMessage(e), 'error') } }
async function remove(job) { if (!await confirm.require({ title: 'Delete Cron job', message: `Delete “${job.name}”?`, confirmText: 'Delete', type: 'danger' })) return; try { await api.delete(`/modules/cron/jobs/${job.id}`); show('Job deleted.'); await load() } catch (e) { show(apiErrorMessage(e), 'error') } }
async function showHistory(job) { selectedJob.value = job; historyOpen.value = true; await refreshHistory(); clearInterval(historyTimer); historyTimer = setInterval(refreshHistory, 5000) }
async function refreshHistory() { if (!selectedJob.value || !historyOpen.value) return; try { history.value = (await api.get(`/modules/cron/jobs/${selectedJob.value.id}/history?limit=100`)).data || [] } catch (e) { show(apiErrorMessage(e), 'error') } }
async function exportJobs(format) { try { const response = await api.get(`/modules/cron/export?format=${format}`, { responseType: 'blob' }); const url = URL.createObjectURL(response.data); const link = document.createElement('a'); link.href = url; link.download = `opendeploy-cron.${format}`; link.click(); URL.revokeObjectURL(url) } catch (e) { show(apiErrorMessage(e), 'error') } }
async function importJobs(event) { const file = event.target.files?.[0]; if (!file) return; const extension = file.name.split('.').pop().toLowerCase(); const format = extension === 'txt' ? 'crontab' : extension; try { await api.post(`/modules/cron/import?format=${format}`, await file.text(), { headers: { 'Content-Type': 'text/plain' } }); show('Cron jobs imported.'); await load() } catch (e) { show(apiErrorMessage(e), 'error') } finally { event.target.value = '' } }
function show(message, type = 'success') { notice.value = message; noticeType.value = type; setTimeout(() => { if (notice.value === message) notice.value = '' }, 5000) }
function formatDate(value) { if (!value || String(value).startsWith('0001-')) return 'Never'; return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) }
</script>
