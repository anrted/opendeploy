<template>
  <section class="space-y-6">
    <header class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-bold text-text-main">Users</h1>
        <p class="mt-1 text-sm text-text-muted">Accounts, roles and access status</p>
      </div>
      <button class="btn-primary" @click="openCreate">Create user</button>
    </header>

    <div class="grid gap-3 rounded-xl border border-border-subtle bg-bg-card p-4 md:grid-cols-4">
      <input v-model="filters.q" class="input" placeholder="Search name or email…" @input="debouncedLoad" />
      <select v-model="filters.role" class="input" @change="load"><option value="">All roles</option><option v-for="role in roles" :key="role">{{ role }}</option></select>
      <select v-model="filters.status" class="input" @change="load"><option value="">All statuses</option><option value="active">Active</option><option value="blocked">Blocked</option></select>
      <button class="btn-secondary" :disabled="loading" @click="load">Refresh</button>
    </div>

    <div v-if="notice" class="rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-3 text-sm text-emerald-400">{{ notice }}</div>
    <div v-if="error" class="rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-400">{{ error }}</div>

    <div class="overflow-x-auto rounded-xl border border-border-subtle bg-bg-card">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-border-subtle text-xs uppercase text-text-muted"><tr><th class="p-4">User</th><th class="p-4">Role</th><th class="p-4">Status</th><th class="p-4">Last login</th><th class="p-4 text-right">Actions</th></tr></thead>
        <tbody>
          <template v-if="loading"><tr v-for="n in 4" :key="n" class="border-b border-border-subtle"><td colspan="5" class="p-4"><div class="h-5 animate-pulse rounded bg-slate-500/15"></div></td></tr></template>
          <tr v-else-if="!users.length"><td colspan="5" class="p-12 text-center text-text-muted">No users match the filters.</td></tr>
          <template v-else><tr v-for="user in users" :key="user.id" class="border-b border-border-subtle last:border-0">
            <td class="p-4"><div class="font-medium text-text-main">{{ user.username }}</div><div class="text-xs text-text-muted">{{ user.email }}</div></td>
            <td class="p-4 capitalize">{{ user.role }}</td>
            <td class="p-4"><span class="rounded-full px-2 py-1 text-xs" :class="user.is_active ? 'bg-emerald-500/10 text-emerald-400' : 'bg-red-500/10 text-red-400'">{{ user.is_active ? 'Active' : 'Blocked' }}</span></td>
            <td class="p-4 text-text-muted">{{ user.last_login ? new Date(user.last_login).toLocaleString() : 'Never' }}</td>
            <td class="p-4"><div class="flex justify-end gap-2">
              <button class="btn-secondary text-xs" @click="openEdit(user)">Edit</button>
              <button class="btn-secondary text-xs" @click="changePassword(user)">Password</button>
              <button class="btn-secondary text-xs" @click="showAudit(user)">History</button>
              <button class="btn-secondary text-xs" :disabled="user.id === auth.user?.id" @click="toggleActive(user)">{{ user.is_active ? 'Block' : 'Unblock' }}</button>
              <button class="rounded-lg px-3 py-2 text-xs text-red-400 hover:bg-red-500/10" :disabled="user.id === auth.user?.id" @click="remove(user)">Delete</button>
            </div></td>
          </tr></template>
        </tbody>
      </table>
      <footer class="flex items-center justify-between p-4 text-sm text-text-muted">
        <span>{{ total }} user{{ total === 1 ? '' : 's' }}</span>
        <div class="flex gap-2"><button class="btn-secondary" :disabled="offset === 0" @click="page(-1)">Previous</button><button class="btn-secondary" :disabled="offset + limit >= total" @click="page(1)">Next</button></div>
      </footer>
    </div>

    <div v-if="modal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" @click.self="modal = null">
      <form class="w-full max-w-lg space-y-4 rounded-2xl border border-border-subtle bg-bg-card p-6 shadow-2xl" @submit.prevent="save">
        <h2 class="text-xl font-semibold">{{ modal === 'create' ? 'Create user' : 'Edit user' }}</h2>
        <label class="block text-sm">Username<input v-model.trim="form.username" class="input mt-1 w-full" minlength="3" maxlength="64" required /></label>
        <label class="block text-sm">Email<input v-model.trim="form.email" type="email" class="input mt-1 w-full" required /></label>
        <label v-if="modal === 'create'" class="block text-sm">Initial password<input v-model="form.password" type="password" class="input mt-1 w-full" minlength="12" required /><small class="text-text-muted">At least 12 characters.</small></label>
        <label class="block text-sm">Role<select v-model="form.role" class="input mt-1 w-full"><option v-for="role in roles" :key="role">{{ role }}</option></select></label>
        <div class="flex justify-end gap-2"><button type="button" class="btn-secondary" @click="modal = null">Cancel</button><button class="btn-primary" :disabled="saving">{{ saving ? 'Saving…' : 'Save' }}</button></div>
      </form>
    </div>
    <div v-if="auditUser" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" @click.self="auditUser = null">
      <div class="max-h-[80vh] w-full max-w-2xl overflow-auto rounded-2xl border border-border-subtle bg-bg-card p-6 shadow-2xl">
        <div class="mb-4 flex items-center justify-between"><h2 class="text-xl font-semibold">History · {{ auditUser.username }}</h2><button class="btn-secondary" @click="auditUser = null">Close</button></div>
        <p v-if="auditLoading" class="py-8 text-center text-text-muted">Loading history…</p>
        <p v-else-if="!auditEntries.length" class="py-8 text-center text-text-muted">No recorded actions.</p>
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
  } catch (e) { error.value = apiErrorMessage(e, 'Unable to load users') }
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
    notice.value = modal.value === 'create' ? 'User created.' : 'User updated.'; modal.value = null; await load()
  } catch (e) { error.value = apiErrorMessage(e, 'Unable to save user') }
  finally { saving.value = false }
}
async function changePassword(user) {
  const password = window.prompt(`New password for ${user.username} (at least 12 characters):`)
  if (!password) return
  try { await api.put(`/users/${user.id}/password`, { password }); notice.value = 'Password changed and active sessions revoked.' }
  catch (e) { error.value = apiErrorMessage(e, 'Unable to change password') }
}
async function toggleActive(user) {
  const action = user.is_active ? 'block' : 'unblock'
  const ok = await confirm.require({ title: `${action === 'block' ? 'Block' : 'Unblock'} user`, message: `${action} ${user.username}?`, confirmText: action, type: action === 'block' ? 'danger' : 'warning' })
  if (!ok) return
  try { await api.post(`/users/${user.id}/${action}`); notice.value = `User ${action}ed.`; await load() }
  catch (e) { error.value = apiErrorMessage(e, `Unable to ${action} user`) }
}
async function remove(user) {
  const ok = await confirm.require({ title: 'Delete user', message: `Permanently delete ${user.username}? Their sessions will be revoked.`, confirmText: 'Delete', type: 'danger' })
  if (!ok) return
  try { await api.delete(`/users/${user.id}`); notice.value = 'User deleted.'; await load() }
  catch (e) { error.value = apiErrorMessage(e, 'Unable to delete user') }
}
async function showAudit(user) {
  auditUser.value = user; auditEntries.value = []; auditLoading.value = true
  try { const { data } = await api.get(`/users/${user.id}/audit`); auditEntries.value = data || [] }
  catch (e) { error.value = apiErrorMessage(e, 'Unable to load user history') }
  finally { auditLoading.value = false }
}
onMounted(load)
</script>
