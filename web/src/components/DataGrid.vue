<template>
  <div class="space-y-4">
    <!-- Toolbar -->
    <div class="flex justify-between items-center mb-4">
      <div class="flex space-x-2">
        <button v-for="action in schema.actions" :key="action.id"
          @click="executeGlobalAction(action)"
          class="btn text-sm py-1.5"
          :class="action.dangerous ? 'bg-red-500/10 text-red-400 hover:bg-red-500/20' : 'bg-indigo-500 text-white hover:bg-indigo-600'">
          <i v-if="action.icon" :class="['feather', 'icon-' + action.icon, 'mr-1']"></i>
          {{ action.title }}
        </button>
      </div>
      <div>
        <input type="text" class="input w-64 text-sm py-1.5" placeholder="Search..." v-model="searchQuery" />
      </div>
    </div>

    <!-- Table -->
    <div class="overflow-x-auto bg-[#1e293b] border border-[#334155] rounded-xl">
      <table class="w-full text-left text-sm text-[#94a3b8]">
        <thead class="text-xs uppercase bg-black/20 text-[#64748b]">
          <tr>
            <th v-for="col in schema.columns" :key="col.key" class="px-6 py-3 font-medium">
              {{ col.title }}
            </th>
            <th class="px-6 py-3 font-medium text-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading" class="border-t border-[#334155]">
            <td :colspan="schema.columns.length + 1" class="px-6 py-8 text-center">
              <div class="inline-block animate-spin rounded-full h-6 w-6 border-b-2 border-indigo-500"></div>
            </td>
          </tr>
          <tr v-else-if="filteredData.length === 0" class="border-t border-[#334155]">
            <td :colspan="schema.columns.length + 1" class="p-0">
              <EmptyState title="No Data" description="No data found." />
            </td>
          </tr>
          <tr v-for="(row, idx) in filteredData" :key="idx" class="border-t border-[#334155] hover:bg-[#334155]/30 transition-colors">
            <td v-for="col in schema.columns" :key="col.key" class="px-6 py-4">
              <span v-if="col.type === 'badge'" class="px-2 py-1 text-xs rounded-full bg-slate-700 text-slate-300">
                {{ row[col.key] }}
              </span>
              <span v-else class="text-[#e2e8f0]">
                {{ row[col.key] }}
              </span>
            </td>
            <td class="px-6 py-4 text-right">
              <!-- Actions for row would go here if schema supported row actions, for now just global actions -->
              <span class="text-xs text-[#64748b]">No actions</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import EmptyState from '@/components/EmptyState.vue'
import { useConfirmStore } from '@/stores/confirm'
import api, { apiErrorMessage } from '@/api/client'

const confirm = useConfirmStore()

const props = defineProps({
  module: { type: Object, required: true },
  page: { type: Object, required: true }
})

const schema = ref({ columns: [], actions: [] })
const data = ref([])
const loading = ref(true)
const searchQuery = ref('')

const fetchSchema = async () => {
  const { data } = await api.get(`/modules/${props.module.id}/datagrid/${props.page.id}/schema`)
  schema.value = data
}

const fetchData = async () => {
  loading.value = true
  try {
    const response = await api.get(`/modules/${props.module.id}/datagrid/${props.page.id}/data`)
    data.value = Array.isArray(response.data) ? response.data : []
  } finally {
    loading.value = false
  }
}

const executeGlobalAction = async (action) => {
  if (action.requires_confirmation) {
    const confirmed = await confirm.require({
      title: 'Confirm Action',
      message: `Are you sure you want to ${action.title}?`,
      confirmText: action.title,
      type: action.dangerous ? 'danger' : 'warning'
    })
    if (!confirmed) return
  }
  try {
    await api.post(`/modules/${props.module.id}/datagrid/${props.page.id}/action/${action.id}`)
    alert(`${action.title} executed successfully`)
    fetchData()
  } catch (error) {
    alert(`Failed to execute ${action.title}: ${apiErrorMessage(error)}`)
  }
}

const filteredData = computed(() => {
  if (!searchQuery.value) return data.value
  const q = searchQuery.value.toLowerCase()
  return data.value.filter(row => {
    return Object.values(row).some(val => 
      String(val).toLowerCase().includes(q)
    )
  })
})

onMounted(async () => {
  try {
    await Promise.all([fetchSchema(), fetchData()])
  } catch (error) {
    console.error('Failed to load data grid:', apiErrorMessage(error))
    loading.value = false
  }
})
</script>
