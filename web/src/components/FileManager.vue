<template>
  <div class="fixed inset-0 z-50 flex flex-col bg-[#0f1117]/95 backdrop-blur-sm">
    <!-- Header -->
    <div class="flex items-center justify-between px-6 py-4 border-b border-[#2d3748] bg-[#161b27]">
      <div class="flex items-center gap-4">
        <h2 class="text-lg font-semibold text-white">{{ $t('sites.fileManager.title') || 'File Manager' }}</h2>
        <span class="text-sm font-mono text-[#64748b] bg-[#0f1117] px-2 py-1 rounded">{{ site.domain }}</span>
      </div>
      <button @click="$emit('close')" class="text-[#64748b] hover:text-white transition-colors">
        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
      </button>
    </div>

    <!-- Toolbar & Breadcrumb -->
    <div class="flex items-center justify-between px-6 py-3 border-b border-[#2d3748] bg-[#161b27]/50">
      <div class="flex items-center gap-2 text-sm font-mono text-[#e2e8f0]">
        <button @click="navigateTo('/')" class="hover:text-indigo-400 transition-colors">/ (root)</button>
        <template v-for="(part, idx) in pathParts" :key="idx">
          <span class="text-[#64748b]">/</span>
          <button @click="navigateTo(buildPath(idx))" class="hover:text-indigo-400 transition-colors">{{ part }}</button>
        </template>
      </div>
      <div class="flex items-center gap-3">
        <label class="btn-primary cursor-pointer">
          <input type="file" class="hidden" @change="uploadFile" />
          {{ $t('sites.fileManager.upload') || 'Upload File' }}
        </label>
        <button @click="refresh" class="btn-secondary" title="Refresh">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
        </button>
      </div>
    </div>

    <!-- Error message -->
    <div v-if="error" class="m-6 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
      {{ error }}
    </div>

    <!-- Content -->
    <div class="flex-1 overflow-auto p-6">
      <div class="card p-0 overflow-hidden">
        <table class="table w-full">
          <thead>
            <tr>
              <th class="w-8"></th>
              <th>{{ $t('sites.fileManager.name') || 'Name' }}</th>
              <th>{{ $t('sites.fileManager.size') || 'Size' }}</th>
              <th>{{ $t('sites.fileManager.modified') || 'Modified' }}</th>
              <th class="w-24"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading">
              <td colspan="5" class="text-center py-8 text-[#64748b]">{{ $t('sites.fileManager.loading') || 'Loading...' }}</td>
            </tr>
            <tr v-else-if="currentPath !== '/'" class="cursor-pointer hover:bg-white/5 transition-colors" @click="navigateUp">
              <td class="text-center text-[#64748b]">
                <svg class="w-5 h-5 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6"/></svg>
              </td>
              <td colspan="4" class="font-medium text-[#94a3b8]">..</td>
            </tr>
            <tr v-if="!loading && !files.length && currentPath === '/'">
              <td colspan="5" class="text-center py-8 text-[#64748b]">{{ $t('sites.fileManager.empty') || 'Directory is empty' }}</td>
            </tr>
            <tr v-for="file in sortedFiles" :key="file.name" class="hover:bg-white/5 transition-colors group">
              <td class="text-center">
                <svg v-if="file.is_dir" class="w-5 h-5 inline text-blue-400" fill="currentColor" viewBox="0 0 20 20"><path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z"/></svg>
                <svg v-else class="w-5 h-5 inline text-[#94a3b8]" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"/></svg>
              </td>
              <td>
                <button v-if="file.is_dir" class="text-white hover:text-indigo-400 font-medium text-left" @click="navigateTo(currentPath === '/' ? '/' + file.name : currentPath + '/' + file.name)">
                  {{ file.name }}
                </button>
                <span v-else class="text-[#e2e8f0]">{{ file.name }}</span>
              </td>
              <td class="text-sm text-[#94a3b8]">{{ file.is_dir ? '—' : formatBytes(file.size) }}</td>
              <td class="text-sm text-[#94a3b8]">{{ new Date(file.mod_time).toLocaleString() }}</td>
              <td>
                <button @click="deleteFile(file)" class="text-red-400 hover:text-red-300 opacity-0 group-hover:opacity-100 transition-opacity p-1">
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import api, { apiErrorMessage } from '@/api/client'

const props = defineProps({
  site: { type: Object, required: true }
})

const emit = defineEmits(['close'])

const currentPath = ref('/')
const files = ref([])
const loading = ref(true)
const error = ref('')

const pathParts = computed(() => {
  return currentPath.value.split('/').filter(Boolean)
})

function buildPath(idx) {
  return '/' + pathParts.value.slice(0, idx + 1).join('/')
}

const sortedFiles = computed(() => {
  return [...files.value].sort((a, b) => {
    if (a.is_dir === b.is_dir) return a.name.localeCompare(b.name)
    return a.is_dir ? -1 : 1
  })
})

onMounted(refresh)

async function refresh() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await api.get(`/sites/${props.site.id}/files`, { params: { path: currentPath.value } })
    files.value = data || []
  } catch (e) {
    error.value = apiErrorMessage(e, 'Failed to load directory')
    files.value = []
  } finally {
    loading.value = false
  }
}

function navigateTo(path) {
  currentPath.value = path
  refresh()
}

function navigateUp() {
  const parts = pathParts.value
  parts.pop()
  currentPath.value = '/' + parts.join('/')
  refresh()
}

async function uploadFile(event) {
  const file = event.target.files[0]
  if (!file) return
  error.value = ''
  const formData = new FormData()
  formData.append('file', file)
  
  const uploadPath = currentPath.value === '/' ? '/' + file.name : currentPath.value + '/' + file.name
  
  try {
    await api.post(`/sites/${props.site.id}/file`, formData, { 
      params: { path: uploadPath },
      headers: { 'Content-Type': 'multipart/form-data' }
    })
    refresh()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Upload failed')
  } finally {
    event.target.value = '' // reset input
  }
}

async function deleteFile(file) {
  if (!confirm(`Delete ${file.name}?`)) return
  error.value = ''
  const deletePath = currentPath.value === '/' ? '/' + file.name : currentPath.value + '/' + file.name
  try {
    await api.delete(`/sites/${props.site.id}/file`, { params: { path: deletePath } })
    refresh()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Delete failed')
  }
}

function formatBytes(bytes) {
  if (bytes === 0) return '0 B'
  const k = 1024, sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}
</script>
