<script setup>
import { ref, onMounted, computed } from 'vue'
import api, { apiErrorMessage } from '@/api/client'
import EmptyState from '@/components/EmptyState.vue'
import { useConfirmStore } from '@/stores/confirm'
import { useI18n } from 'vue-i18n'

const confirm = useConfirmStore()
const { t } = useI18n()
const activeTab = ref('overview')
const status = ref(null)
const rules = ref([])
const loading = ref(false)
const error = ref('')

// Add Rule Form State
const newRule = ref({
  port: '',
  protocol: 'tcp',
  action: 'allow',
  direction: 'in',
  source: '',
  destination: '',
  comment: '',
  ip_version: 'both'
})
const adding = ref(false)

async function fetchStatus() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await api.get('/modules/firewall/status')
    status.value = data.status
  } catch (e) {
    error.value = apiErrorMessage(e, t('firewall.fetchStatusFailed'))
  } finally {
    loading.value = false
  }
}

async function fetchRules() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await api.get('/modules/firewall/rules')
    rules.value = data.rules || []
  } catch (e) {
    error.value = apiErrorMessage(e, t('firewall.fetchRulesFailed'))
  } finally {
    loading.value = false
  }
}

async function refreshAll() {
  await Promise.all([fetchStatus(), fetchRules()])
}

async function toggleFirewall() {
  if (!status.value) return
  const willEnable = !status.value.active
  if (!willEnable) {
    const confirmed = await confirm.require({
      title: t('firewall.disable'),
      message: t('firewall.disableMessage'),
      confirmText: t('moduleDetails.disable'),
      type: 'danger'
    })
    if (!confirmed) return
  }
  
  error.value = ''
  loading.value = true
  try {
    await api.post('/modules/firewall/toggle', { enable: willEnable })
    await fetchStatus()
  } catch (e) {
    error.value = apiErrorMessage(e, t('firewall.toggleFailed'))
  } finally {
    loading.value = false
  }
}

async function resetFirewall() {
  const confirmed = await confirm.require({
    title: t('firewall.reset'),
    message: t('firewall.resetMessage'),
    confirmText: t('firewall.reset'),
    type: 'danger'
  })
  if (!confirmed) return
  
  error.value = ''
  loading.value = true
  try {
    await api.post('/modules/firewall/reset')
    await refreshAll()
  } catch (e) {
    error.value = apiErrorMessage(e, t('firewall.resetFailed'))
  } finally {
    loading.value = false
  }
}

async function addRule() {
  // Basic validation
  if (!newRule.value.port && !newRule.value.source && !newRule.value.destination) {
    error.value = t('firewall.validation')
    return
  }

  // Safety check for blocking SSH
  if ((newRule.value.port === '22' || newRule.value.port === 'ssh') && newRule.value.action !== 'allow') {
    const confirmed = await confirm.require({
      title: t('firewall.sshWarning'),
      message: t('firewall.sshWarningMessage'),
      confirmText: t('firewall.proceed'),
      type: 'danger'
    })
    if (!confirmed) return
  }

  adding.value = true
  error.value = ''
  try {
    await api.post('/modules/firewall/rules', newRule.value)
    
    // Reset form
    newRule.value.port = ''
    newRule.value.source = ''
    newRule.value.destination = ''
    newRule.value.comment = ''
    
    activeTab.value = 'rules'
    await fetchRules()
  } catch (e) {
    error.value = apiErrorMessage(e, t('firewall.addFailed'))
  } finally {
    adding.value = false
  }
}

async function deleteRule(rule) {
  if (rule.port === '22' || rule.port === 'ssh') {
    const confirmed = await confirm.require({
      title: t('firewall.sshDeleteWarning'),
      message: t('firewall.sshDeleteMessage'),
      confirmText: t('firewall.proceed'),
      type: 'danger'
    })
    if (!confirmed) return
  } else if (rule.port === '443' || rule.port === '5888') {
    alert(t('firewall.protectedPort', { port: rule.port }))
    return
  } else {
    const confirmed = await confirm.require({
      title: t('firewall.deleteRule'),
      message: t('firewall.deleteRuleMessage', { id: rule.id }),
      confirmText: t('common.delete'),
      type: 'danger'
    })
    if (!confirmed) return
  }

  error.value = ''
  try {
    await api.delete('/modules/firewall/rules', {
      data: { id: rule.id }
    })
    await fetchRules()
  } catch (e) {
    error.value = apiErrorMessage(e, t('firewall.deleteFailed'))
  }
}

onMounted(() => {
  refreshAll()
})

const tabs = computed(() => ['overview', 'rules', 'add', 'settings'].map(id => ({ id, label: t(`firewall.${id === 'add' ? 'addRule' : id}`) })))

</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h1 class="text-3xl font-bold text-white tracking-tight flex items-center gap-3">
          <svg class="w-8 h-8 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"></path></svg>
          {{ t('firewall.title') }}
        </h1>
        <p class="mt-2 text-sm text-slate-400">{{ t('firewall.subtitle') }}</p>
      </div>
      <div class="flex items-center gap-3">
        <button @click="refreshAll" class="btn-secondary flex items-center gap-2" :disabled="loading">
          <svg class="w-4 h-4" :class="{'animate-spin': loading}" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path></svg>
          {{ t('common.refresh') }}
        </button>
      </div>
    </div>

    <!-- Error Alert -->
    <div v-if="error" class="rounded-xl border border-red-500/30 bg-red-500/10 p-4 flex items-start gap-3 text-red-400">
      <svg class="w-5 h-5 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path></svg>
      <div>
        <h3 class="font-medium">{{ t('firewall.actionFailed') }}</h3>
        <p class="text-sm opacity-90 mt-1">{{ error }}</p>
      </div>
    </div>

    <!-- Tabs Navigation -->
    <div class="border-b border-white/10">
      <nav class="-mb-px flex space-x-8" aria-label="Tabs">
        <button v-for="tab in tabs" :key="tab.id" @click="activeTab = tab.id"
                :class="[
                  activeTab === tab.id
                    ? 'border-blue-500 text-blue-400'
                    : 'border-transparent text-slate-400 hover:text-slate-300 hover:border-slate-300',
                  'whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm transition-colors'
                ]">
          {{ tab.label }}
        </button>
      </nav>
    </div>

    <!-- Tab Contents -->
    <div class="pt-2">
      <!-- OVERVIEW TAB -->
      <div v-if="activeTab === 'overview'" class="space-y-6">
        <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
          
          <!-- Status Card -->
          <div class="card p-6 relative overflow-hidden group">
            <div class="absolute inset-0 bg-gradient-to-br from-blue-500/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity"></div>
            <h3 class="text-lg font-semibold text-white mb-2 flex items-center justify-between">
              {{ t('firewall.status') }}
              <span class="relative flex h-3 w-3">
                <span v-if="status?.active" class="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
                <span class="relative inline-flex rounded-full h-3 w-3" :class="status?.active ? 'bg-green-500' : 'bg-slate-500'"></span>
              </span>
            </h3>
            <div class="text-3xl font-bold text-white mb-4">
              {{ status?.active ? t('common.active') : t('firewall.inactive') }}
            </div>
            <button @click="toggleFirewall" :disabled="loading" 
                    :class="['w-full py-2 px-4 rounded-lg font-medium transition-all duration-200',
                             status?.active ? 'bg-white/5 text-white hover:bg-red-500/20 hover:text-red-400 border border-white/10 hover:border-red-500/50' : 'bg-blue-600 hover:bg-blue-500 text-white shadow-lg shadow-blue-500/25']">
              {{ status?.active ? t('firewall.disable') : t('firewall.enable') }}
            </button>
          </div>

          <!-- Rules Summary Card -->
          <div class="card p-6">
            <h3 class="text-lg font-semibold text-white mb-2">{{ t('firewall.activeRules') }}</h3>
            <div class="text-3xl font-bold text-white mb-4">{{ rules.length }}</div>
            <p class="text-sm text-slate-400">{{ t('firewall.rulesSummary') }}</p>
            <button @click="activeTab = 'rules'" class="mt-4 text-sm text-blue-400 hover:text-blue-300 font-medium">
              {{ t('firewall.viewRules') }}
            </button>
          </div>

          <!-- Provider Card -->
          <div class="card p-6">
            <h3 class="text-lg font-semibold text-white mb-2">{{ t('firewall.provider') }}</h3>
            <div class="text-xl font-medium text-white mb-4 uppercase">{{ status?.profile_name || 'UFW' }}</div>
            <div class="flex items-center gap-2 text-sm text-slate-400">
              <svg class="w-4 h-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg>
              {{ t('firewall.ipv6') }}
            </div>
          </div>
        </div>
      </div>

      <!-- RULES TAB -->
      <div v-else-if="activeTab === 'rules'" class="space-y-4">
        <div class="flex justify-between items-center mb-4">
          <div class="relative w-64">
            <!-- Optional Search Input Could Go Here -->
          </div>
          <button @click="activeTab = 'add'" class="btn-primary">
            + {{ t('firewall.newRule') }}
          </button>
        </div>

        <div class="card overflow-hidden">
          <table class="w-full text-left text-sm text-slate-400">
            <thead class="bg-slate-800/50 text-slate-300">
              <tr>
                <th class="px-6 py-4 font-medium rounded-tl-xl">ID</th>
                <th class="px-6 py-4 font-medium">{{ t('firewall.action') }}</th>
                <th class="px-6 py-4 font-medium">{{ t('firewall.direction') }}</th>
                <th class="px-6 py-4 font-medium">{{ t('firewall.target') }}</th>
                <th class="px-6 py-4 font-medium">{{ t('firewall.source') }}</th>
                <th class="px-6 py-4 font-medium text-right rounded-tr-xl">{{ t('firewall.manage') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-white/5">
              <tr v-if="loading && rules.length === 0">
                <td colspan="6" class="px-6 py-12 text-center">
                  <div class="flex flex-col items-center justify-center text-slate-500">
                    <svg class="w-8 h-8 animate-spin mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path></svg>
                    {{ t('firewall.loadingRules') }}
                  </div>
                </td>
              </tr>
              <tr v-else-if="rules.length === 0">
                <td colspan="6" class="p-0">
                  <EmptyState :title="t('firewall.noRules')" :description="t('firewall.noRulesDescription')">
                    <template #action>
                      <button class="btn-primary" @click="activeTab = 'add'">{{ t('firewall.newRule') }}</button>
                    </template>
                  </EmptyState>
                </td>
              </tr>
              <tr v-for="rule in rules" :key="rule.id" class="hover:bg-white/[0.02] transition-colors group">
                <td class="px-6 py-4 font-mono text-slate-500">[{{ rule.id }}]</td>
                <td class="px-6 py-4">
                  <span :class="{
                    'bg-green-500/10 text-green-400 border border-green-500/20': rule.action === 'allow',
                    'bg-red-500/10 text-red-400 border border-red-500/20': rule.action === 'deny',
                    'bg-orange-500/10 text-orange-400 border border-orange-500/20': rule.action === 'reject'
                  }" class="px-2.5 py-1 rounded-md text-xs font-medium uppercase tracking-wider">
                    {{ rule.action }}
                  </span>
                </td>
                <td class="px-6 py-4 uppercase text-xs font-semibold tracking-wider">{{ rule.direction }}</td>
                <td class="px-6 py-4">
                  <div class="flex items-center gap-2">
                    <span class="text-slate-200 font-medium">{{ rule.destination }}</span>
                    <span v-if="rule.port" class="text-blue-400 font-mono">{{ rule.port }}</span>
                    <span v-if="rule.protocol && rule.protocol !== 'any'" class="text-slate-500 text-xs uppercase">{{ rule.protocol }}</span>
                  </div>
                  <div v-if="rule.ip_version !== 'both'" class="text-xs text-slate-500 mt-0.5">({{ rule.ip_version }})</div>
                </td>
                <td class="px-6 py-4 text-slate-300">{{ rule.source || t('firewall.anywhere') }}</td>
                <td class="px-6 py-4 text-right">
                  <button @click="deleteRule(rule)" class="text-slate-500 hover:text-red-400 transition-colors opacity-0 group-hover:opacity-100 p-2 hover:bg-white/5 rounded-lg" :title="t('firewall.deleteRule')">
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- ADD RULE TAB -->
      <div v-else-if="activeTab === 'add'" class="max-w-2xl">
        <form @submit.prevent="addRule" class="card p-8 space-y-6">
          <h3 class="text-xl font-bold text-white border-b border-white/10 pb-4">{{ t('firewall.createRule') }}</h3>
          
          <div class="grid grid-cols-2 gap-6">
            <div class="space-y-2">
              <label class="block text-sm font-medium text-slate-300">{{ t('firewall.action') }}</label>
              <select v-model="newRule.action" class="input-field bg-slate-900 w-full">
                <option value="allow">{{ t('firewall.allow') }}</option>
                <option value="deny">{{ t('firewall.deny') }}</option>
                <option value="reject">{{ t('firewall.reject') }}</option>
              </select>
            </div>
            
            <div class="space-y-2">
              <label class="block text-sm font-medium text-slate-300">{{ t('firewall.direction') }}</label>
              <select v-model="newRule.direction" class="input-field bg-slate-900 w-full">
                <option value="in">{{ t('firewall.incoming') }}</option>
                <option value="out">{{ t('firewall.outgoing') }}</option>
              </select>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-6">
            <div class="space-y-2">
              <label class="block text-sm font-medium text-slate-300">{{ t('firewall.protocol') }}</label>
              <select v-model="newRule.protocol" class="input-field bg-slate-900 w-full">
                <option value="tcp">TCP</option>
                <option value="udp">UDP</option>
                <option value="any">{{ t('firewall.anyProtocol') }}</option>
              </select>
            </div>
            
            <div class="space-y-2">
              <label class="block text-sm font-medium text-slate-300">{{ t('firewall.ipVersion') }}</label>
              <select v-model="newRule.ip_version" class="input-field bg-slate-900 w-full">
                <option value="both">{{ t('firewall.bothVersions') }}</option>
                <option value="ipv4">{{ t('firewall.ipv4Only') }}</option>
                <option value="ipv6">{{ t('firewall.ipv6Only') }}</option>
              </select>
            </div>
          </div>

          <div class="space-y-4 pt-4 border-t border-white/5">
            <h4 class="text-sm font-semibold text-slate-400 uppercase tracking-wider">{{ t('firewall.targetSpecifications') }}</h4>
            
            <div class="space-y-2">
              <label class="block text-sm font-medium text-slate-300">{{ t('firewall.port') }}</label>
              <input type="text" v-model="newRule.port" class="input-field w-full" placeholder="e.g. 22, 80, 8000:8100, ssh" />
              <p class="text-xs text-slate-500">{{ t('firewall.allPorts') }}</p>
            </div>

            <div class="space-y-2">
              <label class="block text-sm font-medium text-slate-300">{{ t('firewall.sourceOptional') }}</label>
              <input type="text" v-model="newRule.source" class="input-field w-full" placeholder="e.g. 192.168.1.100, 10.0.0.0/8" />
              <p class="text-xs text-slate-500">{{ t('firewall.anywhereHint') }}</p>
            </div>
            
            <div class="space-y-2">
              <label class="block text-sm font-medium text-slate-300">{{ t('firewall.destination') }}</label>
              <input type="text" v-model="newRule.destination" class="input-field w-full" placeholder="e.g. 192.168.1.5" />
            </div>
          </div>
          
          <div class="space-y-2 pt-4 border-t border-white/5">
            <label class="block text-sm font-medium text-slate-300">{{ t('firewall.comment') }}</label>
            <input type="text" v-model="newRule.comment" class="input-field w-full" :placeholder="t('firewall.commentPlaceholder')" />
          </div>

          <div class="pt-6 flex gap-4">
            <button type="submit" class="btn-primary flex-1 py-3 text-base shadow-lg shadow-blue-500/25" :disabled="adding">
              {{ adding ? t('firewall.applying') : t('firewall.addRule') }}
            </button>
            <button type="button" @click="activeTab = 'rules'" class="btn-secondary px-8">
              {{ t('common.cancel') }}
            </button>
          </div>
        </form>
      </div>

      <!-- SETTINGS TAB -->
      <div v-else-if="activeTab === 'settings'" class="max-w-3xl space-y-6">
        <div class="card p-6">
          <h3 class="text-xl font-bold text-white mb-6 border-b border-white/10 pb-4">{{ t('firewall.defaultPolicies') }}</h3>
          
          <div class="space-y-6">
            <div class="flex items-center justify-between">
              <div>
                <h4 class="text-slate-200 font-medium">{{ t('firewall.incomingTraffic') }}</h4>
                <p class="text-sm text-slate-500">{{ t('firewall.incomingDescription') }}</p>
              </div>
              <div class="px-4 py-2 bg-slate-800 rounded-lg border border-white/10 text-slate-300 font-mono text-sm capitalize">
                {{ status?.default_incoming || t('common.unknown') }}
              </div>
            </div>

            <div class="flex items-center justify-between">
              <div>
                <h4 class="text-slate-200 font-medium">{{ t('firewall.outgoingTraffic') }}</h4>
                <p class="text-sm text-slate-500">{{ t('firewall.outgoingDescription') }}</p>
              </div>
              <div class="px-4 py-2 bg-slate-800 rounded-lg border border-white/10 text-slate-300 font-mono text-sm capitalize">
                {{ status?.default_outgoing || t('common.unknown') }}
              </div>
            </div>

            <div class="flex items-center justify-between">
              <div>
                <h4 class="text-slate-200 font-medium">{{ t('firewall.routedTraffic') }}</h4>
                <p class="text-sm text-slate-500">{{ t('firewall.routedDescription') }}</p>
              </div>
              <div class="px-4 py-2 bg-slate-800 rounded-lg border border-white/10 text-slate-300 font-mono text-sm capitalize">
                {{ status?.default_routed || t('common.unknown') }}
              </div>
            </div>
          </div>
        </div>

        <div class="card p-6 border-red-500/20">
          <h3 class="text-xl font-bold text-red-400 mb-6 border-b border-red-500/20 pb-4">{{ t('firewall.dangerZone') }}</h3>
          
          <div class="flex flex-col sm:flex-row gap-4 items-start sm:items-center justify-between p-4 rounded-xl border border-red-500/20 bg-red-500/5">
            <div>
              <h4 class="text-slate-200 font-medium">{{ t('firewall.reset') }}</h4>
              <p class="text-sm text-slate-500 mt-1 max-w-md">{{ t('firewall.resetDescription') }}</p>
            </div>
            <button @click="resetFirewall" :disabled="loading" class="btn-secondary whitespace-nowrap bg-transparent text-red-400 hover:text-white hover:bg-red-500 border-red-500/50">
              {{ t('firewall.resetDefaults') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
