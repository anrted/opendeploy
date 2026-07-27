<template>
  <section class="space-y-6">
    <header class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-bold text-text-main">{{ t('users.title') }}</h1>
        <p class="mt-1 text-sm text-text-muted">{{ t('users.subtitle') }}</p>
      </div>
      <button class="btn-primary" @click="openCreate">{{ t('users.create') }}</button>
    </header>

    <div class="grid gap-3 rounded-xl border border-border-subtle bg-bg-card p-4 md:grid-cols-4">
      <input v-model="filters.q" class="input" :placeholder="t('users.search')" @input="debouncedLoad" />
      <select v-model="filters.role" class="input" @change="load"><option value="">{{ t('users.allRoles') }}</option><option v-for="role in roles" :key="role">{{ role }}</option></select>
      <select v-model="filters.status" class="input" @change="load"><option value="">{{ t('users.allStatuses') }}</option><option value="active">{{ t('common.active') }}</option><option value="blocked">{{ t('common.blocked') }}</option></select>
      <button class="btn-secondary" :disabled="loading" @click="load">{{ t('common.refresh') }}</button>
    </div>

    <div v-if="notice" class="rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-3 text-sm text-emerald-400">{{ notice }}</div>
    <div v-if="error" class="rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-400">{{ error }}</div>

    <div class="overflow-x-auto rounded-xl border border-border-subtle bg-bg-card">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-border-subtle text-xs uppercase text-text-muted"><tr><th class="p-4">{{ t('users.user') }}</th><th class="p-4">{{ t('users.role') }}</th><th class="p-4">{{ t('users.status') }}</th><th class="p-4">{{ t('users.lastLogin') }}</th><th class="p-4 text-right">{{ t('common.actions') }}</th></tr></thead>
        <tbody>
          <template v-if="loading"><tr v-for="n in 4" :key="n" class="border-b border-border-subtle"><td colspan="5" class="p-4"><div class="h-5 animate-pulse rounded bg-slate-500/15"></div></td></tr></template>
          <tr v-else-if="!users.length"><td colspan="5" class="p-12 text-center text-text-muted">{{ t('users.empty') }}</td></tr>
          <template v-else><tr v-for="user in users" :key="user.id" class="border-b border-border-subtle last:border-0">
            <td class="p-4"><div class="font-medium text-text-main">{{ user.username }}</div><div class="text-xs text-text-muted">{{ user.email }}</div></td>
            <td class="p-4 capitalize">{{ user.role }}</td>
            <td class="p-4"><span class="rounded-full px-2 py-1 text-xs" :class="user.is_active ? 'bg-emerald-500/10 text-emerald-400' : 'bg-red-500/10 text-red-400'">{{ user.is_active ? t('common.active') : t('common.blocked') }}</span></td>
            <td class="p-4 text-text-muted">{{ user.last_login ? new Date(user.last_login).toLocaleString() : t('common.never') }}</td>
            <td class="p-4"><div class="flex justify-end gap-2">
              <button class="btn-secondary text-xs" @click="openEdit(user)">{{ t('common.edit') }}</button>
              <button class="btn-secondary text-xs" @click="changePassword(user)">{{ t('users.password') }}</button>
              <button class="btn-secondary text-xs" @click="showAudit(user)">{{ t('users.history') }}</button>
              <button class="btn-secondary text-xs" :disabled="user.id === auth.user?.id" @click="toggleActive(user)">{{ user.is_active ? t('users.block') : t('users.unblock') }}</button>
              <button class="rounded-lg px-3 py-2 text-xs text-red-400 hover:bg-red-500/10" :disabled="user.id === auth.user?.id" @click="remove(user)">{{ t('common.delete') }}</button>
            </div></td>
          </tr></template>
        </tbody>
      </table>
      <footer class="flex items-center justify-between p-4 text-sm text-text-muted">
        <span>{{ t('users.count', { count: total }) }}</span>
        <div class="flex gap-2"><button class="btn-secondary" :disabled="offset === 0" @click="page(-1)">{{ t('common.previous') }}</button><button class="btn-secondary" :disabled="offset + limit >= total" @click="page(1)">{{ t('common.next') }}</button></div>
      </footer>
    </div>

    <div v-if="modal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" @click.self="modal = null">
      <form class="w-full max-w-lg space-y-4 rounded-2xl border border-border-subtle bg-bg-card p-6 shadow-2xl" @submit.prevent="save">
        <h2 class="text-xl font-semibold">{{ modal === 'create' ? t('users.create') : t('users.edit') }}</h2>
        <label class="block text-sm">{{ t('users.username') }}<input v-model.trim="form.username" class="input mt-1 w-full" minlength="3" maxlength="64" required /></label>
        <label class="block text-sm">{{ t('users.email') }}<input v-model.trim="form.email" type="email" class="input mt-1 w-full" required /></label>
        <label v-if="modal === 'create'" class="block text-sm">{{ t('users.initialPassword') }}<input v-model="form.password" type="password" class="input mt-1 w-full" minlength="12" required /><small class="text-text-muted">{{ t('users.passwordHint') }}</small></label>
        <label class="block text-sm">{{ t('users.role') }}<select v-model="form.role" class="input mt-1 w-full"><option v-for="role in roles" :key="role">{{ role }}</option></select></label>
        <div class="flex justify-end gap-2"><button type="button" class="btn-secondary" @click="modal = null">{{ t('common.cancel') }}</button><button class="btn-primary" :disabled="saving">{{ saving ? t('common.saving') : t('common.save') }}</button></div>
      </form>
    </div>
    <div v-if="auditUser" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" @click.self="auditUser = null">
      <div class="max-h-[80vh] w-full max-w-2xl overflow-auto rounded-2xl border border-border-subtle bg-bg-card p-6 shadow-2xl">
        <div class="mb-4 flex items-center justify-between"><h2 class="text-xl font-semibold">{{ t('users.historyTitle', { name: auditUser.username }) }}</h2><button class="btn-secondary" @click="auditUser = null">{{ t('common.close') }}</button></div>
        <p v-if="auditLoading" class="py-8 text-center text-text-muted">{{ t('users.historyLoading') }}</p>
        <p v-else-if="!auditEntries.length" class="py-8 text-center text-text-muted">{{ t('users.noHistory') }}</p>
        <ol v-else class="space-y-3"><li v-for="entry in auditEntries" :key="entry.id" class="rounded-lg border border-border-subtle p-3"><div class="flex justify-between gap-4"><strong>{{ entry.action }}</strong><time class="text-xs text-text-muted">{{ new Date(entry.created_at).toLocaleString() }}</time></div><pre v-if="entry.metadata" class="mt-2 overflow-auto text-xs text-text-muted">{{ JSON.stringify(entry.metadata, null, 2) }}</pre></li></ol>
      </div>
    </div>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import api, { apiErrorMessage } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { useConfirmStore } from '@/stores/confirm'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const auth = useAuthStore()
const confirm = useConfirmStore()
const roles = ['admin', 'operator', 'viewer']
const users = ref([])
const total = ref(0)
const limit = 25
const offset = ref(0)
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')
const modal = ref(null)
const auditUser = ref(null)
const auditEntries = ref([])
const auditLoading = ref(false)
const editingId = ref('')
const filters = reactive({ q: '', role: '', status: '' })
const form = reactive({ username: '', email: '', password: '', role: 'viewer' })
let timer

async function load() {
  loading.value = true; error.value = ''
  try {
    const { data } = await api.get('/users', { params: { ...filters, limit, offset: offset.value } })
    users.value = data.items || []; total.value = data.total || 0
  } catch (e) { error.value = apiErrorMessage(e, t('users.unableLoad')) }
  finally { loading.value = false }
}
function debouncedLoad() { clearTimeout(timer); offset.value = 0; timer = setTimeout(load, 250) }
function page(direction) { offset.value = Math.max(0, offset.value + direction * limit); load() }
function openCreate() { editingId.value = ''; Object.assign(form, { username: '', email: '', password: '', role: 'viewer' }); modal.value = 'create' }
function openEdit(user) { editingId.value = user.id; Object.assign(form, { username: user.username, email: user.email, password: '', role: user.role }); modal.value = 'edit' }
async function save() {
  saving.value = true; error.value = ''
  try {
    if (modal.value === 'create') await api.post('/users', form)
    else await api.put(`/users/${editingId.value}`, { username: form.username, email: form.email, role: form.role })
    notice.value = modal.value === 'create' ? t('users.created') : t('users.updated'); modal.value = null; await load()
  } catch (e) { error.value = apiErrorMessage(e, t('users.unableSave')) }
  finally { saving.value = false }
}
async function changePassword(user) {
  const password = window.prompt(t('users.passwordPrompt', { name: user.username }))
  if (!password) return
  try { await api.put(`/users/${user.id}/password`, { password }); notice.value = t('users.passwordChanged') }
  catch (e) { error.value = apiErrorMessage(e, t('users.unablePassword')) }
}
async function toggleActive(user) {
  const action = user.is_active ? 'block' : 'unblock'
  const actionLabel = action === 'block' ? t('users.block') : t('users.unblock')
  const ok = await confirm.require({ title: actionLabel, message: t('users.confirmToggle', { action: actionLabel, name: user.username }), confirmText: actionLabel, type: action === 'block' ? 'danger' : 'warning' })
  if (!ok) return
  try { await api.post(`/users/${user.id}/${action}`); notice.value = actionLabel; await load() }
  catch (e) { error.value = apiErrorMessage(e, t('users.unableSave')) }
}
async function remove(user) {
  const ok = await confirm.require({ title: t('common.delete'), message: t('users.confirmDelete', { name: user.username }), confirmText: t('common.delete'), type: 'danger' })
  if (!ok) return
  try { await api.delete(`/users/${user.id}`); notice.value = t('users.deleted'); await load() }
  catch (e) { error.value = apiErrorMessage(e, t('users.unableDelete')) }
}
async function showAudit(user) {
  auditUser.value = user; auditEntries.value = []; auditLoading.value = true
  try { const { data } = await api.get(`/users/${user.id}/audit`); auditEntries.value = data || [] }
  catch (e) { error.value = apiErrorMessage(e, t('users.unableHistory')) }
  finally { auditLoading.value = false }
}
onMounted(load)
</script>
