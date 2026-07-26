<template>
  <div>
    <div v-if="errorMessage" class="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{{ errorMessage }}</div>
    
    <!-- Top Bar -->
    <div class="flex flex-col md:flex-row items-start md:items-center justify-between mb-6 gap-4">
      <div>
        <h1 class="page-title">Modules</h1>
        <p class="page-subtitle">Install and manage server capabilities</p>
      </div>
      
      <!-- Stats -->
      <div class="flex gap-4 text-sm text-[#94a3b8]">
        <div class="bg-[#1e293b] px-3 py-1.5 rounded-lg border border-[#334155]">Total: <span class="font-bold text-white">{{ stats.total }}</span></div>
        <div class="bg-[#1e293b] px-3 py-1.5 rounded-lg border border-[#334155]">Installed: <span class="font-bold text-white">{{ stats.installed }}</span></div>
      </div>
    </div>

    <!-- Filters -->
    <div class="flex flex-col md:flex-row gap-4 mb-6">
      <div class="flex-1 relative">
        <input 
          v-model="searchQuery" 
          type="text" 
          placeholder="Search modules..." 
          class="w-full bg-[#1e293b] border border-[#334155] rounded-lg pl-10 pr-4 py-2 text-sm text-[#e2e8f0] focus:outline-none focus:border-indigo-500"
        />
        <svg class="w-4 h-4 text-[#64748b] absolute left-3 top-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
      </div>
      
      <select v-model="selectedCategory" class="bg-[#1e293b] border border-[#334155] rounded-lg px-4 py-2 text-sm text-[#e2e8f0] focus:outline-none focus:border-indigo-500">
        <option value="">All Categories</option>
        <option v-for="cat in categories" :key="cat" :value="cat">{{ cat }}</option>
      </select>
      
      <select v-model="selectedState" class="bg-[#1e293b] border border-[#334155] rounded-lg px-4 py-2 text-sm text-[#e2e8f0] focus:outline-none focus:border-indigo-500">
        <option value="">All States</option>
        <option value="installed">Any Installed</option>
        <option value="available">Available</option>
        <option value="enabled">Enabled</option>
        <option value="disabled">Disabled</option>
        <option value="error">Error</option>
      </select>
    </div>

    <div v-if="loading" class="flex justify-center py-20">
      <svg class="w-6 h-6 text-indigo-400 animate-spin" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
      </svg>
    </div>

    <div v-else-if="filteredModules.length === 0" class="text-center py-20 text-[#64748b]">
      No modules found matching your criteria.
    </div>

    <!-- Grid -->
    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
      <router-link 
        v-for="mod in filteredModules" :key="mod.id" 
        :to="'/modules/' + mod.id"
        class="card flex flex-col gap-4 hover:border-indigo-500/50 transition-colors duration-200 cursor-pointer h-full"
      >
        <!-- Header -->
        <div class="flex items-start justify-between">
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-lg bg-[#1e293b] border border-[#334155] flex items-center justify-center text-[#94a3b8]">
              <!-- Mock Icon -->
              <span class="text-xs uppercase font-bold">{{ mod.icon || mod.name.slice(0, 2) }}</span>
            </div>
            <div>
              <h3 class="font-semibold text-[#e2e8f0] leading-tight">{{ mod.name }}</h3>
              <span class="text-xs text-indigo-400 font-medium">{{ mod.category || 'System' }}</span>
            </div>
          </div>
        </div>

        <!-- Description -->
        <p class="text-xs text-[#64748b] flex-grow line-clamp-3">
          {{ mod.description }}
        </p>

        <!-- Footer -->
        <div class="flex items-center justify-between mt-auto pt-4 border-t border-[#334155]">
          <div class="text-xs text-[#64748b] truncate w-1/2" :title="mod.software_version ? 'v' + mod.software_version : 'Not Installed'">
            {{ mod.software_version ? 'v' + mod.software_version : '—' }}
          </div>
          <span :class="stateBadge(mod.state)">{{ mod.state }}</span>
        </div>
      </router-link>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import api, { apiErrorMessage } from '@/api/client'

const modules = ref([])
const loading = ref(true)
const errorMessage = ref('')
const searchQuery = ref('')
const selectedCategory = ref('')
const selectedState = ref('')

const categories = [
  'Web', 'Databases', 'Languages', 'Security', 'Monitoring', 
  'Network', 'Storage', 'Development', 'Mail', 'Containers', 'VPN', 'System'
]

const stats = computed(() => {
  return {
    total: modules.value.length,
    installed: modules.value.filter(m => m.state !== 'available').length
  }
})

const filteredModules = computed(() => {
  return modules.value.filter(m => {
    // Search filter
    if (searchQuery.value) {
      const q = searchQuery.value.toLowerCase()
      if (!m.name.toLowerCase().includes(q) && !m.description.toLowerCase().includes(q)) {
        return false
      }
    }
    // Category filter
    if (selectedCategory.value && m.category !== selectedCategory.value) {
      return false
    }
    // State filter
    if (selectedState.value) {
      if (selectedState.value === 'installed' && m.state === 'available') return false
      else if (selectedState.value !== 'installed' && m.state !== selectedState.value) return false
    }
    return true
  })
})

onMounted(loadModules)

async function loadModules() {
  loading.value = true
  try {
    errorMessage.value = ''
    const { data } = await api.get('/modules')
    modules.value = data || []
  } catch (e) {
    errorMessage.value = apiErrorMessage(e, 'Unable to load modules')
  } finally {
    loading.value = false
  }
}

function stateBadge(state) {
  const map = {
    enabled: 'badge-success', disabled: 'badge-muted',
    installing: 'badge-primary', error: 'badge-danger',
    available: 'badge-muted', installed: 'badge-warning',
    removing: 'badge-danger',
  }
  return map[state] || 'badge-muted'
}
</script>
