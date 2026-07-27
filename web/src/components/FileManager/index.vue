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

    <FileToolbar
      :root-path="site.root_path" :path-parts="pathParts"
      :selected-count="selectedFiles.length" :loading="loading"
      v-model:search="searchQuery"
      @navigate="navigateTo" @archive="batchArchive" @delete="batchDelete"
      @create-folder="createFolder" @upload="uploadFiles" @refresh="refresh"
    />

    <!-- Error message -->
    <div v-if="error" class="mx-6 mt-6 rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-200 flex items-center shadow-lg shadow-red-500/5">
      <svg class="w-5 h-5 mr-3 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
      {{ error }}
    </div>

    <!-- Main Content Area -->
    <div class="flex-1 overflow-hidden p-6 flex gap-6">
      
      <FileTable
        :files="visibleFiles" :loading="loading" :current-path="currentPath"
        :all-selected="isAllSelected" :is-editable="isEditable"
        v-model:selected="selectedFiles"
        @toggle-all="toggleSelectAll" @sort="sortBy" @up="navigateUp"
        @drop="handleDrop" @context="openContextMenu" @edit="openEditor"
        @open-directory="file => navigateTo(getFilePath(file.name))"
      />
    </div>
    
    <FileContextMenu
      :menu="contextMenu" :editable="isEditable(contextMenu.file?.name)"
      :archive="isArchive(contextMenu.file?.name)"
      @download="downloadFile" @edit="file => { openEditor(file); closeContextMenu() }"
      @rename="renameFile" @copy="copyFile" @move="moveFile"
      @extract="extractFile" @chmod="chmodFile" @delete="deleteFile"
    />

    <!-- Monaco Editor Modal -->
    <FileEditor
      v-if="editingFile && !loadingFile"
      :filename="editingFile.name"
      :initial-content="editingContent"
      :is-saving="savingFile"
      @close="closeEditor"
      @save="saveFile"
    />
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, defineAsyncComponent } from 'vue'
import api, { apiErrorMessage } from '@/api/client'
import { useConfirmStore } from '@/stores/confirm'
import FileToolbar from './FileToolbar.vue'
import FileTable from './FileTable.vue'
import FileContextMenu from './FileContextMenu.vue'
import { useFileListing } from './useFileListing'
import { useFileEditor } from './useFileEditor'

const FileEditor = defineAsyncComponent(() => import('./FileEditor.vue'))

const confirm = useConfirmStore()

const props = defineProps({
  site: { type: Object, required: true }
})

const emit = defineEmits(['close'])

const {
  currentPath, files, loading, error, selectedFiles, searchQuery,
  pathParts, visibleFiles, isAllSelected,
  refresh, navigateTo, navigateUp, sortBy, toggleSelectAll
} = useFileListing(props.site)

const contextMenu = ref({ show: false, x: 0, y: 0, file: null })

const {
  editingFile, editingContent, loadingFile, savingFile,
  isEditable, openEditor, saveFile, closeEditor
} = useFileEditor(props.site, getFilePath, refresh, error)

onMounted(() => {
  refresh()
  document.addEventListener('click', closeContextMenu)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', closeContextMenu)
})

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
        params: { path: uploadPath }
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
  const confirmed = await confirm.require({
    title: 'Delete File',
    message: `Are you sure you want to delete ${file.name}?`,
    confirmText: 'Delete',
    type: 'danger'
  })
  if (!confirmed) return
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
  const confirmed = await confirm.require({
    title: 'Delete Files',
    message: `Are you sure you want to delete ${selectedFiles.value.length} selected items?`,
    confirmText: 'Delete',
    type: 'danger'
  })
  if (!confirmed) return
  
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

async function copyFile(file) {
  closeContextMenu()
  const destination = prompt('Copy to path:', getFilePath(`${file.name}.copy`))
  if (!destination) return
  try {
    await api.post(`/sites/${props.site.id}/files/batch`, { action: 'copy', paths: [getFilePath(file.name)], dst_path: destination })
    refresh()
  } catch (e) { error.value = apiErrorMessage(e, 'Copy failed') }
}

async function moveFile(file) {
  closeContextMenu()
  const source = getFilePath(file.name)
  const destination = prompt('Move to path:', source)
  if (!destination || destination === source) return
  try {
    await api.post(`/sites/${props.site.id}/files/batch`, { action: 'move', paths: [source], dst_path: destination })
    refresh()
  } catch (e) { error.value = apiErrorMessage(e, 'Move failed') }
}

function isArchive(name = '') {
  return /\.(zip|tar|tar\.gz|tgz)$/i.test(name)
}

async function batchArchive() {
  if (!selectedFiles.value.length) return
  const archiveName = prompt('Archive filename:', 'archive.zip')
  if (!archiveName) return
  try {
    await api.post(`/sites/${props.site.id}/files/batch`, {
      action: 'archive',
      paths: selectedFiles.value.map(getFilePath),
      dst_path: getFilePath(archiveName)
    })
    refresh()
  } catch (e) { error.value = apiErrorMessage(e, 'Archive creation failed') }
}

async function extractFile(file) {
  closeContextMenu()
  const confirmed = await confirm.require({ title: 'Extract archive', message: `Extract ${file.name} into the current directory?`, confirmText: 'Extract', type: 'warning' })
  if (!confirmed) return
  try {
    await api.post(`/sites/${props.site.id}/files/batch`, { action: 'extract', paths: [getFilePath(file.name)], dst_path: currentPath.value })
    refresh()
  } catch (e) { error.value = apiErrorMessage(e, 'Archive extraction failed') }
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
