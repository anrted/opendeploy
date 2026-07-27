<template>
  <section class="space-y-6">
    <header class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div><h1 class="text-2xl font-bold">Task Manager</h1><p class="mt-1 text-sm text-text-muted">Background operations and execution history</p></div>
      <button class="btn-secondary" :disabled="loading" @click="load">Refresh</button>
    </header>
    <div class="grid gap-3 rounded-xl border border-border-subtle bg-bg-card p-4 md:grid-cols-3">
      <input v-model.trim="filters.q" class="input" placeholder="Task ID or name…" @input="debouncedLoad" />
      <select v-model="filters.state" class="input" @change="resetAndLoad"><option value="">All statuses</option><option v-for="state in states" :key="state">{{ state }}</option></select>
      <select v-model="filters.type" class="input" @change="resetAndLoad"><option value="">All types</option><option value="install_module">Install module</option><option value="uninstall_module">Uninstall module</option></select>
    </div>
    <div v-if="notice" class="rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-3 text-sm text-emerald-400">{{ notice }}</div>
    <div v-if="error" class="rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-400">{{ error }}</div>
    <div class="overflow-x-auto rounded-xl border border-border-subtle bg-bg-card">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-border-subtle text-xs uppercase text-text-muted"><tr><th class="p-4">Task</th><th class="p-4">Status</th><th class="p-4">Progress</th><th class="p-4">Started</th><th class="p-4">Duration</th><th class="p-4 text-right">Actions</th></tr></thead>
        <tbody>
          <template v-if="loading"><tr v-for="n in 5" :key="n"><td colspan="6" class="p-4"><div class="h-5 animate-pulse rounded bg-slate-500/15"></div></td></tr></template>
          <tr v-else-if="!tasks.length"><td colspan="6" class="p-12 text-center text-text-muted">No task history matches the filters.</td></tr>
          <template v-else><tr v-for="task in tasks" :key="task.id" class="border-b border-border-subtle last:border-0">
            <td class="p-4"><button class="text-left font-medium hover:text-indigo-400" @click="selected = task">{{ task.name }}</button><div class="font-mono text-xs text-text-muted">{{ task.id }}</div></td>
            <td class="p-4"><span class="rounded-full px-2 py-1 text-xs" :class="statusClass(task.state)">{{ task.state }}</span></td>
            <td class="p-4"><div class="h-2 w-28 overflow-hidden rounded bg-slate-500/20"><div class="h-full bg-indigo-500" :style="{ width: `${task.progress}%` }"></div></div><span class="text-xs text-text-muted">{{ task.progress }}%</span></td>
            <td class="p-4 text-text-muted">{{ formatTime(task.started_at || task.created_at) }}</td><td class="p-4 text-text-muted">{{ duration(task) }}</td>
            <td class="p-4"><div class="flex justify-end gap-2"><button v-if="['pending','running'].includes(task.state)" class="btn-secondary text-xs" @click="cancelTask(task)">Cancel</button><button v-if="['error','canceled'].includes(task.state)" class="btn-secondary text-xs" @click="retryTask(task)">Retry</button><button v-if="['success','error','canceled'].includes(task.state)" class="rounded-lg px-3 py-2 text-xs text-red-400 hover:bg-red-500/10" @click="removeTask(task)">Delete</button></div></td>
          </tr></template>
        </tbody>
      </table>
      <footer class="flex items-center justify-between p-4 text-sm text-text-muted"><span>{{ total }} tasks</span><div class="flex gap-2"><button class="btn-secondary" :disabled="offset === 0" @click="page(-1)">Previous</button><button class="btn-secondary" :disabled="offset + limit >= total" @click="page(1)">Next</button></div></footer>
    </div>
    <div v-if="selected" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" @click.self="selected = null">
      <div class="max-h-[80vh] w-full max-w-3xl overflow-auto rounded-2xl border border-border-subtle bg-bg-card p-6"><div class="mb-4 flex justify-between"><div><h2 class="text-xl font-semibold">{{ selected.name }}</h2><p class="font-mono text-xs text-text-muted">{{ selected.id }}</p></div><button class="btn-secondary" @click="selected = null">Close</button></div><h3 class="mb-2 font-medium">Output</h3><pre class="min-h-32 whitespace-pre-wrap rounded-lg bg-black/30 p-4 text-xs">{{ selected.output || 'No output recorded.' }}</pre><template v-if="selected.error"><h3 class="mb-2 mt-4 font-medium text-red-400">Error</h3><pre class="whitespace-pre-wrap rounded-lg bg-red-500/10 p-4 text-xs text-red-300">{{ selected.error }}</pre></template></div>
    </div>
  </section>
</template>

<script setup>
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import api, { apiErrorMessage } from '@/api/client'
import { useConfirmStore } from '@/stores/confirm'
const confirm = useConfirmStore()
const states = ['pending', 'running', 'success', 'error', 'canceled']
const tasks = ref([]), total = ref(0), offset = ref(0), loading = ref(false)
const error = ref(''), notice = ref(''), selected = ref(null)
const filters = reactive({ q: '', state: '', type: '' })
const limit = 25
let debounce, refreshTimer
async function load() { loading.value = true; error.value = ''; try { const { data } = await api.get('/tasks', { params: { ...filters, limit, offset: offset.value } }); tasks.value = data.items || []; total.value = data.total || 0 } catch (e) { error.value = apiErrorMessage(e, 'Unable to load tasks') } finally { loading.value = false } }
function debouncedLoad() { clearTimeout(debounce); offset.value = 0; debounce = setTimeout(load, 250) }
function resetAndLoad() { offset.value = 0; load() }
function page(direction) { offset.value = Math.max(0, offset.value + direction * limit); load() }
function formatTime(value) { return value ? new Date(value).toLocaleString() : '—' }
function duration(task) { if (!task.started_at) return '—'; const end = task.finished_at ? new Date(task.finished_at) : new Date(); const seconds = Math.max(0, Math.round((end - new Date(task.started_at)) / 1000)); return seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m ${seconds % 60}s` }
function statusClass(state) { return { success: 'bg-emerald-500/10 text-emerald-400', error: 'bg-red-500/10 text-red-400', canceled: 'bg-slate-500/10 text-text-muted', running: 'bg-indigo-500/10 text-indigo-400', pending: 'bg-amber-500/10 text-amber-400' }[state] }
async function cancelTask(task) { const ok = await confirm.require({ title: 'Cancel task', message: `Cancel ${task.name}?`, confirmText: 'Cancel task', type: 'danger' }); if (!ok) return; try { await api.post(`/tasks/${task.id}/cancel`); notice.value = 'Task canceled.'; load() } catch (e) { error.value = apiErrorMessage(e) } }
async function retryTask(task) { try { await api.post(`/tasks/${task.id}/retry`); notice.value = 'Retry task started.'; load() } catch (e) { error.value = apiErrorMessage(e) } }
async function removeTask(task) { const ok = await confirm.require({ title: 'Delete task history', message: `Delete history for ${task.name}?`, confirmText: 'Delete', type: 'danger' }); if (!ok) return; try { await api.delete(`/tasks/${task.id}`); notice.value = 'Task history deleted.'; load() } catch (e) { error.value = apiErrorMessage(e) } }
onMounted(() => { load(); refreshTimer = setInterval(() => { if (tasks.value.some(task => ['pending', 'running'].includes(task.state))) load() }, 3000) })
onBeforeUnmount(() => { clearTimeout(debounce); clearInterval(refreshTimer) })
</script>
