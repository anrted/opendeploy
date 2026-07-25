<template>
  <div class="fixed inset-0 z-50 flex flex-col bg-[#0f1117]/95 backdrop-blur-md font-sans">
    <!-- Header -->
    <div class="flex items-center justify-between px-6 py-4 border-b border-[#2d3748]/50 bg-[#161b27]/80">
      <div class="flex items-center gap-4">
        <h2 class="text-xl font-bold text-white tracking-wide">File Manager</h2>
        <span class="text-sm font-mono text-indigo-300 bg-indigo-900/30 border border-indigo-500/30 px-3 py-1 rounded-full shadow-inner">{{ site.domain }}</span>
      </div>
      <button @click="$emit('close')" class="p-2 text-slate-400 hover:text-white hover:bg-white/10 rounded-full transition-all">
        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
      </button>
    </div>

    <!-- Toolbar & Breadcrumb -->
    <div class="flex items-center justify-between px-6 py-4 border-b border-[#2d3748]/50 bg-[#1a202c]/60">
      <div class="flex items-center gap-2 text-sm font-mono text-slate-300">
        <button @click="navigateTo('/')" class="hover:text-indigo-400 transition-colors p-1 rounded hover:bg-white/5">
          <svg class="w-5 h-5 inline-block mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"/></svg>
          {{ site.root_path }}
        </button>
        <template v-for="(part, idx) in pathParts" :key="idx">
          <span class="text-slate-600">/</span>
          <button @click="navigateTo(buildPath(idx))" class="hover:text-indigo-400 transition-colors px-2 py-1 rounded hover:bg-white/5">{{ part }}</button>
        </template>
      </div>
      
      <div class="flex items-center gap-3">
        <!-- Mass Actions (visible if items selected) -->
        <transition name="fade">
          <div v-if="selectedFiles.length > 0" class="flex gap-2 mr-4 border-r border-slate-700 pr-4">
            <button @click="batchDelete" class="btn-secondary text-red-400 hover:text-red-300 hover:bg-red-500/10 border-red-500/30">
              <svg class="w-4 h-4 mr-1 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
              Delete ({{ selectedFiles.length }})
            </button>
          </div>
        </transition>

        <button @click="createFolder" class="btn-secondary flex items-center gap-1 shadow-sm hover:shadow-md transition-shadow">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 13h6m-3-3v6m-9 1V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1m1-1v8a2 2 0 01-2 2H6a2 2 0 01-2-2z"/></svg>
          New Folder
        </button>
        <label class="btn-primary cursor-pointer flex items-center gap-1 shadow-sm hover:shadow-indigo-500/30 transition-shadow bg-indigo-600 hover:bg-indigo-500">
          <input type="file" class="hidden" multiple @change="uploadFiles" />
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"/></svg>
          Upload
        </label>
        <button @click="refresh" class="btn-secondary p-2" title="Refresh">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" :class="{'animate-spin': loading}"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
        </button>
      </div>
    </div>

    <!-- Error message -->
    <div v-if="error" class="mx-6 mt-6 rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-200 flex items-center shadow-lg shadow-red-500/5">
      <svg class="w-5 h-5 mr-3 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
      {{ error }}
    </div>

    <!-- Main Content Area -->
    <div class="flex-1 overflow-auto p-6 flex gap-6">
      
      <!-- File List -->
      <div class="flex-1 bg-[#1e293b]/50 border border-slate-700/50 rounded-2xl shadow-xl overflow-hidden backdrop-blur-sm flex flex-col relative" @drop.prevent="handleDrop" @dragover.prevent>
        <table class="w-full text-left border-collapse">
          <thead>
            <tr class="bg-slate-800/50 text-slate-300 text-sm font-semibold border-b border-slate-700/50">
              <th class="p-4 w-12 text-center">
                <input type="checkbox" class="rounded border-slate-600 bg-slate-700 text-indigo-500 focus:ring-indigo-500 focus:ring-offset-slate-800" 
                       :checked="isAllSelected" @change="toggleSelectAll" />
              </th>
              <th class="p-4 cursor-pointer hover:text-white" @click="sortBy('name')">Name</th>
              <th class="p-4 w-32 cursor-pointer hover:text-white" @click="sortBy('size')">Size</th>
              <th class="p-4 w-48 cursor-pointer hover:text-white" @click="sortBy('mod_time')">Modified</th>
              <th class="p-4 w-32 text-center">Owner</th>
              <th class="p-4 w-24 text-center">Perms</th>
            </tr>
          </thead>
          <tbody class="text-slate-300 text-sm">
            <tr v-if="loading" class="animate-pulse">
              <td colspan="6" class="text-center py-12 text-slate-500">Loading directory contents...</td>
            </tr>
            <tr v-else-if="currentPath !== '/'" class="cursor-pointer hover:bg-slate-700/30 transition-colors border-b border-slate-700/30" @click="navigateUp">
              <td class="p-4 text-center text-slate-500">
                <svg class="w-5 h-5 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6"/></svg>
              </td>
              <td colspan="5" class="p-4 font-medium text-indigo-300">..</td>
            </tr>
            <tr v-if="!loading && !files.length && currentPath === '/'">
              <td colspan="6" class="text-center py-12 text-slate-500">
                <div class="flex flex-col items-center">
                  <svg class="w-16 h-16 text-slate-700 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/></svg>
                  Directory is empty. Drag and drop files here to upload.
                </div>
              </td>
            </tr>
            <tr v-for="file in sortedFiles" :key="file.name" 
                class="hover:bg-slate-700/30 transition-colors border-b border-slate-700/30 group"
                :class="{'bg-indigo-900/20': isSelected(file)}"
                @contextmenu.prevent="openContextMenu($event, file)">
              <td class="p-4 text-center">
                <input type="checkbox" class="rounded border-slate-600 bg-slate-700 text-indigo-500 focus:ring-indigo-500 focus:ring-offset-slate-800" 
                       :value="file.name" v-model="selectedFiles" />
              </td>
              <td class="p-4">
                <div class="flex items-center gap-3">
                  <svg v-if="file.is_dir" class="w-6 h-6 text-blue-400 drop-shadow-sm" fill="currentColor" viewBox="0 0 20 20"><path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z"/></svg>
                  <svg v-else class="w-6 h-6 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"/></svg>
                  
                  <button v-if="file.is_dir" class="text-white hover:text-indigo-400 font-medium text-left truncate max-w-[200px] sm:max-w-xs transition-colors" 
                          @click="navigateTo(currentPath === '/' ? '/' + file.name : currentPath + '/' + file.name)">
                    {{ file.name }}
                  </button>
                  <span v-else class="text-slate-200 truncate max-w-[200px] sm:max-w-xs">{{ file.name }}</span>
                </div>
              </td>
              <td class="p-4 text-slate-400 font-mono text-xs">{{ file.is_dir ? '—' : formatBytes(file.size) }}</td>
              <td class="p-4 text-slate-400">{{ new Date(file.mod_time * 1000).toLocaleString() }}</td>
              <td class="p-4 text-center text-slate-400 font-mono text-xs">{{ file.owner }}:{{ file.group }}</td>
              <td class="p-4 text-center text-slate-400 font-mono text-xs">{{ formatPerms(file.mode) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    
    <!-- Context Menu -->
    <div v-if="contextMenu.show" 
         class="fixed bg-[#1e293b] border border-slate-600 rounded-lg shadow-2xl py-2 z-[60] text-sm w-48 backdrop-blur-md"
         :style="{ top: contextMenu.y + 'px', left: contextMenu.x + 'px' }"
         @click.stop>
      <div class="px-3 py-1 text-xs text-slate-500 uppercase tracking-wider font-semibold border-b border-slate-700/50 mb-1 truncate">
        {{ contextMenu.file?.name }}
      </div>
      <button v-if="!contextMenu.file?.is_dir" @click="downloadFile(contextMenu.file)" class="w-full text-left px-4 py-2 hover:bg-slate-700 hover:text-white transition-colors flex items-center">
        <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/></svg> Download
      </button>
      <button @click="renameFile(contextMenu.file)" class="w-full text-left px-4 py-2 hover:bg-slate-700 hover:text-white transition-colors flex items-center">
        <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/></svg> Rename
      </button>
      <button @click="chmodFile(contextMenu.file)" class="w-full text-left px-4 py-2 hover:bg-slate-700 hover:text-white transition-colors flex items-center">
        <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg> Permissions
      </button>
      <div class="border-t border-slate-700/50 my-1"></div>
      <button @click="deleteFile(contextMenu.file)" class="w-full text-left px-4 py-2 text-red-400 hover:bg-red-500/10 hover:text-red-300 transition-colors flex items-center">
        <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg> Delete
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import api, { apiErrorMessage } from '@/api/client'

const props = defineProps({
  site: { type: Object, required: true }
})

const emit = defineEmits(['close'])

const currentPath = ref('/')
const files = ref([])
const loading = ref(true)
const error = ref('')
const selectedFiles = ref([])
const sortConfig = ref({ key: 'name', dir: 'asc' })

const contextMenu = ref({ show: false, x: 0, y: 0, file: null })

const pathParts = computed(() => {
  return currentPath.value.split('/').filter(Boolean)
})

function buildPath(idx) {
  return '/' + pathParts.value.slice(0, idx + 1).join('/')
}

const sortedFiles = computed(() => {
  let sorted = [...files.value]
  const { key, dir } = sortConfig.value
  const modifier = dir === 'asc' ? 1 : -1
  
  sorted.sort((a, b) => {
    // Directories always on top
    if (a.is_dir && !b.is_dir) return -1
    if (!a.is_dir && b.is_dir) return 1
    
    if (a[key] < b[key]) return -1 * modifier
    if (a[key] > b[key]) return 1 * modifier
    return 0
  })
  return sorted
})

const isAllSelected = computed(() => {
  return files.value.length > 0 && selectedFiles.value.length === files.value.length
})

function toggleSelectAll(e) {
  if (e.target.checked) {
    selectedFiles.value = files.value.map(f => f.name)
  } else {
    selectedFiles.value = []
  }
}

function isSelected(file) {
  return selectedFiles.value.includes(file.name)
}

function sortBy(key) {
  if (sortConfig.value.key === key) {
    sortConfig.value.dir = sortConfig.value.dir === 'asc' ? 'desc' : 'asc'
  } else {
    sortConfig.value.key = key
    sortConfig.value.dir = 'asc'
  }
}

onMounted(() => {
  refresh()
  document.addEventListener('click', closeContextMenu)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', closeContextMenu)
})

async function refresh() {
  loading.value = true
  error.value = ''
  selectedFiles.value = []
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

function getFilePath(name) {
  return currentPath.value === '/' ? '/' + name : currentPath.value + '/' + name
}

async function uploadFiles(event) {
  const fileList = event.target.files
  if (!fileList.length) return
  await processUploads(fileList)
  event.target.value = ''
}

async function handleDrop(event) {
  const fileList = event.dataTransfer.files
  if (!fileList.length) return
  await processUploads(fileList)
}

async function processUploads(fileList) {
  error.value = ''
  for (let i = 0; i < fileList.length; i++) {
    const file = fileList[i]
    const formData = new FormData()
    formData.append('file', file)
    
    const uploadPath = getFilePath(file.name)
    
    try {
      await api.post(`/sites/${props.site.id}/file`, formData, { 
        params: { path: uploadPath },
        headers: { 'Content-Type': 'multipart/form-data' }
      })
    } catch (e) {
      error.value = apiErrorMessage(e, `Upload failed for ${file.name}`)
      break
    }
  }
  refresh()
}

async function createFolder() {
  const folderName = prompt('Enter new folder name:')
  if (!folderName || !folderName.trim()) return
  error.value = ''
  const createPath = getFilePath(folderName.trim())
  
  try {
    await api.post(`/sites/${props.site.id}/directory`, { path: createPath })
    refresh()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Failed to create directory')
  }
}

function openContextMenu(event, file) {
  contextMenu.value = {
    show: true,
    x: event.clientX,
    y: event.clientY,
    file
  }
}

function closeContextMenu() {
  contextMenu.value.show = false
}

async function deleteFile(file) {
  closeContextMenu()
  if (!confirm(`Delete ${file.name}?`)) return
  error.value = ''
  
  try {
    await api.post(`/sites/${props.site.id}/files/batch`, { 
      action: 'delete',
      paths: [getFilePath(file.name)]
    })
    refresh()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Delete failed')
  }
}

async function batchDelete() {
  if (!selectedFiles.value.length) return
  if (!confirm(`Delete ${selectedFiles.value.length} selected items?`)) return
  
  const paths = selectedFiles.value.map(name => getFilePath(name))
  try {
    await api.post(`/sites/${props.site.id}/files/batch`, { 
      action: 'delete',
      paths: paths
    })
    refresh()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Batch delete failed')
  }
}

async function renameFile(file) {
  closeContextMenu()
  const newName = prompt('Enter new name:', file.name)
  if (!newName || newName === file.name) return
  
  try {
    await api.post(`/sites/${props.site.id}/files/batch`, { 
      action: 'move',
      paths: [getFilePath(file.name)],
      dst_path: getFilePath(newName)
    })
    refresh()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Rename failed')
  }
}

async function chmodFile(file) {
  closeContextMenu()
  const perms = prompt('Enter new permissions (e.g. 644 or 755):', formatPerms(file.mode))
  if (!perms) return
  
  try {
    await api.post(`/sites/${props.site.id}/files/batch`, { 
      action: 'chmod',
      paths: [getFilePath(file.name)],
      mode: parseInt(perms, 8)
    })
    refresh()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Chmod failed')
  }
}

function downloadFile(file) {
  closeContextMenu()
  window.open(`${api.defaults.baseURL}/sites/${props.site.id}/file?path=${encodeURIComponent(getFilePath(file.name))}`, '_blank')
}

function formatBytes(bytes) {
  if (bytes === 0) return '0 B'
  const k = 1024, sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function formatPerms(mode) {
  if (mode === undefined) return ''
  return (mode & 0o777).toString(8).padStart(3, '0')
}
</script>

<style scoped>
.fade-enter-active, .fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}
.fade-enter-from, .fade-leave-to {
  opacity: 0;
  transform: translateY(-5px);
}
</style>
