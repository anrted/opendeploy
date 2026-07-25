<script setup>
import { ref, onMounted } from 'vue'
import api, { apiErrorMessage } from '@/api/client'

const rules = ref([])
const loading = ref(false)
const error = ref('')

const newRule = ref({
  port: '',
  protocol: 'tcp',
  action: 'allow'
})
const adding = ref(false)

async function fetchRules() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await api.get('/modules/ufw/rules')
    rules.value = data.rules || []
  } catch (e) {
    error.value = apiErrorMessage(e, 'Failed to fetch rules')
  } finally {
    loading.value = false
  }
}

async function addRule() {
  if (!newRule.value.port) return
  adding.value = true
  error.value = ''
  try {
    await api.post('/modules/ufw/rules', {
      port: parseInt(newRule.value.port, 10),
      protocol: newRule.value.protocol,
      action: newRule.value.action
    })
    newRule.value.port = '' // reset
    await fetchRules()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Failed to add rule')
  } finally {
    adding.value = false
  }
}

async function deleteRule(rule) {
  if (!confirm(`Delete rule ${rule.action} ${rule.port}/${rule.protocol}?`)) return
  error.value = ''
  try {
    await api.delete('/modules/ufw/rules', {
      data: {
        port: parseInt(rule.port, 10),
        protocol: String(rule.protocol).toLowerCase(),
        action: 'delete'
      }
    })
    await fetchRules()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Failed to delete rule')
  }
}

onMounted(() => {
  fetchRules()
})
</script>

<template>
  <div class="space-y-6">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-2xl font-bold text-white tracking-tight">Firewall Rules</h1>
          <p class="mt-1 text-sm text-[#94a3b8]">Manage UFW firewall access rules</p>
        </div>
        <button @click="fetchRules" class="btn-secondary" :disabled="loading">
          Refresh
        </button>
      </div>

      <div v-if="error" class="rounded-lg border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-400">
        {{ error }}
      </div>

      <div class="card p-6">
        <h2 class="text-lg font-semibold text-white mb-4">Add New Rule</h2>
        <form @submit.prevent="addRule" class="flex flex-wrap items-end gap-4">
          <div class="flex-1 min-w-[200px]">
            <label class="block text-sm font-medium text-[#94a3b8] mb-1">Port</label>
            <input type="number" v-model="newRule.port" class="input-field" placeholder="e.g. 8080" required min="1" max="65535" />
          </div>
          <div class="w-32">
            <label class="block text-sm font-medium text-[#94a3b8] mb-1">Protocol</label>
            <select v-model="newRule.protocol" class="input-field">
              <option value="tcp">TCP</option>
              <option value="udp">UDP</option>
            </select>
          </div>
          <div class="w-32">
            <label class="block text-sm font-medium text-[#94a3b8] mb-1">Action</label>
            <select v-model="newRule.action" class="input-field">
              <option value="allow">Allow</option>
              <option value="deny">Deny</option>
            </select>
          </div>
          <button type="submit" class="btn-primary mb-[2px]" :disabled="adding">
            {{ adding ? 'Adding...' : 'Add Rule' }}
          </button>
        </form>
      </div>

      <div class="card overflow-hidden">
        <table class="w-full text-left text-sm text-[#94a3b8]">
          <thead class="bg-white/[0.02] text-[#e2e8f0]">
            <tr>
              <th class="px-6 py-4 font-medium">Port</th>
              <th class="px-6 py-4 font-medium">Protocol</th>
              <th class="px-6 py-4 font-medium">Action</th>
              <th class="px-6 py-4 font-medium text-right">Manage</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-white/5">
            <tr v-if="loading && rules.length === 0">
              <td colspan="4" class="px-6 py-8 text-center">Loading rules...</td>
            </tr>
            <tr v-else-if="rules.length === 0">
              <td colspan="4" class="px-6 py-8 text-center text-[#64748b]">No specific rules configured. Defaults apply.</td>
            </tr>
            <tr v-for="(rule, idx) in rules" :key="idx" class="hover:bg-white/[0.02] transition-colors">
              <td class="px-6 py-4 font-mono text-[#e2e8f0]">{{ rule.port }}</td>
              <td class="px-6 py-4 uppercase">{{ rule.protocol }}</td>
              <td class="px-6 py-4">
                <span :class="rule.action.toLowerCase() === 'allow' ? 'badge-success' : 'badge-error'">
                  {{ rule.action }}
                </span>
              </td>
              <td class="px-6 py-4 text-right">
                <button @click="deleteRule(rule)" class="text-red-400 hover:text-red-300 transition-colors" title="Delete Rule">
                  <svg class="w-5 h-5 inline-block" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
</template>
